package adocs

import (
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"gopkg.in/yaml.v2"
)

// ResolveLink type defines function for modifying link destination
// dest - original destination
// isEmbeddable - if true, raw destination required
type ResolveLink func(dest string, isEmbeddable bool) (string, error)

type plainRenderer struct {
	ResolveLink
	properties map[string]interface{}
}

// New create new adoc renderer
func New(lr ResolveLink, properties map[string]interface{}) renderer.Renderer {
	return plainRenderer{lr, properties}
}

// Render render adoc content
func (p plainRenderer) Render(w io.Writer, source []byte, _ ast.Node) error {
	processed, err := ProcessAdocContent(source, p.properties, p.ResolveLink)
	if err != nil {
		return err
	}
	_, err = w.Write(processed)
	return err
}

// ProcessAdocContent processes adoc content
func ProcessAdocContent(content []byte, properties map[string]interface{}, rl ResolveLink) ([]byte, error) {
	var err error
	frontmatter := []byte{}
	if properties["frontmatter"] != nil {
		if frontmatter, err = frontMatter(properties["frontmatter"]); err != nil {
			return []byte{}, err
		}
	}
	content = append(frontmatter, content...)
	if properties["adocPath"] != nil {
		content = adocPathIncludes(content, fmt.Sprintf("%v", properties["adocPath"]))
	}
	content = replaceAndDownloadImages(content, rl)
	return content, nil

}
func adocPathIncludes(content []byte, adocPath string) []byte {
	re := regexp.MustCompile(`include::(.+)\.adoc\[(.*)\]`)
	return re.ReplaceAllFunc(content, func(include []byte) []byte {
		// no need to check array because include is guaranteed to have 2 capture groups from the regex
		parts := re.FindSubmatch(include)
		path := string(parts[1])
		tail := string(parts[2])
		res := "include::" + adocPath + "/" + path + ".adoc[" + tail + "]"
		return []byte(res)
	})
}

func replaceAndDownloadImages(content []byte, rl ResolveLink) []byte {
	re := regexp.MustCompile(`image::(.+)\.(png|jpeg|jpg)\[(.*)\]`)
	return re.ReplaceAllFunc(content, func(image []byte) []byte {
		// no need to check array because include is guaranteed to have 3 capture groups from the regex
		parts := re.FindSubmatch(image)
		path := string(parts[1])
		ext := string(parts[2])
		tail := string(parts[3])
		newLink, err := rl(path+"."+ext, true)
		if err != nil {
			return []byte{}
		}
		res := "image::" + newLink + "[" + tail + "]"
		return []byte(res)
	})
}

func frontMatter(frontmatter interface{}) ([]byte, error) {
	buf := bytes.Buffer{}
	_, _ = buf.Write([]byte("---\n"))
	fm, err := yaml.Marshal(frontmatter)
	if err != nil {
		return []byte{}, err
	}
	_, _ = buf.Write(fm)
	_, _ = buf.Write([]byte("---\n"))
	return buf.Bytes(), nil

}

func (p plainRenderer) AddOptions(...renderer.Option) {}
