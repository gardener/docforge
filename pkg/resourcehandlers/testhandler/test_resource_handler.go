// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package testhandler

import (
	"context"
	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/httpclient"
	"net/http"
)

//TestResourceHandler ...
type TestResourceHandler struct {
	accept               func(uri string) bool
	resolveNodeSelector  func(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)
	resolveDocumentation func(ctx context.Context, uri string) (*api.Documentation, error)
	resourceName         func(link string) (string, string)
	buildAbsLink         func(source, link string) (string, error)
	getRawFormatLink     func(absLink string) (string, error)
	setVersion           func(absLink, version string) (string, error)
	read                 func(ctx context.Context, uri string) ([]byte, error)
	readGitInfo          func(ctx context.Context, uri string) ([]byte, error)
	getClient            func() *http.Client
}

//NewTestResourceHandler ...
func NewTestResourceHandler() *TestResourceHandler {
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

//ResolveNodeSelector ...
func (t *TestResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	if t.resolveNodeSelector != nil {
		return t.resolveNodeSelector(ctx, node, excludePaths, frontMatter, excludeFrontMatter, depth)
	}
	return []*api.Node{}, nil
}

//WithResolveNodeSelector ...
func (t *TestResourceHandler) WithResolveNodeSelector(resolveNodeSelector func(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error)) *TestResourceHandler {
	t.resolveNodeSelector = resolveNodeSelector
	return t
}

// Read a resource content at uri into a byte array
func (t *TestResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	if t.read != nil {
		return t.read(ctx, uri)
	}
	return []byte{}, nil
}

//WithRead ...
func (t *TestResourceHandler) WithRead(read func(ctx context.Context, uri string) ([]byte, error)) *TestResourceHandler {
	t.read = read
	return t
}

// ReadGitInfo ..
func (t *TestResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	if t.readGitInfo != nil {
		return t.readGitInfo(ctx, uri)
	}
	return []byte{}, nil
}

//WithReadGitInfo ...
func (t *TestResourceHandler) WithReadGitInfo(readGitInfo func(ctx context.Context, uri string) ([]byte, error)) *TestResourceHandler {
	t.readGitInfo = readGitInfo
	return t
}

// ResourceName returns a breakdown of a resource name in the link, consisting
// of name and potentially and extention without the dot.
func (t *TestResourceHandler) ResourceName(link string) (string, string) {
	if t.resourceName != nil {
		return t.resourceName(link)
	}
	return "", ""
}

// WithResourceName ...
func (t *TestResourceHandler) WithResourceName(resourceName func(link string) (string, string)) *TestResourceHandler {
	t.resourceName = resourceName
	return t
}

// BuildAbsLink should return an absolute path of a relative link in regards of the provided
// source
func (t *TestResourceHandler) BuildAbsLink(source, link string) (string, error) {
	if t.buildAbsLink != nil {
		return t.buildAbsLink(source, link)
	}
	return "", nil
}

//WithBuildAbsLink ...
func (t *TestResourceHandler) WithBuildAbsLink(buildAbsLink func(source, link string) (string, error)) *TestResourceHandler {
	t.buildAbsLink = buildAbsLink
	return t
}

// GetRawFormatLink returns a link to an embedable object (image) in raw format.
// If the provided link is not referencing an embedable object, the function
// returns absLink without changes.
func (t *TestResourceHandler) GetRawFormatLink(absLink string) (string, error) {
	if t.getRawFormatLink != nil {
		return t.getRawFormatLink(absLink)
	}
	return "", nil
}

//WithGetRawFormatLink ...
func (t *TestResourceHandler) WithGetRawFormatLink(getRawFormatLink func(absLink string) (string, error)) *TestResourceHandler {
	t.getRawFormatLink = getRawFormatLink
	return t
}

// SetVersion sets version to absLink according to the API scheme. For GitHub
// for example this would replace e.g. the 'master' segment in the path with version
func (t *TestResourceHandler) SetVersion(absLink, version string) (string, error) {
	if t.setVersion != nil {
		return t.setVersion(absLink, version)
	}
	return "", nil
}

//WithSetVersion ...
func (t *TestResourceHandler) WithSetVersion(setVersion func(absLink, version string) (string, error)) *TestResourceHandler {
	t.setVersion = setVersion
	return t
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

// GetClient for this handler
func (t *TestResourceHandler) GetClient() httpclient.Client {
	return nil
}

// WithGetClient ...
func (t *TestResourceHandler) WithGetClient(getClient func() *http.Client) *TestResourceHandler {
	t.getClient = getClient
	return t
}
