package renderer

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
)

const (
	extensions = parser.CommonExtensions | parser.AutoHeadingIDs
)

func TestRender(t *testing.T) {
	var (
		documentBlob, expectedDocumentBlob []byte
		err                                error
	)
	if documentBlob, err = ioutil.ReadFile(filepath.Join("testdata", "renderer_test_00.md")); err != nil {
		t.Fatalf(err.Error())
	}
	if expectedDocumentBlob, err = ioutil.ReadFile(filepath.Join("testdata", "renderer_test_00-expected.md")); err != nil {
		t.Fatalf(err.Error())
	}
	mdParser := parser.NewWithExtensions(extensions)
	document := markdown.Parse(documentBlob, mdParser)
	r := NewRenderer(RendererOptions{
		TextWidth: -1,
	})
	documentBlob = markdown.Render(document, r)

	assert.Equal(t, string(documentBlob), string(expectedDocumentBlob))
}
