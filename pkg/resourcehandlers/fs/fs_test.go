// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fs

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers/utils"
	"github.com/stretchr/testify/assert"
)

func TestFSRead(t *testing.T) {
	var (
		content []byte
		err     error
	)
	fs := &fsHandler{}
	if content, err = fs.Read(context.TODO(), filepath.Join("testdata", "f00.md")); err != nil {
		t.Fatalf("%s", err.Error())
	}
	assert.Equal(t, []byte("Test data"), content)
}

func TestGitLog(t *testing.T) {
	var (
		log []*utils.GitLogEntry
		err error
	)
	if log, err = utils.GitLog(filepath.Join("testdata", "f00.md")); err != nil {
		t.Fatalf("%s", err.Error())
	}
	assert.NotNil(t, log)
}

func TestReadGitInfo(t *testing.T) {
	var (
		log []byte
		err error
	)
	fs := &fsHandler{}
	if log, err = fs.ReadGitInfo(context.TODO(), filepath.Join("testdata", "f00.md")); err != nil {
		t.Fatalf("%s", err.Error())
	}
	assert.NotNil(t, log)
}

func TestResolveNodeSelector(t *testing.T) {
	var (
		err error
	)
	fs := &fsHandler{}
	node := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "testdata",
		},
	}
	expected := &api.Node{
		NodeSelector: &api.NodeSelector{
			Path: "testdata",
		},
		Nodes: []*api.Node{
			{
				Name: "d00",
				Nodes: []*api.Node{
					{
						Name: "d02",
						Nodes: []*api.Node{
							{
								Name:   "f020.md",
								Source: "testdata/d00/d02/f020.md",
							},
						},
					},
					{
						Name:   "f01.md",
						Source: "testdata/d00/f01.md",
					},
				},
			},
			{
				Name:   "f00.md",
				Source: "testdata/f00.md",
			},
		},
	}
	expected.SetParentsDownwards()
	expectedNodes := expected.Nodes
	for _, n := range expectedNodes {
		n.SetParent(nil)
	}
	nodes, err := fs.ResolveNodeSelector(context.TODO(), node, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("%s", err.Error())
	}

	assert.Equal(t, expected.Nodes, nodes)
}

func TestBuildAbsLink(t *testing.T) {
	testCases := []struct {
		source   string
		link     string
		wantLink string
		wantErr  error
	}{
		{
			source:   "a/b/c.md",
			link:     "/d/e/f.md",
			wantLink: "/d/e/f.md",
		},
		{
			source:   "a/b/c.md",
			link:     "./d.md",
			wantLink: "a/b/d.md",
		},
		{
			source:   "a/b/c.md",
			link:     "d.md",
			wantLink: "a/b/d.md",
		},
		{
			source:   "a/b/c.md",
			link:     "../d.md",
			wantLink: "a/d.md",
		},
		{
			source:   "a/b/c.md",
			link:     "d/e/f.md",
			wantLink: "a/b/d/e/f.md",
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			fs := fsHandler{}
			var wantLink string
			if !filepath.IsAbs(tc.link) {
				absPath, _ := filepath.Abs(".")
				wantLink = filepath.Join(absPath, tc.wantLink)
			} else {
				wantLink = tc.wantLink
			}
			gotLink, gotErr := fs.BuildAbsLink(tc.source, tc.link)
			if tc.wantErr != nil {
				assert.Error(t, gotErr)
			} else {
				assert.Nil(t, gotErr)
			}
			assert.Equal(t, wantLink, gotLink)
		})
	}
}
