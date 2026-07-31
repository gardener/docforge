// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package titleresolver

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ResolveTitle determines the display title for a node using a three-level priority chain:
//  1. manifest frontmatter "title" (non-empty string) — highest priority, explicit override
//  2. document frontmatter "title" (non-empty string) — from the source .md file
//  3. filename derivation — index/README nodes use parentName (or "Root" for root index);
//     all others use nodeName. Applied: TrimSuffix ".md", replace "_"/"-" → " ", Title case.
//
// Pure function: no mutations, no I/O.
func ResolveTitle(manifestFM, docFM map[string]interface{}, nodeName string, indexFileNames []string, parentName string, isRootIndex bool) string {
	if t := stringFrom(manifestFM, "title"); t != "" {
		return t
	}
	if t := stringFrom(docFM, "title"); t != "" {
		return t
	}
	return deriveFromFilename(nodeName, indexFileNames, parentName, isRootIndex)
}

// stringFrom extracts a non-empty, non-whitespace-only string from a frontmatter map.
// Returns "" if the map is nil, the key is absent, the value is not a string, or the
// value is blank after trimming.
func stringFrom(fm map[string]interface{}, key string) string {
	if fm == nil {
		return ""
	}
	v, ok := fm[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// deriveFromFilename replicates the ComputeNodeTitle filename-derivation logic exactly:
//   - index/README nodes: use parentName if available; root index → "Root"
//   - all others: use nodeName
//   - transform: TrimSuffix ".md", replace "_" and "-" with " ", Title case (English)
func deriveFromFilename(nodeName string, indexFileNames []string, parentName string, isRootIndex bool) string {
	title := nodeName
	if isIndexFile(nodeName, indexFileNames) {
		if isRootIndex {
			return "Root"
		}
		if parentName != "" {
			title = parentName
		}
	}
	title = strings.TrimSuffix(title, ".md")
	title = strings.TrimSuffix(title, ".html")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, "-", " ")
	return cases.Title(language.English).String(title)
}

func isIndexFile(name string, indexFileNames []string) bool {
	lower := strings.ToLower(name)
	for _, s := range indexFileNames {
		if strings.ToLower(s) == lower {
			return true
		}
	}
	return lower == "_index.md"
}
