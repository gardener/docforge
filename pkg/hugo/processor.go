// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"k8s.io/klog/v2"

	mdutil "github.com/gardener/docforge/pkg/markdown"
)

var (
	htmlTagLinkRegex    = regexp.MustCompile(`<\b[^>]*?\b((?i)href|(?i)src)\s*=\s*(\"([^"]*\")|'[^']*'|([^'">\s]+))`)
	htmlTagLinkURLRegex = regexp.MustCompile(`((http|https|ftp|mailto):\/\/)?(\.?\/?[\w\.\-]+)+\/?([#?=&])?`)
)

// Processor is a processor implementation responsible to rewrite links
// on document that use source format (<path>/<name>.md) to destination format
// (<path>/<name> for sites configured for pretty URLs and <path>/<name>.html
// for sites configured for ugly URLs)
type Processor struct {
	// PrettyUrls indicates if links will rewritten for Hugo will be
	// formatted for pretty url support or not. Pretty urls in Hugo
	// place built source content in index.html, which resides in a path segment with
	// the name of the file, making request URLs more resource-oriented.
	// Example: (source) sample.md -> (build) sample/index.html -> (runtime) ./sample
	PrettyUrls bool
	// IndexFileNames defines a list of file names that indicate
	// their content can be used as Hugo section files (_index.md).
	IndexFileNames []string
}

// Process implements Processor#Process
func (f *Processor) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	fm, contentBytes, err := markdown.StripFrontMatter(documentBlob)
	if err != nil {
		return nil, err
	}
	if documentBlob, err = mdutil.UpdateMarkdownLinks(contentBytes, func(markdownType mdutil.Type, destination, text, title []byte) ([]byte, []byte, []byte, error) {
		// quick sanity check for ill-parsed links if any
		if destination == nil {
			return destination, text, title, nil
		}
		return f.rewriteDestination(destination, text, title, node.Name)
	}); err != nil {
		return nil, err
	}
	if documentBlob, err = mdutil.UpdateHTMLLinksRefs(documentBlob, func(url []byte) ([]byte, error) {
		destination, _, _, err := f.rewriteDestination([]byte(url), []byte(""), []byte(""), node.Name)
		if err != nil {
			return url, err
		}
		return destination, err
	}); err != nil {
		return nil, err
	}
	documentBlob, err = markdown.InsertFrontMatter(fm, documentBlob)
	if err != nil {
		return nil, err
	}

	return documentBlob, nil
}

func (f *Processor) rewriteDestination(destination, text, title []byte, nodeName string) ([]byte, []byte, []byte, error) {
	if len(destination) == 0 {
		return destination, nil, nil, nil
	}
	link := string(destination)
	link = strings.TrimSpace(link)
	// trim leading and trailing quotes if any
	link = strings.TrimSuffix(strings.TrimPrefix(link, "\""), "\"")
	u, err := url.Parse(link)
	if err != nil {
		klog.Warning("Invalid link:", link)
		return destination, text, title, nil
	}
	if !u.IsAbs() && !strings.HasPrefix(link, "/") && !strings.HasPrefix(link, "#") {
		_l := link
		if f.PrettyUrls {
			link = strings.TrimSuffix(u.Path, ".md")
			link = strings.TrimPrefix(link, "./")
			// Remove the last path segment if it is readme, index or _index
			// The Hugo writer will rename those files to _index.md and runtime
			// references will be to the sections in which they reside.
			for _, s := range f.IndexFileNames {
				if strings.HasSuffix(strings.ToLower(link), s) {
					pathSegments := strings.Split(link, "/")
					if len(pathSegments) > 0 {
						pathSegments = pathSegments[:len(pathSegments)-1]
						link = strings.Join(pathSegments, "/")
					}
					break
				}
			}
			link = fmt.Sprintf("../%s", link)
		} else {
			if strings.HasSuffix(u.Path, ".md") {
				link = strings.TrimSuffix(u.Path, ".md")
				// TODO: propagate fragment and query if any
				link = fmt.Sprintf("%s.html", link)
			}
		}
		if _l != link {
			klog.V(6).Infof("[%s] Rewriting node link for Hugo: %s -> %s \n", nodeName, _l, link)
		}
		return []byte(link), text, title, nil
	}
	return destination, text, title, nil
}
