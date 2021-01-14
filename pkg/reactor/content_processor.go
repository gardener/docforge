// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	htmlTagLinkRegex    = regexp.MustCompile(`<\b[^>]*?\b((?i)href|(?i)src)\s*=\s*(\"([^"]*\")|'[^']*'|([^'">\s]+))`)
	htmlTagLinkURLRegex = regexp.MustCompile(`((http|https|ftp|mailto):\/\/)?(\.?\/?[\w\.\-]+)+\/?([#?=&])?`)
)

// NodeContentProcessor operates on documents content to reconcile links and
// schedule linked resources downloads
type NodeContentProcessor interface {
	ReconcileLinks(ctx context.Context, node *api.Node, contentSourcePath string, documentBlob []byte) ([]byte, error)
	GetDownloadController() DownloadController
}

type nodeContentProcessor struct {
	resourceAbsLinks  map[string]string
	rwlock            sync.RWMutex
	globalLinksConfig *api.Links
	// resourcesRoot specifies the root location for downloaded resource.
	// It is used to rewrite resource links in documents to relative paths.
	resourcesRoot      string
	downloadController DownloadController
	failFast           bool
	rewriteEmbedded    bool
	resourceHandlers   resourcehandlers.Registry
}

// NewNodeContentProcessor creates NodeContentProcessor objects
func NewNodeContentProcessor(resourcesRoot string, globalLinksConfig *api.Links, downloadJob DownloadController, failFast bool, rewriteEmbedded bool, resourceHandlers resourcehandlers.Registry) NodeContentProcessor {
	c := &nodeContentProcessor{
		resourceAbsLinks:   make(map[string]string),
		globalLinksConfig:  globalLinksConfig,
		resourcesRoot:      resourcesRoot,
		downloadController: downloadJob,
		failFast:           failFast,
		rewriteEmbedded:    rewriteEmbedded,
		resourceHandlers:   resourceHandlers,
	}
	return c
}

func (c *nodeContentProcessor) GetDownloadController() DownloadController {
	return c.downloadController
}

//convenience wrapper
func (c *nodeContentProcessor) schedule(ctx context.Context, download *DownloadTask) error {
	return c.downloadController.Schedule(ctx, download)
}

// ReconcileLinks analyzes a document referenced by a node's contentSourcePath
// and processes its links to other resources to resolve their inconsistencies.
// The processing might involve rewriting links to relative and having new
// destinations, or rewriting them to absolute, as well as downloading some of
// the linked resources.
// The function returns the processed document or error.
func (c *nodeContentProcessor) ReconcileLinks(ctx context.Context, node *api.Node, contentSourcePath string, documentBlob []byte) ([]byte, error) {
	klog.V(6).Infof("[%s] Reconciling links for %s\n", node.Name, contentSourcePath)

	fm, contentBytes, err := markdown.StripFrontMatter(documentBlob)
	if err != nil {
		return nil, err
	}

	documentBytes, err := c.reconcileMDLinks(ctx, node, contentBytes, contentSourcePath)
	if err != nil {
		return nil, err
	}
	if documentBytes, err = c.reconcileHTMLLinks(ctx, node, documentBytes, contentSourcePath); err != nil {
		return nil, err
	}

	documentBytes, err = markdown.InsertFrontMatter(fm, documentBytes)
	if err != nil {
		return nil, err
	}
	return documentBytes, err
}

func (c *nodeContentProcessor) reconcileMDLinks(ctx context.Context, docNode *api.Node, contentBytes []byte, contentSourcePath string) ([]byte, error) {
	var errors *multierror.Error
	contentBytes, _ = markdown.UpdateMarkdownLinks(contentBytes, func(markdownType markdown.Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
		var (
			_destination, _text, _title *string
			download                    *DownloadTask
			err                         error
		)
		// quick sanity check for ill-parsed links if any
		if destination == nil {
			klog.V(6).Infof("[%s] skipping ill parsed link: destination[%s] text[%s] title[%s]", contentSourcePath, string(destination), string(text), string(title))
			return destination, text, title, nil
		}
		if _destination, _text, _title, download, err = c.resolveLink(ctx, docNode, string(destination), contentSourcePath); err != nil {
			errors = multierror.Append(errors, err)
			if c.failFast {
				return destination, text, title, err
			}
		}
		if download != nil {
			if err := c.schedule(ctx, download); err != nil {
				return destination, text, title, err
			}
		}
		// rewrite abs links to embedded images to their raw format if necessary, to
		// ensure they are embedable
		if c.rewriteEmbedded && markdownType == markdown.Image && _destination != nil {
			if err = c.rawImage(_destination); err != nil {
				return destination, text, title, err
			}
		}
		// write node processing stats for document nodes
		if docNode != nil {
			if _destination != nil && *_destination != string(destination) {
				if len(*_destination) == 0 {
					*_destination = "*deleted*"
				}
				recordLinkStats(docNode, "Links", fmt.Sprintf("%s -> %s", string(destination), *_destination))
			} else {
				if _text != nil && len(*_text) == 0 {
					recordLinkStats(docNode, "Links", fmt.Sprintf("%s -> *deleted*", string(destination)))
				}
			}
		}
		if _text != nil {
			text = []byte(*_text)
		}
		if _title != nil {
			title = []byte(*_title)
		}
		if _destination == nil {
			return nil, text, title, nil
		}
		return []byte(*_destination), text, title, nil
	})
	if c.failFast && errors != nil && errors.Len() > 0 {
		return nil, errors.ErrorOrNil()
	}

	return contentBytes, errors.ErrorOrNil()
}

// replace html raw links of any sorts.
func (c *nodeContentProcessor) reconcileHTMLLinks(ctx context.Context, docNode *api.Node, documentBytes []byte, contentSourcePath string) ([]byte, error) {
	var errors *multierror.Error
	documentBytes, _ = markdown.UpdateHTMLLinksRefs(documentBytes, func(url []byte) ([]byte, error) {
		destination, _, _, download, err := c.resolveLink(ctx, docNode, string(url), contentSourcePath)
		if err != nil {
			errors = multierror.Append(errors, err)
			return url, nil
		}
		if docNode != nil && destination != nil {
			if string(url) != *destination {
				recordLinkStats(docNode, "Links", fmt.Sprintf("%s -> %s", url, *destination))
			} else {
				recordLinkStats(docNode, "Links", "")
			}
		}
		if download != nil {
			if err := c.schedule(ctx, download); err != nil {
				errors = multierror.Append(errors, err)
				return []byte(*destination), nil
			}
		}
		return []byte(*destination), nil
	})
	return documentBytes, errors.ErrorOrNil()
}

// returns destination, text (alt-text for images), title, download(url, downloadName), err
func (c *nodeContentProcessor) resolveLink(ctx context.Context, node *api.Node, destination string, contentSourcePath string) (*string, *string, *string, *DownloadTask, error) {
	var (
		substituteDestination, version, text, title *string
		downloadResourceName, absLink               string
		ok                                          bool
		globalRewrites                              map[string]*api.LinkRewriteRule
	)
	if strings.HasPrefix(destination, "#") || strings.HasPrefix(destination, "mailto:") {
		return &destination, nil, nil, nil, nil
	}

	// validate destination
	u, err := urls.Parse(destination)
	if err != nil {
		return nil, text, title, nil, err
	}
	if u.IsAbs() {
		// can we handle changes to this destination?
		if c.resourceHandlers.Get(destination) == nil {
			// we don't have a handler for it. Leave it be.
			return &destination, text, title, nil, err
		}
		absLink = destination
	}
	if len(absLink) == 0 {
		// build absolute path for the destination using contentSourcePath as base
		handler := c.resourceHandlers.Get(contentSourcePath)
		if handler == nil {
			return &destination, text, title, nil, nil
		}
		if absLink, err = handler.BuildAbsLink(contentSourcePath, destination); err != nil {
			return nil, text, title, nil, err
		}
	}
	// rewrite link if required
	if gLinks := c.globalLinksConfig; gLinks != nil {
		globalRewrites = gLinks.Rewrites
	}
	_a := absLink
	if node != nil {
		if version, substituteDestination, text, title, ok = MatchForLinkRewrite(absLink, node, globalRewrites); ok {
			if substituteDestination != nil {
				if len(*substituteDestination) == 0 {
					// quit early. substitution is a request to remove this link
					s := ""
					return nil, &s, nil, nil, nil
				}
				absLink = *substituteDestination
			}
			if version != nil {
				handler := c.resourceHandlers.Get(absLink)
				if handler == nil {
					return &absLink, text, title, nil, nil
				}
				if absLink, err = handler.SetVersion(absLink, *version); err != nil {
					klog.Warningf("Failed to set version %s to %s: %s\n", *version, absLink, err.Error())
					return &absLink, text, title, nil, nil
				}
			}
		}
	}

	// validate potentially rewritten links
	u, err = urls.Parse(absLink)
	if err != nil {
		return nil, text, title, nil, err
	}
	if _a != absLink {
		klog.V(6).Infof("[%s] Link rewritten %s -> %s\n", contentSourcePath, _a, absLink)
	}

	if node != nil {
		// Links to other documents are enforced relative when
		// linking documents from the node structure.
		// Check if md extension to reduce the walkthroughs
		if u.Extension == "md" {
			if existingNode := api.FindNodeBySource(absLink, node); existingNode != nil {
				relPathBetweenNodes := node.RelativePath(existingNode)
				if destination != relPathBetweenNodes {
					klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, destination, relPathBetweenNodes)
				}
				return &relPathBetweenNodes, text, title, nil, nil
			}
			return &absLink, text, title, nil, nil
		}

		// Links to resources that are not structure document nodes are
		// assessed for download eligibility and if applicable their
		// destination is updated to relative path to predefined location
		// for resources.
		var globalDownloadsConfig *api.Downloads
		if c.globalLinksConfig != nil {
			globalDownloadsConfig = c.globalLinksConfig.Downloads
		}
		if downloadResourceName, ok = MatchForDownload(u, node, globalDownloadsConfig); ok {
			resourceName := c.getDownloadResourceName(u, downloadResourceName)
			_d := destination
			destination = buildDownloadDestination(node, resourceName, c.resourcesRoot)
			if _d != destination {
				klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, _d, destination)
			}
			return &destination, text, title, &DownloadTask{
				absLink,
				resourceName,
				contentSourcePath,
				_d,
			}, nil
		}
	}

	if destination != absLink {
		klog.V(6).Infof("[%s] %s -> %s\n", contentSourcePath, destination, absLink)
	}

	return &absLink, text, title, nil, nil
}

// rewrite abs links to embedded objects to their raw link format if necessary, to
// ensure they are embedable
func (c *nodeContentProcessor) rawImage(link *string) (err error) {
	var (
		u *url.URL
	)
	if u, err = url.Parse(*link); err != nil {
		return
	}
	if !u.IsAbs() {
		return nil
	}
	handler := c.resourceHandlers.Get(*link)
	if handler == nil {
		return nil
	}
	if *link, err = handler.GetRawFormatLink(*link); err != nil {
		return
	}
	return nil
}

// Builds destination path for links from node to resource in root path
// If root is not specified as document root (with leading "/"), the
// returned destinations are relative paths from the node to the resource
// in root, e.g. "../../__resources/image.png", where root is "__resources".
// If root is document root path, destinations are paths from the root,
// e.g. "/__resources/image.png", where root is "/__resources".
func buildDownloadDestination(node *api.Node, resourceName, root string) string {
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

// Check for cached resource name first and return that if found. Otherwise,
// return the downloadName
func (c *nodeContentProcessor) getDownloadResourceName(u *urls.URL, downloadName string) string {
	c.rwlock.Lock()
	defer c.rwlock.Unlock()
	if cachedDownloadName, ok := c.resourceAbsLinks[u.Path]; ok {
		return cachedDownloadName
	}
	return downloadName
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
