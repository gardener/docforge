package processors

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/api"

	mdutil "github.com/gardener/docforge/pkg/markdown"
)

var (
	hrefAttrMatchRegex = regexp.MustCompile(`href=["\']?([^"\'>]+)["\']?`)
)

// HugoProcessor is a processor implementation responsible to rewrite links
// on document that use source format (<path>/<name>.md) to destination format
// (<path>/<name> for sites configured for pretty URLs and <path>/<name>.html
// for sites configured for ugly URLs)
type HugoProcessor struct {
	PrettyUrls bool
}

// Process implements Processor#Process
func (f *HugoProcessor) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	var (
		d   []byte
		err error
	)
	if d, err = mdutil.TransformLinks(documentBlob, func(destination []byte) ([]byte, error) {
		return rewriteDestination(destination, node.Name)
	}); err != nil {
		return nil, err
	}
	// TODO: process also HTML links

	return d, nil
}

func rewriteDestination(destination []byte, nodeName string) ([]byte, error) {
	if len(destination) == 0 {
		return destination, nil
	}
	link := string(destination)
	link = strings.TrimSpace(link)
	// trim leading and trailing quotes
	link = strings.TrimRight(strings.TrimLeft(link, "\""), "\"")
	u, err := url.Parse(link)
	if err != nil {
		fmt.Printf("Invalid link: %s", link)
		return destination, nil
	}
	if !u.IsAbs() && !strings.HasPrefix(link, "#") {
		_l := link
		link = strings.TrimRight(link, ".md")
		link = strings.TrimPrefix(link, "./")
		link = fmt.Sprintf("../%s", link)
		if _l != link {
			fmt.Printf("[%s] Rewriting node link for Hugo: %s -> %s \n", nodeName, _l, link)
		}
		return []byte(link), nil
	}
	return destination, nil
}
