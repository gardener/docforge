// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver

import (
	"slices"
	"strings"
)

// ReAnchorRootAbsolute re-anchors a root-absolute link (e.g. "/docs/x.png") to the prefix docforge stripped
// during the original aggregation: resourcePath up to and including the first configured structural dir.
// Returns link unchanged when it is not root-absolute or when no structural dir is present in resourcePath.
func ReAnchorRootAbsolute(link, resourcePath string, structuralDirs []string) string {
	if len(link) < 2 || link[0] != '/' || link[1] == '/' {
		return link // not "/xxx": empty, "/", "//host", or relative
	}
	pathSegs := strings.Split(resourcePath, "/")
	idx := -1
	for i, seg := range pathSegs {
		if slices.Contains(structuralDirs, seg) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return link // no structural dir in resourcePath -> nothing to reconstruct
	}
	return "/" + strings.Join(pathSegs[:idx+1], "/") + link
}
