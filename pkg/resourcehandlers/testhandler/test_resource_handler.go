// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package testhandler

import (
	"context"

	"github.com/gardener/docforge/pkg/api"
)

//TestResourceHandler ...
type TestResourceHandler struct {
	accept               func(uri string) bool
	resolveNodeSelector  func(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)
	resolveDocumentation func(ctx context.Context, uri string) (*api.Documentation, error)
}

//NewTestResouceHandlere ...
func NewTestResouceHandlere() *TestResourceHandler {
	return &TestResourceHandler{}
}

//Accept ...
func (t *TestResourceHandler) Accept(uri string) bool {
	if t.accept != nil {
		return t.accept(uri)
	}
	return true
}

//WithAccept ...
func (t *TestResourceHandler) WithAccept(accept func(uri string) bool) *TestResourceHandler {
	t.accept = accept
	return t
}

//WithResolveNodeSelector ...
func (t *TestResourceHandler) WithResolveNodeSelector(resolveNodeSelector func(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)) *TestResourceHandler {
	t.resolveNodeSelector = resolveNodeSelector
	return t
}

//ResolveNodeSelector ...
func (t *TestResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	if t.resolveNodeSelector != nil {
		return t.resolveNodeSelector(ctx, node, excludePaths, frontMatter, excludeFrontMatter, depth)
	}
	return []*api.Node{}, nil
}

// Read a resource content at uri into a byte array
func (t *TestResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return []byte{}, nil
}

// ReadGitInfo ..
func (t *TestResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return []byte{}, nil
}

// ResourceName returns a breakdown of a resource name in the link, consisting
// of name and potentially and extention without the dot.
func (t *TestResourceHandler) ResourceName(link string) (string, string) {
	return "", ""
}

// BuildAbsLink should return an absolute path of a relative link in regards of the provided
// source
func (t *TestResourceHandler) BuildAbsLink(source, link string) (string, error) {
	return "", nil
}

// GetRawFormatLink returns a link to an embedable object (image) in raw format.
// If the provided link is not referencing an embedable object, the function
// returns absLink without changes.
func (t *TestResourceHandler) GetRawFormatLink(absLink string) (string, error) {
	return "", nil
}

// SetVersion sets version to absLink according to the API scheme. For GitHub
// for example this would replace e.g. the 'master' segment in the path with version
func (t *TestResourceHandler) SetVersion(absLink, version string) (string, error) {
	return "", nil
}

// ResolveDocumentation for a given uri
func (t *TestResourceHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	if t.resolveDocumentation != nil {
		return t.resolveDocumentation(ctx, uri)
	}
	return nil, nil
}

//WithResolveDocumentation overriding the default TestResourceHandler function
func (t *TestResourceHandler) WithResolveDocumentation(resolveDocumentation func(ctx context.Context, uri string) (*api.Documentation, error)) *TestResourceHandler {
	t.resolveDocumentation = resolveDocumentation
	return t
}
