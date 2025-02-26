package link

import (
	"fmt"
	"net/url"
	"strings"
)

// Build builds a link given its elements
func Build(elem ...string) (string, error) {
	if len(elem) == 0 {
		return "", nil
	}
	jointPath, err := url.JoinPath(elem[0], elem[1:]...)
	// TODO Remove this after improvement in manifest.go
	if jointPath == "" {
		return ".", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to join paths: %w", err)
	}
	escapedQuery, err := url.QueryUnescape(jointPath)
	if err != nil {
		return "", fmt.Errorf("failed to unescape joint path: %w", err)
	}
	return strings.ReplaceAll(escapedQuery, " ", "%20"), nil
}
