// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcehandlers

import (
	"github.com/gardener/docforge/pkg/resourcehandlers/testhandler"
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

func TestGet(t *testing.T) {
	nonAcceptingHandler := testhandler.NewTestResourceHandler().WithAccept(func(uri string) bool {
		return false
	})
	acceptingHandler := testhandler.NewTestResourceHandler().WithAccept(func(uri string) bool {
		return true
	})

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
