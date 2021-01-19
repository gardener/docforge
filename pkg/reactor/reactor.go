// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"text/template"

	"github.com/gardener/docforge/pkg/processors"
	"github.com/google/uuid"
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

	if err := r.ResolveManifest(ctx, manifest); err != nil {
		return err
	}

	klog.V(4).Info("Building documentation structure\n\n")
	if err := r.Build(ctx, manifest.Structure); err != nil {
		return err
	}

	return nil
}

// ResolveManifest resolves the manifests into buildable model
func (r *Reactor) ResolveManifest(ctx context.Context, manifest *api.Documentation) error {
	var (
		structure []*api.Node
		err       error
	)
	if manifest.NodeSelector != nil {
		node, err := r.resolveNodeSelector(ctx, &api.Node{NodeSelector: manifest.NodeSelector}, manifest.Links)
		if err != nil {
			return err
		}
		manifest.NodeSelector = nil
		structure = node.Nodes
	}
	if structure == nil {
		structure = manifest.Structure
	} else {
		// TODO: this should be rather merge than append
		structure = append(manifest.Structure, structure...)
	}

	if structure == nil {
		return fmt.Errorf("document structure resolved to nil")
	}

	if err = r.resolveStructure(ctx, structure, manifest.Links); err != nil {
		return err
	}

	rootNode := &api.Node{
		Nodes: []*api.Node{},
	}
	for _, n := range structure {
		n.SetParent(rootNode)
		rootNode.Nodes = append(rootNode.Nodes, n)
	}

	manifest.Structure = structure
	return nil
}

func printResolved(ctx context.Context, manifest *api.Documentation, writer io.Writer) error {
	// for _, node := range manifest.Structure {
	// 	if links := resolveNodeLinks(node, manifest.Links); len(links) > 0 {
	// 		for _, l := range links {
	// 			l := mergeLinks(node.ResolvedLinks, l)
	// 			node.ResolvedLinks = l
	// 		}
	// 	}
	// 	// remove resolved links for container nodes
	// 	if node.Nodes != nil {
	// 		node.ResolvedLinks = nil
	// 	}
	// }
	s, err := api.Serialize(manifest)
	if err != nil {
		return err
	}
	writer.Write([]byte(s))
	writer.Write([]byte("\n\n"))
	return nil
}

// ResolveStructure resolves the following in a structure model:
// - Node name variables
// - NodeSelectors
// The resulting model is the actual flight plan for replicating resources.
func (r *Reactor) resolveStructure(ctx context.Context, nodes []*api.Node, globalLinksConfig *api.Links) error {
	for _, node := range nodes {
		node.SetParentsDownwards()
		if len(node.Source) > 0 {
			nodeName, err := r.resolveNodeName(ctx, node)
			if err != nil {
				return err
			}
			node.Name = nodeName
		}
		if node.NodeSelector != nil {
			newNode, err := r.resolveNodeSelector(ctx, node, globalLinksConfig)
			if err != nil {
				return err
			}
			node.Nodes = append(node.Nodes, newNode.Nodes...)
			node.Links = mergeLinks(node.Links, newNode.Links)
			node.NodeSelector = nil
		}
		if len(node.Nodes) > 0 {
			if err := r.resolveStructure(ctx, node.Nodes, globalLinksConfig); err != nil {
				return err
			}
		}
		node.SetParentsDownwards()
	}
	return nil
}

func (r *Reactor) resolveNodeSelector(ctx context.Context, node *api.Node, globalLinksConfig *api.Links) (*api.Node, error) {
	newNode := &api.Node{}
	handler := r.ResourceHandlers.Get(node.NodeSelector.Path)
	if handler == nil {
		return nil, fmt.Errorf("No suitable handler registered for path %s", node.NodeSelector.Path)
	}

	moduleDocumentation, err := handler.ResolveDocumentation(ctx, node.NodeSelector.Path)
	if err != nil {
		return nil, err
	}

	if moduleDocumentation != nil {
		newNode.Nodes = moduleDocumentation.Structure
		if moduleLinks := moduleDocumentation.Links; moduleLinks != nil {
			globalNode := &api.Node{
				Links: globalLinksConfig,
			}
			pruneModuleLinks(moduleLinks.Rewrites, node, getNodeRewrites)
			pruneModuleLinks(moduleLinks.Rewrites, globalNode, getNodeRewrites)
			if moduleLinks.Downloads != nil {
				pruneModuleLinks(moduleLinks.Downloads.Renames, node, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(moduleLinks.Downloads.Renames, globalNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(moduleLinks.Downloads.Scope, node, getNodeDownloadsScopeKeys)
				pruneModuleLinks(moduleLinks.Downloads.Scope, globalNode, getNodeDownloadsScopeKeys)
			}
		}
		newNode.Links = moduleDocumentation.Links
		if moduleDocumentation.NodeSelector != nil {
			childNode := &api.Node{
				NodeSelector: moduleDocumentation.NodeSelector,
			}
			childNode.SetParent(node)
			res, err := r.resolveNodeSelector(ctx, childNode, globalLinksConfig)
			if err != nil {
				return nil, err
			}
			for _, n := range res.Nodes {
				n.SetParent(node)
				n.SetParentsDownwards()
			}

			pruneChildNodesLinks(node, res.Nodes, globalLinksConfig)
			newNode.Nodes = append(newNode.Nodes, res.Nodes...)
		}
		return newNode, nil
	}

	nodes, err := handler.ResolveNodeSelector(ctx, node, node.NodeSelector.ExcludePaths, node.NodeSelector.ExcludeFrontMatter, node.NodeSelector.FrontMatter, node.NodeSelector.Depth)
	if err != nil {
		return nil, err
	}

	newNode.Nodes = nodes
	return newNode, nil
}

func (r *Reactor) resolveNodeName(ctx context.Context, node *api.Node) (string, error) {
	name := node.Name
	handler := r.ResourceHandlers.Get(node.Source)
	if handler == nil {
		return "", fmt.Errorf("No suitable handler registered for URL %s", node.Source)
	}
	if len(node.Name) == 0 {
		name = "$name"
	}
	resourceName, ext := handler.ResourceName(node.Source)
	id := uuid.New().String()
	name = strings.ReplaceAll(name, "$name", resourceName)
	name = strings.ReplaceAll(name, "$uuid", id)
	name = strings.ReplaceAll(name, "$ext", fmt.Sprintf(".%s", ext))
	return name, nil
}

func pruneModuleLinks(moduleLinks interface{}, node *api.Node, getParentLinks func(node *api.Node) map[string]struct{}) {
	v := reflect.ValueOf(moduleLinks)
	if v.Kind() != reflect.Map {
		return
	}

	for _, moduleLinkKey := range v.MapKeys() {
		for parent := node; parent != nil; parent = parent.Parent() {
			if parent.Links == nil {
				continue
			}

			if parentLinks := getParentLinks(parent); parentLinks != nil {
				if _, ok := parentLinks[moduleLinkKey.String()]; ok {
					klog.Warningf("Overriding module link %s", moduleLinkKey.String())
					v.SetMapIndex(moduleLinkKey, reflect.Value{})
				}
			}
		}
	}
}

func getNodeRewrites(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	keys := make(map[string]struct{})
	for k := range node.Links.Rewrites {
		keys[k] = struct{}{}
	}
	return keys
}

func getNodeDownloadsRenamesKeys(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	if node.Links.Downloads == nil {
		return nil
	}

	keys := make(map[string]struct{})
	for k := range node.Links.Downloads.Renames {
		keys[k] = struct{}{}
	}
	return keys
}

func getNodeDownloadsScopeKeys(node *api.Node) map[string]struct{} {
	if node.Links == nil {
		return nil
	}
	if node.Links.Downloads == nil {
		return nil
	}

	keys := make(map[string]struct{})
	for k := range node.Links.Downloads.Scope {
		keys[k] = struct{}{}
	}
	return keys
}

func pruneChildNodesLinks(parentNode *api.Node, nodes []*api.Node, globalLinksConfig *api.Links) {
	for _, node := range nodes {
		if node.Nodes != nil {
			pruneChildNodesLinks(node, node.Nodes, globalLinksConfig)
		}

		if node.Links != nil {
			globalNode := &api.Node{
				Links: globalLinksConfig,
			}
			pruneModuleLinks(node.Links.Rewrites, parentNode, getNodeRewrites)
			pruneModuleLinks(node.Links.Rewrites, globalNode, getNodeRewrites)
			if node.Links.Downloads != nil {
				pruneModuleLinks(node.Links.Downloads.Renames, parentNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(node.Links.Downloads.Renames, globalNode, getNodeDownloadsRenamesKeys)
				pruneModuleLinks(node.Links.Downloads.Scope, parentNode, getNodeDownloadsScopeKeys)
				pruneModuleLinks(node.Links.Downloads.Scope, globalNode, getNodeDownloadsScopeKeys)
			}
		}
	}
}
