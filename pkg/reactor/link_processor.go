package reactor

import (
	"fmt"
	"regexp"
	"strings"
)

// Link ...
type Link struct {
	Source string
	Target string
}

func SelectContent(blob []byte, selectorExpression string) error {
	// TODO: select content sections from blob if source has a content selector and then filter the rest of it.
	// TODO: define selector expression language. Do CSS/SaaS selectors apply?
	// Example: "h1-first-of-type" -> the first level one heading (#) in the document
	return nil
}

// ProcessContent ...
func ProcessContent(blob []byte, source string) ([]*Link, error) {
	// TODO: harvest links from this blob and resolve them to downloadable addresses and serialization targets
	return nil, nil
}

func Process(filePath string) error {
	// var (
	// 	err          error
	// 	contentBytes []byte
	// )
	// if contentBytes, err = ioutil.ReadFile(filePath); err != nil {
	// 	return err
	// }
	// content := string(contentBytes)
	return nil
}

var (
	mdMatchRegex                    = regexp.MustCompile(`\[([^\]]+)]\((.+?)\)`)
	matchRegexDownloadabeExtensions = regexp.MustCompile(`.(jpg|png|gif)$`)
)

func A(link string) string {
	// if (isRelativeUrl(link) && link.indexOf("#")!==0) {
	// there is a token for images required. In this case we must download them and store the images in the
	// local directory for Hugo. Reference (link) to the image didn't work. "Missing token for auth" HTTP 404
	// download the image and store them relative to this document
	// if(githubToken) {
	//      downloadImage(documentRootUrl + "/" + link, markdownRootDir, githubToken)
	// }
	// else {
	//     link = documentRootUrl + "/" + link;
	// }
	// }
	link = matchRegexDownloadabeExtensions.ReplaceAllString(link, ".$1?raw=true")
	link = strings.Replace(link, "/./", "/", -1)
	return fmt.Sprintf("[%s](%s)", "", link)
}

func rewriteMarkdownHyperlinks(content string) {
	mdMatchRegex.ReplaceAllStringFunc(content, A)
}
