package link

import (
	"fmt"
	"net/url"
)

// Build builds a link given its elements
func Build(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", nil
	}
	jointPath, err := url.JoinPath(elem[0], elem[1:]...)
	if err != nil {
		return "", fmt.Errorf("failed to join paths: %w", err)
	}
	return url.QueryUnescape(jointPath)
}
