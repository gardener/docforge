package reactor

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"k8s.io/klog/v2"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/urls"
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
	localityDomain   *localityDomain
	// ResourcesRoot specifies the root location for downloaded resource.
	// It is used to rewrite resource links in documents to relative paths.
	resourcesRoot      string
	DownloadController DownloadController
	failFast           bool
	markdownFmt        bool
	rewriteEmbedded    bool
	ResourceHandlers   resourcehandlers.Registry
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, ld *localityDomain, downloadJob DownloadController, failFast bool, markdownFmt bool, rewriteEmbedded bool, resourceHandlers resourcehandlers.Registry) *NodeContentProcessor {
	if ld == nil {
		ld = &localityDomain{
			mapping: map[string]*localityDomainValue{},
		}
	}
	c := &NodeContentProcessor{
		resourceAbsLinks:   make(map[string]string),
		localityDomain:     ld,
		resourcesRoot:      resourcesRoot,
		DownloadController: downloadJob,
		failFast:           failFast,
		markdownFmt:        markdownFmt,
		rewriteEmbedded:    rewriteEmbedded,
		ResourceHandlers:   resourceHandlers,
	}
	return c
}

//convenience wrapper adding logging
func (c *NodeContentProcessor) schedule(ctx context.Context, download *Download, from string) {
	klog.V(6).Infof("[%s] Linked resource scheduled for download: %s\n", from, download.url)
	c.DownloadController.Schedule(ctx, download.url, download.resourceName)
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
	contentBytes, _ = markdown.UpdateLinkRefs(contentBytes, func(destination, text, title []byte) ([]byte, []byte, []byte, error) {
		var (
			_destination  string
			_text, _title *string
			download      *Download
			err           error
		)
		if _destination, _text, _title, download, err = c.resolveLink(ctx, docNode, string(destination), contentSourcePath); err != nil {
			errors = multierror.Append(err)
			if c.failFast {
				return destination, text, title, err
			}
		}
		if docNode != nil {
			if _destination != string(destination) {
				recordLinkStats(docNode, "Links", fmt.Sprintf("%s -> %s", string(destination), _destination))
			} else {
				recordLinkStats(docNode, "Links", "")
			}
		}
		if download != nil {
			c.schedule(ctx, download, contentSourcePath)
		}
		if _text != nil {
			text = []byte(*_text)
		}
		if _title != nil {
			title = []byte(*_title)
		}
		if len(_destination) < 1 {
			return nil, text, title, nil
		}
		return []byte(_destination), text, title, nil
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
			destination, _, _, download, err := c.resolveLink(ctx, docNode, url, contentSourcePath)
			klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, url, destination)
			if docNode != nil {
				if url != destination {
					recordLinkStats(docNode, "Links", fmt.Sprintf("%s -> %s", url, destination))
				} else {
					recordLinkStats(docNode, "Links", "")
				}
			}
			if download != nil {
				c.schedule(ctx, download, contentSourcePath)
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

// Download represents a resource that can be downloaded
type Download struct {
	url          string
	resourceName string
}

// returns destination, text (alt-text for images), title, download(url, downloadName), err
func (c *NodeContentProcessor) resolveLink(ctx context.Context, node *api.Node, destination string, contentSourcePath string) (string, *string, *string, *Download, error) {
	var (
		text, title, substituteDestination *string
		hasSubstition                      bool
		inLD                               bool
		absLink                            string
	)
	if strings.HasPrefix(destination, "#") || strings.HasPrefix(destination, "mailto:") {
		return destination, nil, nil, nil, nil
	}

	// validate destination
	u, err := url.Parse(destination)
	if err != nil {
		return "", text, title, nil, err
	}
	// can we handle this destination?
	if u.IsAbs() && c.ResourceHandlers.Get(destination) == nil {
		// It's a valid absolute link that is not in our scope. Leave it be.
		return destination, text, title, nil, err
	}

	handler := c.ResourceHandlers.Get(contentSourcePath)
	if handler == nil {
		return destination, text, title, nil, nil
	}
	absLink, err = handler.BuildAbsLink(contentSourcePath, destination)
	if err != nil {
		return "", text, title, nil, err
	}

	if hasSubstition, substituteDestination, text, title = substitute(absLink, node); hasSubstition && substituteDestination != nil {
		if len(*substituteDestination) == 0 {
			// quit early. substitution is a request to remove this link
			return "", text, title, nil, nil
		}
		absLink = *substituteDestination
	}

	//TODO: this is URI-specific (URLs only) - fixme
	u, err = url.Parse(absLink)
	if err != nil {
		return "", text, title, nil, err
	}
	_a := absLink

	resolvedLD := c.localityDomain
	if node != nil {
		resolvedLD = resolveLocalityDomain(node, c.localityDomain)
	}
	if resolvedLD != nil {
		absLink, inLD = resolvedLD.MatchPathInLocality(absLink, c.ResourceHandlers)
	}
	if _a != absLink {
		klog.V(6).Infof("[%s] Link converted %s -> %s\n", contentSourcePath, _a, absLink)
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
			return destination, text, title, nil, nil
		}
		return absLink, text, title, nil, nil
	}

	// Links to resources are assessed for download eligibility
	// and if applicable their destination is updated as relative
	// path to predefined location for resources
	if absLink != "" && inLD {
		resourceName := c.generateResourceName(absLink, resolvedLD)
		_d := destination
		destination = buildDestination(node, resourceName, c.resourcesRoot)
		if _d != destination {
			klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, _d, destination)
		}
		return destination, text, title, &Download{absLink, resourceName}, nil
	}

	if c.rewriteEmbedded {
		// rewrite abs links to embedded objects to their raw format if necessary, to
		// ensure they are embedable
		if absLink, err = handler.GetRawFormatLink(absLink); err != nil {
			return "", text, title, nil, err
		}
	}

	if destination != absLink {
		klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
	}

	return absLink, text, title, nil, nil
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

func (c *NodeContentProcessor) generateResourceName(absURL string, resolvedLD *localityDomain) string {
	var (
		ok           bool
		resourceName string
	)
	u, _ := urls.Parse(absURL)
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	if resourceName, ok = c.resourceAbsLinks[u.Path]; !ok {
		resourceName = u.ResourceName
		if len(u.Extension) > 0 {
			resourceName = fmt.Sprintf("%s.%s", u.ResourceName, u.Extension)
		}
		resourceName = resolvedLD.GetDownloadedResourceName(u)
		c.resourceAbsLinks[absURL] = resourceName
	}
	return resourceName
}

// returns substitution found, destination, text, title
func substitute(absLink string, node *api.Node) (ok bool, destination *string, text *string, title *string) {
	if node == nil {
		return false, nil, nil, nil
	}
	if substitutes := node.LinksSubstitutes; substitutes != nil {
		for substituteK, substituteV := range substitutes {
			// remove trailing slashes to avoid inequality only due to that
			l := strings.TrimSuffix(absLink, "/")
			s := strings.TrimSuffix(substituteK, "/")
			if s == l {
				return true, substituteV.Destination, substituteV.Text, substituteV.Title
			}
		}
	}
	return false, nil, nil, nil
}

// recordLinkStats records link stats for a node
func recordLinkStats(node *api.Node, title, details string) {
	var (
		stat *api.Stat
	)
	nodeStats := node.GetStats()
	if nodeStats != nil {
		for _, _stat := range nodeStats {
			if _stat.Title == title {
				stat = _stat
				break
			}
		}
	}
	if stat == nil {
		stat = &api.Stat{
			Title: title,
		}
		if len(details) > 0 {
			stat.Details = []string{details}
		} else {
			stat.Details = []string{}
		}
		stat.Figures = fmt.Sprintf("%d link rewrites", len(stat.Details))
		node.AddStats(stat)
		return
	}
	if len(details) > 0 {
		stat.Details = append(stat.Details, details)
	}
	stat.Figures = fmt.Sprintf("%d link rewrites", len(stat.Details))
}
