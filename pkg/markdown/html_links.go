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
	htmlTagLinkRegex    = regexp.MustCompile(`<\b[^>]*?\b((?i)href|(?i)src)\s*=\s*(\"([^"]*\")|'[^']*'|([^'">\s]+))`)
	htmlTagLinkURLRegex = regexp.MustCompile(`((http|https|ftp|mailto):\/\/)?(\.?\/?[\w\.\-]+)+\/?([#?=&])?`)
)

// UpdateHTMLLinkRef is a callback function invoked by UpdateHTMLLinksRefs on
// each link (src|href attribute) found in an HTML tag.
// It is supplied the link reference destination and is expected to return
// a destination that will be used to update the link , if different, or error.
type UpdateHTMLLinkRef func(destination []byte) ([]byte, error)

// UpdateHTMLLinksRefs matches links in HTML tags in a document content and
// invokes the supplied updateRef callback function supplying the link
// reference as argument and using the function call result to update the
// destination of the matched link.
func UpdateHTMLLinksRefs(documentBytes []byte, updateRef UpdateHTMLLinkRef) ([]byte, error) {
	if updateRef == nil {
		return documentBytes, nil
	}
	var errors *multierror.Error
	documentBytes = htmlTagLinkRegex.ReplaceAllFunc(documentBytes, func(match []byte) []byte {
		var prefix, suffix string
		attrs := strings.SplitAfter(string(match), "=")
		url := attrs[len(attrs)-1]
		url = htmlTagLinkURLRegex.FindString(url)
		splits := strings.Split(string(match), url)
		prefix = splits[0]
		if len(splits) > 1 {
			suffix = strings.Split(string(match), url)[1]
		}
		destination, err := updateRef([]byte(url))
		if err != nil {
			errors = multierror.Append(err)
			return match
		}
		return []byte(fmt.Sprintf("%s%s%s", prefix, destination, suffix))
	})
	return documentBytes, errors.ErrorOrNil()
}
