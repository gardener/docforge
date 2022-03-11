// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gardener/docforge/pkg/resourcehandlers/git/gitinterface"
	"github.com/google/go-github/v43/github"
)

func TestTransform(t *testing.T) {
	testCases := []struct {
		testFileNameIn  string
		testFileNameOut string
		want            *gitinterface.Info
	}{
		{
			"test_format_00_in.json",
			"test_format_00_out.json",
			&gitinterface.Info{},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			var (
				blobIn, blobOut, b []byte
				err                error
			)
			if blobIn, err = ioutil.ReadFile(filepath.Join("testdata", tc.testFileNameIn)); err != nil {
				t.Fatalf(err.Error())
			}
			commits := []*github.RepositoryCommit{}
			if err = json.Unmarshal(blobIn, &commits); err != nil {
				t.Fatalf(err.Error())
			}
			got := Transform(commits)

			if blobOut, err = ioutil.ReadFile(filepath.Join("testdata", tc.testFileNameOut)); err != nil {
				t.Fatalf(err.Error())
			}

			if b, err = json.MarshalIndent(got, "", "  "); err != nil {
				t.Fatalf(err.Error())
			}
			assert.JSONEq(t, string(blobOut), string(b))
		})
	}

}
