package urls

import (
	"fmt"
	"net/url"
	"strings"
)

// URL extends the standard url.URL struct
// with additional fields
type URL struct {
	*url.URL
	// ResourcePath is the URL Path without the resource
	// name segment
	ResourcePath string
	// The resource name part of the path.
	// If the url is for a resource with extension,
	// such as .html, it is not part of this string.
	// See Extension for the extension segment.
	ResourceName string
	// The resource extension part of the path
	// identified as the last segment after resource
	// name, starting with a dot (.).
	// May be nil (empty string) if the url resource
	// does not specify extension.
	Extension string
}

const (
	// PathSeparator is the URL paths separator character
	PathSeparator = '/'
)

// Parse extends the standard url.Parse to produce urls.URL
// structure objects
func Parse(urlString string) (*URL, error) {
	var (
		u                               *url.URL
		ext, resourceName, resourcePath string
		err                             error
	)
	if u, err = url.Parse(urlString); err != nil {
		return nil, err
	}
	pSegments := strings.Split(u.Path, string(PathSeparator))
	resourceName = pSegments[len(pSegments)-1]
	resourcePath = strings.Join(pSegments[:len(pSegments)-1], string(PathSeparator))
	if ext = Ext(resourceName); len(ext) > 0 {
		resourceName = strings.TrimSuffix(resourceName, fmt.Sprintf(".%s", ext))
	}
	_u := &URL{
		u,
		resourcePath,
		resourceName,
		ext,
	}
	return _u, nil
}

// Ext returns the resource name extension used by URL path.
// The extension is the suffix beginning at the final dot
// in the final element of path; it is empty if there is
// no dot.
func Ext(path string) string {
	for i := len(path) - 1; i >= 0 && path[i] != PathSeparator; i-- {
		if path[i] == '.' {
			if i <= (len(path) - 1) {
				return path[i+1:]
			}
			return path[i:]
		}
	}
	return ""
}
