package processors

import (
	"reflect"
	"testing"
)

func TestHugoProcess(t *testing.T) {
	var (
		in, got, expected []byte
		err error
	)
	in = []byte("[GitHub](\"./a/b.md\") ![img](\"./images/img.png\") <a href=\"./a/b.md\">link</a>")
	expected = []byte("[GitHub](\"./a/b\") ![img](\"./images/img.png\") <a href=\"./a/b.md\">link</a>\n")
	p:= &HugoProcessor{
		PrettyUrls: true,
	}
	if got, err = p.Process(in, nil); err!=nil {
		t.Errorf("%v!=nil", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("`%v`!=`%v`", string(expected), string(got))
	}
}