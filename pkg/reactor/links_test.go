// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"testing"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/stretchr/testify/assert"
)

func Test_MatchForLinkRewrite(t *testing.T) {
	testCases := []struct {
		link            string
		globalRules     map[string]*api.LinkRewriteRule
		wantVersion     *string
		wantDestination *string
		wantText        *string
		wantTitle       *string
		wantOK          bool
		mutateNode      func(n *api.Node)
	}{
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": &api.LinkRewriteRule{
					Destination: tests.StrPtr("cda"),
				},
			},
			nil,
			tests.StrPtr("cda"),
			nil,
			nil,
			true,
			nil,
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": nil,
			},
			nil,
			tests.StrPtr(""),
			tests.StrPtr(""),
			nil,
			true,
			nil,
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{},
			nil,
			nil,
			nil,
			nil,
			false,
			nil,
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": &api.LinkRewriteRule{
					Version:     tests.StrPtr("v1.10.1"),
					Destination: tests.StrPtr("cda"),
					Text:        tests.StrPtr("Test"),
					Title:       tests.StrPtr("Test Title"),
				},
			},
			tests.StrPtr("v1.10.1"),
			tests.StrPtr("cda"),
			tests.StrPtr("Test"),
			tests.StrPtr("Test Title"),
			true,
			nil,
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": &api.LinkRewriteRule{
					Destination: tests.StrPtr("abc"),
				},
			},
			nil,
			tests.StrPtr("cda"),
			nil,
			nil,
			true,
			func(n *api.Node) {
				n.Links = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"abc": &api.LinkRewriteRule{
							Destination: tests.StrPtr("cda"),
						},
					},
				}
			},
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": &api.LinkRewriteRule{
					Version:     tests.StrPtr("v1.10.1"),
					Destination: tests.StrPtr("abc"),
					Text:        tests.StrPtr("Test"),
					Title:       tests.StrPtr("Test Title"),
				},
			},
			tests.StrPtr("v1.10.1"),
			tests.StrPtr("cda"),
			tests.StrPtr("Test"),
			tests.StrPtr("Test Title"),
			true,
			func(n *api.Node) {
				n.Links = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"abc": &api.LinkRewriteRule{
							Destination: tests.StrPtr("cda"),
						},
					},
				}
			},
		},
		{
			"abc",
			map[string]*api.LinkRewriteRule{
				"abc": &api.LinkRewriteRule{
					Version: tests.StrPtr("v1.10.1"),
				},
			},
			tests.StrPtr("v1.10.1"),
			tests.StrPtr("cda"),
			tests.StrPtr("Test"),
			tests.StrPtr("Test Title"),
			true,
			func(n *api.Node) {
				n.Links = &api.Links{
					Rewrites: map[string]*api.LinkRewriteRule{
						"abc": &api.LinkRewriteRule{
							Text:        tests.StrPtr("Test"),
							Destination: tests.StrPtr("cda"),
						},
					},
				}
				n1 := &api.Node{
					Nodes: []*api.Node{n},
				}
				n.SetParent(n1)
				n2 := &api.Node{
					Links: &api.Links{
						Rewrites: map[string]*api.LinkRewriteRule{
							"abc": &api.LinkRewriteRule{
								Title: tests.StrPtr("Test Title"),
							},
						},
					},
					Nodes: []*api.Node{n1},
				}
				n1.SetParent(n2)
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			n := &api.Node{}
			if tc.mutateNode != nil {
				tc.mutateNode(n)
			}
			gotVersion, gotDestination, gotText, gotTitle, gotOK := MatchForLinkRewrite(tc.link, n, tc.globalRules)
			assert.Equal(t, tc.wantOK, gotOK)
			if gotVersion != nil && tc.wantVersion == nil {
				t.Errorf("expected version to be nil but it was %s", *gotVersion)
			} else if gotVersion == nil && tc.wantVersion != nil {
				t.Errorf("expected version to be %s but it was nil", *tc.wantVersion)
			} else if gotVersion != nil && tc.wantVersion != nil {
				assert.Equal(t, *tc.wantVersion, *gotVersion)
			} else {
				assert.Nil(t, gotVersion)
			}
			if gotDestination != nil && tc.wantDestination == nil {
				t.Errorf("expected destination to be nil but it was %s", *gotDestination)
			} else if gotDestination == nil && tc.wantDestination != nil {
				t.Errorf("expected destination to be %s but it was nil", *tc.wantDestination)
			} else if gotDestination != nil && tc.wantDestination != nil {
				assert.Equal(t, *tc.wantDestination, *gotDestination)
			} else {
				assert.Nil(t, gotDestination)
			}
			if gotText != nil && tc.wantText == nil {
				t.Errorf("expected text to be nil but it was %s", *gotText)
			} else if gotDestination == nil && tc.wantText != nil {
				t.Errorf("expected text to be %s but it was nil", *tc.wantText)
			} else if gotText != nil && tc.wantText != nil {
				assert.Equal(t, *tc.wantText, *gotText)
			} else {
				assert.Nil(t, gotText)
			}
			if gotTitle != nil && tc.wantTitle == nil {
				t.Errorf("expected title to be nil but it was %s", *gotTitle)
			} else if gotDestination == nil && tc.wantTitle != nil {
				t.Errorf("expected title to be %s but it was nil", *tc.wantTitle)
			} else if gotTitle != nil && tc.wantTitle != nil {
				assert.Equal(t, *tc.wantTitle, *gotTitle)
			} else {
				assert.Nil(t, gotTitle)
			}
		})
	}
}

func Test_MatchForDownload(t *testing.T) {
	testCases := []struct {
		link             string
		globalRules      *api.Downloads
		wantDownloadName string
		wantOK           bool
		mutateNode       func(n *api.Node)
	}{
		{
			"abc",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"abc": api.ResourceRenameRules{
						"abc": "cda",
					},
				},
			},
			"cda",
			true,
			nil,
		},
		{
			"abc",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"ABC": api.ResourceRenameRules{
						"abc": "cda",
					},
				},
			},
			"",
			false,
			nil,
		},
		{
			"abc",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"abc": api.ResourceRenameRules{
						"abc": "cda",
					},
				},
			},
			"def",
			true,
			func(n *api.Node) {
				n.Links = &api.Links{
					Downloads: &api.Downloads{
						Scope: map[string]api.ResourceRenameRules{
							"abc": api.ResourceRenameRules{
								"abc": "def",
							},
						},
					},
				}
			},
		},
		{
			"abc/a.md",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"abc": api.ResourceRenameRules{
						"abc": "$name-test",
					},
				},
			},
			"a-test",
			true,
			nil,
		},
		{
			"abc/a.md",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"abc": api.ResourceRenameRules{
						"abc": "$name-test$ext",
					},
				},
			},
			"a-test.md",
			true,
			nil,
		},
		{
			"abc/a.md",
			&api.Downloads{
				Scope: map[string]api.ResourceRenameRules{
					"abc": api.ResourceRenameRules{
						"abc": "$name-0$ext",
					},
				},
				Renames: map[string]string{
					"abc": "$name-1$ext",
				},
			},
			"a-0.md",
			true,
			nil,
		},
		{
			"abc/a.md",
			&api.Downloads{
				Renames: map[string]string{
					"\\.(md)": "$name-1$ext",
				},
			},
			"a-1.md",
			true,
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			var (
				link *urls.URL
				err  error
			)
			n := &api.Node{}
			if tc.mutateNode != nil {
				tc.mutateNode(n)
			}
			if link, err = urls.Parse(tc.link); err != nil {
				t.Fatalf("%v", err)
				return
			}
			gotName, gotOK := MatchForDownload(link, n, tc.globalRules)
			assert.Equal(t, tc.wantOK, gotOK)
			assert.Equal(t, tc.wantDownloadName, gotName)
		})
	}
}
