// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"context"
	// "fmt"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

var (
	apiRefNode = &api.Node{
		Name:             "apiRef",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture/apireference.md"}},
	}

	archNode = &api.Node{
		Name:             "arch",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/architecture"}},
		Nodes: []*api.Node{
			apiRefNode,
		},
	}

	blogNode = &api.Node{
		Name:             "blog",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/blog/blog-part1.md"}},
	}

	tasksNode = &api.Node{
		Name:             "tasks",
		ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs/tasks"}},
	}
)

func createNewDocumentation() *api.Documentation {
	return &api.Documentation{
		Structure: []*api.Node{
			{
				Name:             "rootNode",
				ContentSelectors: []api.ContentSelector{{Source: "https://github.com/org/repo/tree/master/docs"}},
				Nodes: []*api.Node{
					archNode,
					blogNode,
					tasksNode,
				},
			},
		},
	}
}

type FakeResourceHandler struct{}

func (f *FakeResourceHandler) Accept(uri string) bool {
	return true
}

func (f *FakeResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	return nil, nil
}

func (f *FakeResourceHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	return nil, nil
}

func (f *FakeResourceHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	return []byte(uri), nil
}
func (f *FakeResourceHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return []byte(""), nil
}

func (f *FakeResourceHandler) Name(uri string) string {
	return uri
}

func (f *FakeResourceHandler) ResourceName(uri string) (string, string) {
	return "", ""
}

func (f *FakeResourceHandler) BuildAbsLink(source, relLink string) (string, error) {
	return relLink, nil
}

func (f *FakeResourceHandler) GetLocalityDomainCandidate(source string) (string, string, string, error) {
	return source, source, "", nil
}

func (f *FakeResourceHandler) SetVersion(link, version string) (string, error) {
	return link, nil
}

func (f *FakeResourceHandler) GetRawFormatLink(link string) (string, error) {
	return link, nil
}
