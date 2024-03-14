// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhostsfakes

import (
	"context"
	"embed"
)

// FilesystemRegistry builds fake registry from directory
func FilesystemRegistry(dir embed.FS) *FakeRegistry {
	localHost := FakeRepositoryHost{}
	localHost.ReadCalls(func(ctx context.Context, url string) ([]byte, error) {
		return dir.ReadFile(url)
	})
	localHost.ToAbsLinkCalls(func(url, link string) (string, error) {
		return link, nil
	})
	registry := &FakeRegistry{}
	registry.GetReturns(&localHost, nil)
	return registry
}
