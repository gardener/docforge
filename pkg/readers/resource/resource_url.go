package resource

import (
	"fmt"
	"net/url"
	"regexp"
)

var (
	rawPrefixed       = regexp.MustCompile(`https://([^/]+)/raw/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
	resource          = regexp.MustCompile(`https://([^/]+)/([^/]+)/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
	githubusercontent = regexp.MustCompile(`https://raw.githubusercontent.com/([^/]+)/([^/]+)/([^/]+)/([^\?#]+).*`)
)

// IsResourceURL checks if link is resource URL
func IsResourceURL(link string) bool {
	return rawPrefixed.MatchString(link) || resource.MatchString(link) || githubusercontent.MatchString(link)
}

// URL represents a GitHub resource URL
type URL struct {
	Host         string
	Owner        string
	Repo         string
	Type         string
	Ref          string
	ResourcePath string
}

// New creates new resource from url as string
func New(resourceURL string) (URL, error) {
	u, err := url.Parse(resourceURL)
	if err != nil {
		return URL{}, err
	}
	return FromURL(u)
}

// FromURL creates new resource from url object
func FromURL(u *url.URL) (URL, error) {
	if u.String() == "" {
		return URL{}, nil
	}
	components := rawPrefixed.FindStringSubmatch(u.String())
	if components != nil {
		return URL{
			Host:         components[1],
			Owner:        components[2],
			Repo:         components[3],
			Type:         "raw",
			Ref:          components[4],
			ResourcePath: components[5],
		}, nil
	}
	components = githubusercontent.FindStringSubmatch(u.String())
	if components != nil {
		return URL{
			Host:         "github.com",
			Owner:        components[1],
			Repo:         components[2],
			Type:         "raw",
			Ref:          components[3],
			ResourcePath: components[4],
		}, nil
	}
	components = resource.FindStringSubmatch(u.String())
	if components != nil {
		return URL{
			Host:         components[1],
			Owner:        components[2],
			Repo:         components[3],
			Type:         components[4],
			Ref:          components[5],
			ResourcePath: components[6],
		}, nil
	}
	return URL{}, fmt.Errorf("%s is not a resource URL", u.String())
}

// String returns the u
func (r *URL) String() string {
	return fmt.Sprintf("https://%s/%s/%s/%s/%s/%s", r.Host, r.Owner, r.Repo, r.Type, r.Ref, r.ResourcePath)
}

// RepoURL returns the GitHub repository URL
func (r *URL) RepoURL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Owner, r.Repo)
}

// RawURL returns the GitHub raw URL if the resource is 'blob', otherwise returns the origin URL
func (r *URL) RawURL() string {
	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s", r.Host, r.Owner, r.Repo, r.Ref, r.ResourcePath)
}
