package reactor

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/google/uuid"
	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/hashicorp/go-multierror"

	"github.com/gardener/docforge/pkg/markdown"
)

var (
	htmlLinksRegexList = []*regexp.Regexp{
		regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`),
		regexp.MustCompile(`src=["\']?([^"\'>]+)["\']?`),
	}
)

// NodeContentProcessor operates on documents content to reconcile links and
// schedule linked resources downloads
type NodeContentProcessor struct {
	resourceAbsLinks map[string]string
	rwlock           sync.RWMutex
	localityDomain   localityDomain
	// ResourcesRoot specifies the root location for downloaded resource.
	// It is used to rewrite resource links in documents to relative paths.
	resourcesRoot      string
	DownloadController DownloadController
	failFast           bool
	markdownFmt        bool
	ResourceHandlers   resourcehandlers.Registry
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, ld localityDomain, downloadJob DownloadController, failFast bool, markdownFmt bool, resourceHandlers resourcehandlers.Registry) *NodeContentProcessor {
	if ld == nil {
		ld = localityDomain{}
	}
	c := &NodeContentProcessor{
		resourceAbsLinks:   make(map[string]string),
		localityDomain:     ld,
		resourcesRoot:      resourcesRoot,
		DownloadController: downloadJob,
		failFast:           failFast,
		markdownFmt:        markdownFmt,
		ResourceHandlers:   resourceHandlers,
	}
	return c
}

//convenience wrapper adding logging
func (c *NodeContentProcessor) schedule(ctx context.Context, link, resourceName, from string) {
	klog.V(6).Infof("[%s] Linked resource scheduled for download: %s\n", from, link)
	c.DownloadController.Schedule(ctx, link, resourceName)
}

// ReconcileLinks analyzes a document referenced by a node's contentSourcePath
// and processes its links to other resources to resolve their inconsistencies.
// The processing might involve rewriting links to relative and having new
// destinations, or rewriting them to absolute, as well as downloading some of
// the linked resources.
// The function returns the processed document or error.
func (c *NodeContentProcessor) ReconcileLinks(ctx context.Context, node *api.Node, contentSourcePath string, documentBlob []byte) ([]byte, error) {
	klog.V(6).Infof("[%s] Reconciling links for %s\n", node.Name, contentSourcePath)

	fm, contentBytes, err := markdown.StripFrontMatter(documentBlob)
	if err != nil {
		return nil, err
	}

	documentBytes, err := c.reconcileMDLinks(ctx, node, contentBytes, contentSourcePath)
	if err != nil {
		return nil, err
	}
	if _, err := c.reconcileHTMLLinks(ctx, node, documentBytes, contentSourcePath); err != nil {
		return nil, err
	}

	documentBytes, err = markdown.InsertFrontMatter(fm, documentBytes)
	if err != nil {
		return nil, err
	}
	return documentBytes, err
}

func (c *NodeContentProcessor) reconcileMDLinks(ctx context.Context, docNode *api.Node, contentBytes []byte, contentSourcePath string) ([]byte, error) {
	var errors *multierror.Error
	contentBytes, _ = markdown.TransformLinks(contentBytes, func(destination []byte) ([]byte, error) {
		var (
			_destination string
			downloadLink string
			resourceName string
			err          error
		)
		if _destination, downloadLink, resourceName, err = c.processLink(ctx, docNode, string(destination), contentSourcePath); err != nil {
			errors = multierror.Append(err)
			if c.failFast {
				return nil, err
			}
		}
		if len(downloadLink) > 0 {
			c.schedule(ctx, downloadLink, resourceName, contentSourcePath)
		}
		return []byte(_destination), nil
	})
	if c.failFast && errors != nil && errors.Len() > 0 {
		return nil, errors.ErrorOrNil()
	}

	return contentBytes, errors.ErrorOrNil()
}

// replace html raw links of any sorts.
func (c *NodeContentProcessor) reconcileHTMLLinks(ctx context.Context, docNode *api.Node, documentBytes []byte, contentSourcePath string) ([]byte, error) {
	var errors *multierror.Error
	for _, regex := range htmlLinksRegexList {
		documentBytes = regex.ReplaceAllFunc(documentBytes, func(match []byte) []byte {
			attr := strings.Split(string(match), "=")
			name := attr[0]
			url := attr[1]
			if len(url) > 0 {
				url = strings.TrimPrefix(url, "\"")
				url = strings.TrimSuffix(url, "\"")
			}
			destination, downloadURL, resourceName, err := c.processLink(ctx, docNode, url, contentSourcePath)
			klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, url, destination)
			if len(downloadURL) > 0 {
				c.schedule(ctx, downloadURL, resourceName, contentSourcePath)
			}
			if err != nil {
				errors = multierror.Append(err)
				return match
			}
			return []byte(fmt.Sprintf("%s=%s", name, destination))
		})
	}
	return documentBytes, errors.ErrorOrNil()
}

// returns destination, downloadURL, resourceName, err
func (c *NodeContentProcessor) processLink(ctx context.Context, node *api.Node, destination string, contentSourcePath string) (string, string, string, error) {
	if strings.HasPrefix(destination, "#") {
		return destination, "", "", nil
	}

	handler := c.ResourceHandlers.Get(contentSourcePath)
	if handler == nil {
		return destination, "", "", nil
	}
	absLink, err := handler.BuildAbsLink(contentSourcePath, destination)
	if err != nil {
		return "", "", "", err
	}
	//TODO: this is URI-specific (URLs only) - fixme
	u, err := url.Parse(absLink)
	if err != nil {
		return "", "", "", err
	}
	_a := absLink
	var (
		include, exclude bool
	)
	// check if the links is not eligible by explicit exclude
	if node.Links != nil && len(node.Links.Exclude) > 0 {
		for _, rx := range node.Links.Exclude {
			if exclude, err = regexp.MatchString(rx, absLink); err != nil {
				klog.V(6).Infof("[%s] exclude pattern match %s failed for %s\n", contentSourcePath, node.Links.Exclude, absLink)
			}
			if exclude {
				break
			}
		}
	}
	absLink, inLD := c.localityDomain.MatchPathInLocality(absLink, c.ResourceHandlers)
	if _a != absLink {
		klog.V(6).Infof("[%s] Link converted %s -> %s\n", contentSourcePath, _a, absLink)
	}
	// check if the links is eligible by explicit include
	if node.Links != nil && len(node.Links.Include) > 0 {
		for _, rx := range node.Links.Include {
			if include, err = regexp.MatchString(rx, absLink); err != nil {
				klog.V(6).Infof("[%s] exclude pattern match %s failed for %s\n", contentSourcePath, node.Links.Exclude, absLink)
			}
			if include {
				break
			}
		}
		exclude = !include
	}
	if exclude {
		return absLink, "", "", nil
	}
	// Links to other documents are enforced relative when
	// linking documents from the node structure.
	// Links to other documents are changed to match the linking
	// document version when appropriate or left untouched.
	if strings.HasSuffix(u.Path, ".md") {
		//TODO: this is URI-specific (URLs only) - fixme
		l := strings.TrimSuffix(absLink, "?")
		l = strings.TrimSuffix(l, "#")
		if existingNode := api.FindNodeByContentSource(l, node); existingNode != nil {
			relPathBetweenNodes := node.RelativePath(existingNode)
			if destination != relPathBetweenNodes {
				klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, destination, relPathBetweenNodes)
			}
			destination = relPathBetweenNodes
			return destination, "", "", nil
		}
		return absLink, "", "", nil
	}
	// Links to resources are assessed for download eligibility
	// and if applicable their destination is updated as relative
	// path to predefined location for resources
	if absLink != "" && (inLD || include) {
		resourceName := c.generateResourceName(absLink)
		_d := destination
		destination = buildDestination(node, resourceName, c.resourcesRoot)
		if _d != destination {
			klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, _d, destination)
		}
		return destination, absLink, resourceName, nil
	}
	if destination != absLink {
		klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
	}
	return absLink, "", "", nil
}

// Builds destination path for links from node to resource in root path
// If root is not specified as document root (with leading "/"), the
// returned destinations are relative paths from the node to the resource
// in root, e.g. "../../__resources/image.png", where root is "__resources".
// If root is document root path, destinations are paths from the root,
// e.g. "/__resources/image.png", where root is "/__resources".
func buildDestination(node *api.Node, resourceName, root string) string {
	if strings.HasPrefix(root, "/") {
		return root + "/" + resourceName
	}
	resourceRelPath := fmt.Sprintf("%s/%s", root, resourceName)
	parentsSize := len(node.Parents())
	for ; parentsSize > 0; parentsSize-- {
		resourceRelPath = "../" + resourceRelPath
	}
	return resourceRelPath
}

func (c *NodeContentProcessor) generateResourceName(path string) string {
	var (
		ok           bool
		resourceName string
	)

	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	if resourceName, ok = c.resourceAbsLinks[path]; !ok {
		separatedSource := strings.Split(path, "/")
		resource := separatedSource[len(separatedSource)-1]
		resourceFileExtension := filepath.Ext(resource)
		resourceName = uuid.New().String() + resourceFileExtension
		c.resourceAbsLinks[path] = resourceName
	}
	return resourceName
}
