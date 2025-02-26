package link

import "net/url"

// Build builds a link given its elements
func Build(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", nil
	}
	return url.JoinPath(elem[0], elem[1:]...)
}
