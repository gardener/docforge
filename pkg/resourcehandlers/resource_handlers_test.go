// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcehandlers

import (
	"context"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

type TestResourceHandler struct {
	accept bool
}

func (rh *TestResourceHandler) Accept(uri string) bool {
	return rh.accept
}

func (rh *TestResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	return nil, nil
}

func (rh *TestResourceHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	return nil, nil
}

func (rh *TestResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return nil, nil
}
func (rh *TestResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return nil, nil
}
func (rh *TestResourceHandler) Name(uri string) string {
	return ""
}
func (rh *TestResourceHandler) ResourceName(uri string) (string, string) {
	return "", ""
}
func (rh *TestResourceHandler) BuildAbsLink(source, relLink string) (string, error) {
	return relLink, nil
}
func (rh *TestResourceHandler) GetLocalityDomainCandidate(source string) (string, string, string, error) {
	return source, source, "master", nil
}
func (rh *TestResourceHandler) SetVersion(link, version string) (string, error) {
	return link, nil
}

func (rh *TestResourceHandler) GetRawFormatLink(absLink string) (string, error) {
	return absLink, nil
}

func TestGet(t *testing.T) {
	nonAcceptingHandler := &TestResourceHandler{
		accept: false,
	}
	acceptingHandler := &TestResourceHandler{
		accept: true,
	}

	testCases := []struct {
		description string
		handlers    []ResourceHandler
		want        ResourceHandler
	}{
		{
			"should return handler",
			[]ResourceHandler{
				nonAcceptingHandler,
				acceptingHandler,
			},
			acceptingHandler,
		},
		{
			"should not return handler",
			[]ResourceHandler{
				nonAcceptingHandler,
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			r := NewRegistry(tc.handlers...)
			got := r.Get("")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected ResourceHandler %q != %q", got, tc.want)
			}
			r.Remove(tc.handlers...)
		})
	}
}
