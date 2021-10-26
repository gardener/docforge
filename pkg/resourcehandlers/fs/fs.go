// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/markdown"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/github"

	ghclient "github.com/google/go-github/v32/github"
)

type fsHandler struct {
	client *ghclient.Client
}

// NewFSResourceHandler create file system ResourceHandler
func NewFSResourceHandler() resourcehandlers.ResourceHandler {
	return &fsHandler{}
}

// Accept implements resourcehandlers.ResourceHandler#Accept
func (fs *fsHandler) Accept(uri string) bool {
	_, err := os.Stat(uri)
	return err == nil
}

// ResolveNodeSelector implements resourcehandlers.ResourceHandler#ResolveNodeSelector
func (fs *fsHandler) ResolveNodeSelector(ctx context.Context, node *api.Node, excludePaths []string, frontMatter map[string]interface{}, excludeFrontMatter map[string]interface{}, depth int32) ([]*api.Node, error) {
	var (
		fileInfo os.FileInfo
		err      error
	)
	if node.NodeSelector == nil {
		return nil, nil
	}
	if fileInfo, err = os.Stat(node.NodeSelector.Path); err != nil {
		return nil, err
	}
	if !fileInfo.IsDir() && filepath.Ext(fileInfo.Name()) == ".md" {
		return nil, fmt.Errorf("nodeSelector path is neither directory or module")
	}
	_node := &api.Node{
		Nodes: []*api.Node{},
	}
	filepath.Walk(node.NodeSelector.Path, func(node *api.Node, parentPath string) filepath.WalkFunc {
		return func(path string, info os.FileInfo, err error) error {
			if node.NodeSelector != nil {
				return nil
			}
			if path != parentPath {
				if len(strings.Split(path, string(os.PathSeparator)))-len(strings.Split(parentPath, string(os.PathSeparator))) != 1 {
					node = node.Parent()
					pathSegments := strings.Split(path, string(os.PathSeparator))
					if len(pathSegments) > 0 {
						pathSegments = pathSegments[:len(pathSegments)-1]
						parentPath = filepath.Join(pathSegments...)
					}
				}
			}
			if !info.IsDir() {
				// check for frontMatter filter compliance
				if frontMatter != nil || excludeFrontMatter != nil {
					// TODO: cache and reuse to avoid redundant reads when the structure nodes are processed
					b, err := fs.Read(ctx, path)
					if err != nil {
						return err
					}
					selected, err := markdown.MatchFrontMatterRules(b, frontMatter, excludeFrontMatter)
					if err != nil {
						return err
					}
					if !selected {
						return nil
					}
				}
			}
			n := &api.Node{
				Name: info.Name(),
			}
			n.SetParent(node)
			node.Nodes = append(node.Nodes, n)
			if info.IsDir() {
				node = n
				node.Nodes = []*api.Node{}
				parentPath = path
			} else {
				n.Source = path
			}
			return nil
		}
	}(_node, node.NodeSelector.Path))
	if len(_node.Nodes) > 0 && len(_node.Nodes[0].Nodes) > 0 {
		for _, n := range _node.Nodes[0].Nodes {
			n.SetParent(nil)
		}
		return _node.Nodes[0].Nodes, nil
	}
	return nil, nil
}

// Read implements resourcehandlers.ResourceHandler#Read
func (fs *fsHandler) Read(ctx context.Context, uri string) ([]byte, error) {
	fileInfo, err := os.Stat(uri)
	if err != nil {
		return nil, err
	}
	if fileInfo.IsDir() {
		return nil, nil
	}
	return ioutil.ReadFile(uri)
}

func (fs *fsHandler) ResolveDocumentation(ctx context.Context, uri string) (*api.Documentation, error) {
	blob, err := fs.Read(ctx, uri)
	if err != nil {
		return nil, err
	}

	return api.ParseWithMetadata([]string{}, blob, true, uri, "master")
}

// ReadGitInfo implements resourcehandlers.ResourceHandler#ReadGitInfo
func (d *fsHandler) ReadGitInfo(ctx context.Context, uri string) ([]byte, error) {
	return github.ReadGitInfo(ctx, uri, d.client)
}

// ResourceName implements resourcehandlers.ResourceHandler#ResourceName
func (fs *fsHandler) ResourceName(link string) (name string, extension string) {
	_, name = filepath.Split(link)
	if len(name) > 0 {
		if e := filepath.Ext(name); len(e) > 0 {
			extension = e[1:]
			name = strings.TrimSuffix(name, e)
		}
	}
	return
}

// BuildAbsLink implements resourcehandlers.ResourceHandler#BuildAbsLink
func (fs *fsHandler) BuildAbsLink(source, link string) (string, error) {
	if filepath.IsAbs(link) {
		return link, nil
	}
	dir, _ := filepath.Split(source)
	p := filepath.Join(dir, link)
	p = filepath.Clean(p)
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

// GetRawFormatLink implements resourcehandlers.ResourceHandler#GetRawFormatLink
func (fs *fsHandler) GetRawFormatLink(absLink string) (string, error) {
	return absLink, nil
}

// SetVersion implements resourcehandlers.ResourceHandler#SetVersion
func (fs *fsHandler) SetVersion(absLink, version string) (string, error) {
	return absLink, nil
}

func (fs *fsHandler) GetClient() *http.Client {
	return nil
}
