// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var (
	htmlLinkTagPattern = regexp.MustCompile(`(?P<Prefix><\b[^>]*?\b((?i)href|(?i)src)\s*=\s*)((\"(?P<Link>[^"]*)\")|('(?P<Link>[^']*)')|(?P<Link>[^'">\s]+))`)
)

// UpdateHTMLLinkRef is a callback function invoked by UpdateHTMLLinksRefs on
// each link (src|href attribute) found in an HTML tag.
// It is supplied the isImage flag and the link reference destination and is expected to return
// a destination that will be used to update the link , if different, or error.
type UpdateHTMLLinkRef func(isImage bool, destination []byte) ([]byte, error)

// UpdateHTMLLinksRefs matches links in HTML tags in a document content and
// invokes the supplied updateRef callback function supplying the link
// reference as argument and using the function call result to update the
// destination of the matched link.
func UpdateHTMLLinksRefs(documentBytes []byte, updateRef UpdateHTMLLinkRef) ([]byte, error) {
	if updateRef == nil {
		return documentBytes, nil
	}
	var errors *multierror.Error
	documentBytes = htmlLinkTagPattern.ReplaceAllFunc(documentBytes, func(match []byte) []byte {
		matchedLink := string(match)
		url := extractLink(matchedLink)
		if len(url) == 0 {
			return match
		}
		var prefix, suffix string
		prefixSufix := strings.Split(matchedLink, url)
		prefix = prefixSufix[0]
		isImage := strings.HasPrefix(prefix, "<img ")
		if len(prefixSufix) > 1 {
			suffix = prefixSufix[1]
		}
		destination, err := updateRef(isImage, []byte(url))
		if err != nil {
			errors = multierror.Append(errors, err)
			return match
		}
		newLinkTag := fmt.Sprintf("%s%s%s", prefix, destination, suffix)
		return []byte(newLinkTag)
	})
	return documentBytes, errors.ErrorOrNil()
}

func extractLink(matchedLink string) string {
	subMatch := htmlLinkTagPattern.FindStringSubmatch(matchedLink)
	var linkIndecies []int
	for i, name := range htmlLinkTagPattern.SubexpNames() {
		if name == "Link" {
			linkIndecies = append(linkIndecies, i)
		}
	}

	for _, linkIndex := range linkIndecies {
		if len(subMatch[linkIndex]) != 0 {
			return subMatch[linkIndex]
		}
	}

	return ""
}
