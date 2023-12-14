// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package repositoryhosts_test

import (
	"reflect"
	"testing"

	repositoryhosts "github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
)

func TestGet(t *testing.T) {
	nonAcceptingHandler := &repositoryhostsfakes.FakeRepositoryHost{}
	nonAcceptingHandler.AcceptStub = func(uri string) bool {
		return false
	}
	acceptingHandler := &repositoryhostsfakes.FakeRepositoryHost{}
	acceptingHandler.AcceptStub = func(uri string) bool {
		return true
	}

	testCases := []struct {
		description string
		handlers    []repositoryhosts.RepositoryHost
		want        repositoryhosts.RepositoryHost
	}{
		{
			"should return handler",
			[]repositoryhosts.RepositoryHost{
				nonAcceptingHandler,
				acceptingHandler,
			},
			acceptingHandler,
		},
		{
			"should not return handler",
			[]repositoryhosts.RepositoryHost{
				nonAcceptingHandler,
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			r := repositoryhosts.NewRegistry(tc.handlers...)
			got := r.Get("")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected RepositoryHost %q != %q", got, tc.want)
			}
			r.Remove(tc.handlers...)
		})
	}
}
