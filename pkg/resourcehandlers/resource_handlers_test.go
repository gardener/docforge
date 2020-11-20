// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcehandlers

import (
	"reflect"
	"testing"

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

func TestGet(t *testing.T) {
	nonAcceptingHandler := &TestResourceHandler{
		accept: false,
	}
	acceptingHandler := &TestResourceHandler{
		accept: true,
	}

	testCases := []struct {
		description string
		handlers    []URIValidator
		want        URIValidator
	}{
		{
			"should return handler",
			[]URIValidator{
				nonAcceptingHandler,
				acceptingHandler,
			},
			acceptingHandler,
		},
		{
			"should not return handler",
			[]URIValidator{
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
