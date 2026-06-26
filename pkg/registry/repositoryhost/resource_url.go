package repositoryhost

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/internal/must"
)

var (
	// knownHosts holds all GitHub-compatible hosts that docforge can process.
	// Populated at startup via RegisterHost() for each entry in github-oauth-env-map.
	// TODO: extend this mechanism to support GitLab hosts when a GitLab implementation is added.
	knownHosts = []string{"github.com"}

	cachedRaw      *regexp.Regexp
	cachedResource *regexp.Regexp

	githubusercontent = regexp.MustCompile(`https://raw.githubusercontent.com/([^/]+)/([^/]+)/([^/]+)/([^\?#]*)(.*)`)
)

func init() {
	cachedRaw, cachedResource = buildRegexps()
}

func buildRegexps() (*regexp.Regexp, *regexp.Regexp) {
	escaped := make([]string, len(knownHosts))
	for i, h := range knownHosts {
		escaped[i] = regexp.QuoteMeta(h)
	}
	hostGroup := strings.Join(escaped, "|")
	raw := regexp.MustCompile(`https://(` + hostGroup + `)/raw/([^/]+)/([^/]+)/([^/]+)/([^\?#]*)(.*)`)
	res := regexp.MustCompile(`https://(` + hostGroup + `)/([^/]+)/([^/]+)/([^/]+)/([^/]+)/?([^\?#]*)(.*)`)
	return raw, res
}

// RegisterHost adds a GitHub-compatible host to the list of recognized hosts.
// Must be called before any URL parsing occurs (i.e. at startup, before goroutines are started).
func RegisterHost(host string) {
	for _, h := range knownHosts {
		if h == host {
			return
		}
	}
	knownHosts = append(knownHosts, host)
	cachedRaw, cachedResource = buildRegexps()
}

// IsResourceURL checks if link is resource URL
func IsResourceURL(link string) bool {
	return cachedRaw.MatchString(link) || cachedResource.MatchString(link) || githubusercontent.MatchString(link)
}

// IsRelative is a helper function that checks if a link is relative
func IsRelative(link string) bool {
	url, err := url.Parse(link)
	if err != nil {
		return false
	}
	return !url.IsAbs()
}

// RawURL returns the GitHub raw URL for the resource, unless the resource type is 'actions' in which case it returns the origin URL
func RawURL(resourceURL string) (string, error) {
	r, err := new(resourceURL)
	if err != nil {
		return "", err
	}
	if r.resourceType == "actions" {
		return resourceURL, nil
	}
	return link.Build("https://", r.host, r.owner, r.repo, "raw", r.ref, r.resourcePath)
}

// URL represents an repsource url
type URL struct {
	host           string
	owner          string
	repo           string
	resourceType   string
	ref            string
	resourcePath   string
	resourceSuffix string
}

// new creates new resource from url as string
func new(resourceURL string) (*URL, error) {
	u, err := url.Parse(resourceURL)
	if err != nil {
		return nil, err
	}
	if u.String() == "" {
		return nil, nil
	}
	components := cachedRaw.FindStringSubmatch(u.String())
	if components != nil {
		return &URL{
			host:           components[1],
			owner:          components[2],
			repo:           components[3],
			resourceType:   "raw",
			ref:            components[4],
			resourcePath:   components[5],
			resourceSuffix: components[6],
		}, nil
	}
	components = githubusercontent.FindStringSubmatch(u.String())
	if components != nil {
		return &URL{
			host:           "github.com",
			owner:          components[1],
			repo:           components[2],
			resourceType:   "blob",
			ref:            components[3],
			resourcePath:   components[4],
			resourceSuffix: components[5],
		}, nil
	}
	components = cachedResource.FindStringSubmatch(u.String())
	if components != nil {
		return &URL{
			host:           components[1],
			owner:          components[2],
			repo:           components[3],
			resourceType:   components[4],
			ref:            components[5],
			resourcePath:   components[6],
			resourceSuffix: components[7],
		}, nil
	}
	return nil, fmt.Errorf("%s is not a resource URL", u.String())
}

// String returns the full url
func (r URL) String() string {
	if r.resourcePath == "" {
		return must.Succeed(link.Build("https://", r.host, r.owner, r.repo, r.resourceType, r.ref))
	}
	return must.Succeed(link.Build("https://", r.host, r.owner, r.repo, r.resourceType, r.ref, r.resourcePath+r.resourceSuffix))
}

// ResourceURL returns the resource url without resource suffix
func (r URL) ResourceURL() string {
	if r.resourcePath == "" {
		return must.Succeed(link.Build("https://", r.host, r.owner, r.repo, r.resourceType, r.ref))
	}
	return must.Succeed(link.Build("https://", r.host, r.owner, r.repo, r.resourceType, r.ref, r.resourcePath))
}

// ReferenceURL returns the reference url object
func (r URL) ReferenceURL() URL {
	return URL{
		host:         r.host,
		owner:        r.owner,
		repo:         r.repo,
		resourceType: "tree",
		ref:          r.ref,
	}
}

// RepositoryURLString return the repository url as string
func (r URL) RepositoryURLString() string {
	return must.Succeed(link.Build("https://", r.host, r.owner, r.repo))
}

// ResolveRelativeLink returns the possible blob and tree url string of a given relative link
func (r URL) ResolveRelativeLink(relativeLink string) (string, string, error) {
	if !IsRelative(relativeLink) {
		return "", "", fmt.Errorf("expected relative link, got %s", relativeLink)
	}
	// resources can have a trailing /
	if relativeLink != "/" {
		relativeLink = strings.TrimSuffix(relativeLink, "/")
	}
	resourcePathURL, err := url.Parse(r.resourcePath)
	if err != nil {
		return "", "", errors.New("unexpected error in resource.ResolveRelativeLink")
	}
	resolvedPath, err := resourcePathURL.Parse(relativeLink)
	if err != nil {
		return "", "", errors.New("unexpected error in resource.ResolveRelativeLink")
	}
	referenceURL := r.ReferenceURL().String()
	finalLink, err := url.JoinPath(referenceURL, resolvedPath.String())
	if err != nil {
		return "", "", errors.New("unexpected error in resource.ResolveRelativeLink")
	}
	finalLink, err = url.PathUnescape(finalLink)
	if err != nil {
		return "", "", errors.New("unexpected error in resource.ResolveRelativeLink")
	}
	finalTreeResource, err := new(finalLink)
	if err != nil {
		return "", "", errors.New("unexpected error in resource.ResolveRelativeLink")
	}
	finalBlobResource := *finalTreeResource
	finalBlobResource.resourceType = "blob"
	return finalBlobResource.String(), finalTreeResource.String(), nil
}

// GetHost returns the host of the URL
func (r URL) GetHost() string {
	return r.host
}

// GetOwner returns the owner of the URL
func (r URL) GetOwner() string {
	return r.owner
}

// GetRepo returns the repository of the URL
func (r URL) GetRepo() string {
	return r.repo
}

// GetResourceType returns the resource type of the URL
func (r URL) GetResourceType() string {
	return r.resourceType
}

// GetRef returns the reference of the URL
func (r URL) GetRef() string {
	return r.ref
}

// GetResourcePath returns the resource path of the URL
func (r URL) GetResourcePath() string {
	return r.resourcePath
}

// GetResourceSuffix returns the resource suffix of the URL
func (r URL) GetResourceSuffix() string {
	return r.resourceSuffix
}

// GetDifferentType returns the url string of the given resource but with a different type
func (r URL) GetDifferentType(newType string) (string, error) {
	if newType != "blob" && newType != "tree" {
		return "", fmt.Errorf("tried creating resource URL with type %s where only blob and tree types are supported", newType)
	}
	r.resourceType = newType
	return r.String(), nil
}
