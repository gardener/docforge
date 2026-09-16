// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package hugo

import (
	"strings"

	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/internal/must"
	"github.com/gardener/docforge/pkg/manifest"
)

// HugoPrettyPath returns the Hugo pretty-URL path for n, trimming the .md
// extension and any index file stem (from indexFileNames) to produce a
// directory-style path (e.g. "docs/readme/" instead of "docs/README.md").
func HugoPrettyPath(n *manifest.Node, indexFileNames []string) string {
	name := n.Name()
	if n.Type == "dir" {
		return must.Succeed(link.Build(n.Path, name, "/"))
	}
	if !strings.HasSuffix(name, ".md") {
		return must.Succeed(link.Build(n.Path, name))
	}
	name = strings.TrimSuffix(name, ".md")
	name = strings.TrimSuffix(name, "_index")
	for _, idx := range indexFileNames {
		stem := strings.TrimSuffix(idx, ".md")
		name = strings.TrimSuffix(name, stem)
	}
	return must.Succeed(link.Build(n.Path, name, "/"))
}
