// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

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
func NewReactor(o *Options) *Reactor {
	var gitInfoController GitInfoController
	rhRegistry := resourcehandlers.NewRegistry(o.ResourceHandlers...)
	downloadController := NewDownloadController(nil, o.ResourceDownloadWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	if o.GitInfoWriter != nil {
		gitInfoController = NewGitInfoController(nil, o.GitInfoWriter, o.ResourceDownloadWorkersCount, o.FailFast, rhRegistry)
	}
	worker := &DocumentWorker{
		Writer:               o.Writer,
		Reader:               &GenericReader{rhRegistry},
		NodeContentProcessor: NewNodeContentProcessor(o.ResourcesPath, o.GlobalLinksConfig, downloadController, o.FailFast, o.RewriteEmbedded, rhRegistry),
		Processor:            o.Processor,
		GitHubInfoController: gitInfoController,
		templates:            map[string]*template.Template{},
	}
	docController := NewDocumentController(worker, o.MaxWorkersCount, o.FailFast)
	r := &Reactor{
		FailFast:           o.FailFast,
		ResourceHandlers:   rhRegistry,
		DocController:      docController,
		DownloadController: downloadController,
		GitInfoController:  gitInfoController,
		DryRunWriter:       o.DryRunWriter,
		Resolve:            o.Resolve,
		IndexFileNames:     o.IndexFileNames,
		manifestAbsPath:    o.ManifestAbsPath,
	}
	return r
}

// Reactor orchestrates the documentation build workflow
type Reactor struct {
	FailFast           bool
	ResourceHandlers   resourcehandlers.Registry
	DocController      DocumentController
	DownloadController DownloadController
	GitInfoController  GitInfoController
	DryRunWriter       writers.DryRunWriter
	Resolve            bool
	IndexFileNames     []string
	manifestAbsPath    string
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
		return err
	}

	if err := checkForCollisions(manifest.Structure); err != nil {
		klog.Errorf("checkForCollisions: %s", err.Error())
	}

	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

func printResolved(ctx context.Context, manifest *api.Documentation, writer io.Writer) error {
	s, err := api.Serialize(manifest)
	if err != nil {
		return err
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
