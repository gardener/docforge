package resource

import (
	"fmt"
	"net/url"
)

func IsResourceURL(link string) bool {
	return rawPrefixed.MatchString(link) || resource.MatchString(link) || githubusercontent.MatchString(link)
}

// Resource represents a GitHub resource URL
type ResourceURL struct {
	Host         string
	Owner        string
	Repo         string
	Type         string
	Ref          string
	ResourcePath string
}

// NewResource creates new resource from url as string
func NewResourceURL(URL string) (ResourceURL, error) {
	u, err := url.Parse(URL)
	if err != nil {
		return ResourceURL{}, err
	}
	return NewResourceURLFromURL(u)
}

// NewResourceFromURL creates new resource from url object
func NewResourceURLFromURL(u *url.URL) (ResourceURL, error) {
	if u.String() == "" {
		return ResourceURL{}, nil
	}
	components := rawPrefixed.FindStringSubmatch(u.String())
	if components != nil {
		return ResourceURL{
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
		return ResourceURL{
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
		return ResourceURL{
			Host:         components[1],
			Owner:        components[2],
			Repo:         components[3],
			Type:         components[4],
			Ref:          components[5],
			ResourcePath: components[6],
		}, nil
	}
	return ResourceURL{}, fmt.Errorf("unknown link type for resource %s", u.String())
}

// ToResourceURL returns the u
func (r *ResourceURL) ToResourceURL() string {
	return fmt.Sprintf("https://%s/%s/%s/%s/%s/%s", r.Host, r.Owner, r.Repo, r.Type, r.Ref, r.ResourcePath)
}

// TotRepoURL returns the GitHub repository URL
func (r *ResourceURL) TotRepoURL() string {
	return fmt.Sprintf("https://%s/%s/%s", r.Host, r.Owner, r.Repo)
}

// ToRawURL returns the GitHub raw URL if the resource is 'blob', otherwise returns the origin URL
func (r *ResourceURL) ToRawURL() string {
	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s", r.Host, r.Owner, r.Repo, r.Ref, r.ResourcePath)
}
