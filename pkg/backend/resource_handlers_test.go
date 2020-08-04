package backend

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

type TestResourceHandler struct {
	accept bool
}

func (rh *TestResourceHandler) Accept(uri string) bool {
	return rh.accept
}
func (rh *TestResourceHandler) ResolveNodeSelector(ctx context.Context, node *api.Node) error {
	return nil
}
func (rh *TestResourceHandler) Read(ctx context.Context, node *api.Node) ([]byte, error) {
	return nil, nil
}
func (rh *TestResourceHandler) Path(uri string) string {
	return string("")
}
func (rh *TestResourceHandler) DownloadUrl(uri string) string {
	return string("")
}

func TestGet(t *testing.T) {
	nonAcceptingHandler := &TestResourceHandler{
		accept: false,
	}
	acceptingHandler := &TestResourceHandler{
		accept: true,
	}

	cases := []struct {
		description string
		handlers    *ResourceHandlers
		want        ResourceHandler
	}{
		{
			"should return handler",
			&ResourceHandlers{
				nonAcceptingHandler,
				acceptingHandler,
			},
			acceptingHandler,
		},
		{
			"should not return handler",
			&ResourceHandlers{
				nonAcceptingHandler,
			},
			nil,
		},
	}
	for _, c := range cases {
		fmt.Println(c.description)
		got := c.handlers.Get("")
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Get(\"\") == %q, want %q", got, c.want)
		}
	}
}
