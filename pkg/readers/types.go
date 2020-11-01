// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package readers

import "context"

// ContextResourceReader implements reading a resource
// from uri into byte array, in a context.
type ContextResourceReader interface {
	// Read a resource content at uri into a byte array
	Read(ctx context.Context, uri string) ([]byte, error)
}

// GitInfoReader implements reading Git information about a resource
// at URI into a byte array and within context
type GitInfoReader interface {
	// Read git info
	ReadGitInfo(ctx context.Context, uri string) ([]byte, error)
}
