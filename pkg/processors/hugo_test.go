package processors

import (
	"reflect"
	"testing"

	"github.com/gardener/docforge/pkg/api"
)

func TestHugoProcess(t *testing.T) {
	var (
		in, got, expected []byte
		err               error
	)
	in = []byte("[GitHub](./a/b.md) ![img](./images/img.png)")
	expected = []byte("[GitHub](../a/b)\n![img](../images/img.png)\n")
	p := &HugoProcessor{
		PrettyUrls: true,
	}
	if got, err = p.Process(in, &api.Node{Name: "Test"}); err != nil {
		t.Errorf("%v!=nil", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("`%v`!=`%v`", string(expected), string(got))
	}
}
