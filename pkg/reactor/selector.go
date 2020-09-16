package reactor

// SelectContent TODO: Not implemented
func SelectContent(contentBytes []byte, selectorExpression string) ([]byte, error) {
	// TODO: select content sections from contentBytes if source has a content selector and then filter the rest of it.
	// TODO: define selector expression language. Do CSS/SaaS selectors or alike apply/ can be adapted?
	// Example: "h1-first-of-type" -> the first level one heading (#) in the document
	return contentBytes, nil
}
