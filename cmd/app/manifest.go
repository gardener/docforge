// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
)

// Manifest reads the resource at uri, resolves it as template applying vars,
// and finally parses it into api.Documentation model
func manifest(ctx context.Context, uri string, resourceHandlers []resourcehandlers.ResourceHandler) (*api.Documentation, error) {
	var handler resourcehandlers.ResourceHandler
	uri = strings.TrimSpace(uri)
	registry := resourcehandlers.NewRegistry(resourceHandlers...)
	if handler = registry.Get(uri); handler == nil {
		return nil, fmt.Errorf("no suitable reader found for %s. Is this path correct?", uri)
	}
	return handler.ResolveDocumentation(ctx, uri)
}
