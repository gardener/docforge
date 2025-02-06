package link

import "net/url"

// Build builds a link given its elements
func Build(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", nil
	}
	return url.JoinPath(elem[0], elem[1:]...)
}

// MustBuild builds a link given its elements and panics if it fails
func MustBuild(elem ...string) string {
	res, err := Build(elem...)
	if err != nil {
		panic(err)
	}
	return res
}
