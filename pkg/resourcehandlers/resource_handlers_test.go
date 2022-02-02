// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcehandlers_test

import (
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/resourcehandlers/resourcehandlersfakes"

	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetKlogV(6)
}

func TestGet(t *testing.T) {
	nonAcceptingHandler := &resourcehandlersfakes.FakeResourceHandler{}
	nonAcceptingHandler.AcceptStub = func(uri string) bool {
		return false
	}
	acceptingHandler := &resourcehandlersfakes.FakeResourceHandler{}
	acceptingHandler.AcceptStub = func(uri string) bool {
		return true
	}

	testCases := []struct {
		description string
		handlers    []resourcehandlers.ResourceHandler
		want        resourcehandlers.ResourceHandler
	}{
		{
			"should return handler",
			[]resourcehandlers.ResourceHandler{
				nonAcceptingHandler,
				acceptingHandler,
			},
			acceptingHandler,
		},
		{
			"should not return handler",
			[]resourcehandlers.ResourceHandler{
				nonAcceptingHandler,
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			r := resourcehandlers.NewRegistry(tc.handlers...)
			got := r.Get("")
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("expected ResourceHandler %q != %q", got, tc.want)
			}
			r.Remove(tc.handlers...)
		})
	}
}
