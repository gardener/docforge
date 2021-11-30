// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"text/template"

	"github.com/gardener/docforge/pkg/jobs"

	"github.com/gardener/docforge/pkg/processors"
	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers"
)

// Options encapsulates the parameters for creating
// new Reactor objects with NewReactor
type Options struct {
	MaxWorkersCount              int
	MinWorkersCount              int
	FailFast                     bool
	DestinationPath              string
	ResourcesPath                string
	ManifestAbsPath              string
	ResourceDownloadWorkersCount int
	RewriteEmbedded              bool
	processors.Processor
	ResourceDownloadWriter writers.Writer
	GitInfoWriter          writers.Writer
	Writer                 writers.Writer
	ResourceHandlers       []resourcehandlers.ResourceHandler
	DryRunWriter           writers.DryRunWriter
	Resolve                bool
	GlobalLinksConfig      *api.Links
	IndexFileNames         []string
}

// NewReactor creates a Reactor from Options
func NewReactor(o *Options) (*Reactor, error) {
	reactorWG := &sync.WaitGroup{}
	var ghInfo GitHubInfo
	var ghInfoTasks *jobs.JobQueue
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	dWork, err := DownloadWorkFunc(&GenericReader{
		ResourceHandlers: rhRegistry,
	}, o.ResourceDownloadWriter)
	if err != nil {
		return nil, err
	}
	downloadTasks, err := jobs.NewJobQueue("Download", o.ResourceDownloadWorkersCount, dWork, o.FailFast, reactorWG)
	if err != nil {
		return nil, err
	}
	dScheduler := NewDownloadScheduler(downloadTasks)
	if o.GitInfoWriter != nil {
		ghInfoWork, err := GitHubInfoWorkerFunc(&GenericReader{
			ResourceHandlers: rhRegistry,
			IsGitHubInfo:     true,
		}, o.GitInfoWriter)
		if err != nil {
			return nil, err
		}
		ghInfoTasks, err = jobs.NewJobQueue("GitHubInfo", o.ResourceDownloadWorkersCount, ghInfoWork, o.FailFast, reactorWG)
		if err != nil {
			return nil, err
		}
		ghInfo = NewGitHubInfo(ghInfoTasks)
	}
	valWork, err := ValidateWorkerFunc(http.DefaultClient, rhRegistry)
	if err != nil {
		return nil, err
	}
	validatorTasks, _ := jobs.NewJobQueue("Validator", o.MaxWorkersCount, valWork, o.FailFast, reactorWG)
	v := NewValidator(validatorTasks)
	worker := &DocumentWorker{
		writer:               o.Writer,
		reader:               &GenericReader{ResourceHandlers: rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, o.GlobalLinksConfig, dScheduler, v, o.FailFast, o.RewriteEmbedded, rhRegistry),
		Processor:            o.Processor,
		gitHubInfo:           ghInfo,
		templates:            map[string]*template.Template{},
	}
	docTasks, err := jobs.NewJobQueue("Document", o.MaxWorkersCount, worker.Work, o.FailFast, reactorWG)
	if err != nil {
		return nil, err
	}
	r := &Reactor{
		FailFast:         o.FailFast,
		ResourceHandlers: rhRegistry,
		DocumentWorker:   worker,
		DocumentTasks:    docTasks,
		DownloadTasks:    downloadTasks,
		GitHubInfoTasks:  ghInfoTasks,
		ValidatorTasks:   validatorTasks,
		DryRunWriter:     o.DryRunWriter,
		Resolve:          o.Resolve,
		IndexFileNames:   o.IndexFileNames,
		manifestAbsPath:  o.ManifestAbsPath,
		reactorWaitGroup: reactorWG,
	}
	return r, nil
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	FailFast         bool
	ResourceHandlers resourcehandlers.Registry
	DocumentWorker   *DocumentWorker
	DocumentTasks    *jobs.JobQueue
	DownloadTasks    *jobs.JobQueue
	GitHubInfoTasks  *jobs.JobQueue
	ValidatorTasks   *jobs.JobQueue
	DryRunWriter     writers.DryRunWriter
	Resolve          bool
	IndexFileNames   []string
	manifestAbsPath  string
	// reactorWaitGroup used to determine when all parallel tasks are done
	reactorWaitGroup *sync.WaitGroup
}

// Run starts build operation on documentation
func (r *Reactor) Run(ctx context.Context, manifest *api.Documentation, dryRun bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if r.Resolve {
			if err := printResolved(ctx, manifest, os.Stdout); err != nil {
				klog.Errorf("failed to print resolved manifest: %s", err.Error())
			}
		}
		cancel()
		if dryRun {
			r.DryRunWriter.Flush()
		}
	}()

	if err := ResolveManifest(ctx, manifest, r.ResourceHandlers, r.manifestAbsPath, r.IndexFileNames); err != nil {
		return fmt.Errorf("failed to resolve manifest: %s. %+v", r.manifestAbsPath, err)
	}

	if err := checkForCollisions(manifest.Structure); err != nil {
		return err
	}

	sourceLocations := getSourceLocationsMap(manifest.Structure)
	if ncp, ok := r.DocumentWorker.NodeContentProcessor.(*nodeContentProcessor); ok {
		ncp.sourceLocations = sourceLocations
	}
	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

func getSourceLocationsMap(structure []*api.Node) map[string][]*api.Node {
	locations := make(map[string][]*api.Node)
	for _, node := range structure {
		addSourceLocation(locations, node)
	}
	return locations
}

func addSourceLocation(locations map[string][]*api.Node, node *api.Node) {
	if node.Source != "" {
		locations[node.Source] = append(locations[node.Source], node)
	}
	if node.GetSourceLocation() != "" {
		locations[node.GetSourceLocation()] = append(locations[node.GetSourceLocation()], node)
	}
	for _, childNode := range node.Nodes {
		addSourceLocation(locations, childNode)
	}
}

func printResolved(ctx context.Context, manifest *api.Documentation, writer io.Writer) error {
	s, err := api.Serialize(manifest)
	if err != nil {
		return fmt.Errorf("failed to serialize the manifest. %+v", err)
	}
	writer.Write([]byte(s))
	writer.Write([]byte("\n\n"))
	return nil
}

type collision struct {
	nodeParentPath string
	collidedNodes  map[string][]string
}

func checkForCollisions(nodes []*api.Node) error {
	var collisions []collision

	collisions = deepCheckNodesForCollisions(nodes, nil, collisions)

	if len(collisions) <= 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString("Node collisions detected.")
	for _, collision := range collisions {
		sb.WriteString("\nIn ")
		sb.WriteString(collision.nodeParentPath)
		sb.WriteString(" container node.")
		for node, sources := range collision.collidedNodes {
			sb.WriteString(" Node with name ")
			sb.WriteString(node)
			sb.WriteString(" appears ")
			sb.WriteString(fmt.Sprint(len(sources)))
			sb.WriteString(" times for sources: ")
			sb.WriteString(strings.Join(sources, ", "))
			sb.WriteString(".")
		}
	}
	return errors.New(sb.String())
}

func deepCheckNodesForCollisions(nodes []*api.Node, parent *api.Node, collisions []collision) []collision {
	collisions = checkNodesForCollision(nodes, parent, collisions)
	for _, node := range nodes {
		if len(node.Nodes) > 0 {
			collisions = deepCheckNodesForCollisions(node.Nodes, node, collisions)
		}
	}
	return collisions
}

func checkNodesForCollision(nodes []*api.Node, parent *api.Node, collisions []collision) []collision {
	if len(nodes) < 2 {
		return collisions
	}
	// It is unlikely to have a collision so keep the detection logic as simple and fast as possible.
	checked := make(map[string]struct{}, len(nodes))
	var collisionsNames []string
	for _, node := range nodes {
		if _, ok := checked[node.Name]; !ok {
			checked[node.Name] = struct{}{}
		} else {
			collisionsNames = append(collisionsNames, node.Name)
		}
	}

	if len(collisionsNames) == 0 {
		return collisions
	}

	return append(collisions, buildNodeCollision(nodes, parent, collisionsNames))
}

func buildNodeCollision(nodes []*api.Node, parent *api.Node, collisionsNames []string) collision {
	c := collision{
		nodeParentPath: getNodeParentPath(parent),
		collidedNodes:  make(map[string][]string, len(collisionsNames)),
	}

	for _, collisionName := range collisionsNames {
		for _, node := range nodes {
			if node.Name == collisionName {
				collidedNodes := c.collidedNodes[node.Name]
				c.collidedNodes[node.Name] = append(collidedNodes, node.Source)
			}
		}
	}

	return c
}

func getNodeParentPath(node *api.Node) string {
	if node == nil {
		return "root"
	}
	parents := node.Parents()
	var sb strings.Builder
	for _, child := range parents {
		sb.WriteString(child.Name)
		sb.WriteRune('.')
	}
	sb.WriteString(node.Name)
	return sb.String()
}
