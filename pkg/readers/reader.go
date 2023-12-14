// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package readers

import (
	"context"
	"fmt"

	resourcehandlers "github.com/gardener/docforge/pkg/readers/repositoryhosts"
)

// Reader reads the bytes' data from a given source URI
//
//counterfeiter:generate . Reader
type Reader interface {
	Read(ctx context.Context, source string) ([]byte, error)
}

// GenericReader is generic implementation for Reader interface
type GenericReader struct {
	RepositoryHosts resourcehandlers.Registry
	// if IsGitHubInfo is true the GitHub info for the resource is read
	IsGitHubInfo bool
}

// Read reads from the resource at the source URL delegating
// the actual operation to a suitable resource handler
func (g *GenericReader) Read(ctx context.Context, source string) ([]byte, error) {
	if handler := g.RepositoryHosts.Get(source); handler != nil {
		if g.IsGitHubInfo {
			return handler.ReadGitInfo(ctx, source)
		}
		return handler.Read(ctx, source)
	}
	return nil, fmt.Errorf("failed to get handler to read from %s", source)
}
