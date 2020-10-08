package reactor

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/util/urls"
	"github.com/google/uuid"
	"k8s.io/klog/v2"
)

// localityDomain contains the entries defining a
// locality domain scope. Each entry is a mapping
// between a domain, such as github.com/gardener/gardener,
// and a path in it that defines "local" resources.
// Documents referenced by documentation node structure
// are always part of the locality domain. Other
// resources referenced by those documents are checked
// against the path hierarchy of locality domain
// entries to determine hwo they will be processed.
type localityDomain map[string]*localityDomainValue

// LocalityDomainValue encapsulates the memebers of a
// localityDomain entry value
type localityDomainValue struct {
	// Version is the version of the resources in this
	// locality odmain
	Version string
	// Path defines the scope of this locality domain
	// and is relative to it
	Path                string
	Include             []string
	Exclude             []string
	LinkSubstitutes     Substitutes
	DownloadSubstitutes Substitutes
}

// Substitutes is ...
type Substitutes map[string]string

func copySubstitutes(s api.Substitutes) Substitutes {
	_s := Substitutes{}
	for k, v := range s {
		_s[k] = v
	}
	return _s
}

// fromAPI creates new localityDomain copy object from
// api.LocalityDomain
func copy(ld api.LocalityDomain) localityDomain {
	localityDomain := localityDomain{}
	for k, v := range ld {
		localityDomain[k] = &localityDomainValue{
			v.Version,
			v.Path,
			v.Include,
			v.Exclude,
			copySubstitutes(v.LinkSubstitutes),
			copySubstitutes(v.DownloadSubstitutes),
		}
	}
	return localityDomain
}

// Set creates or updates a locality domain entry
// with key and path. An update is performed when
// the path is ancestor оф the existing path for
// that key.
func (ld localityDomain) Set(key, path, version string) {
	var (
		existingLD *localityDomainValue
		ok         bool
	)
	if existingLD, ok = ld[key]; !ok {
		ld[key] = &localityDomainValue{
			version,
			path,
			nil,
			nil,
			nil,
			nil,
		}
		return
	}

	localityDomain := strings.Split(existingLD.Path, "/")
	localityDomainCandidate := strings.Split(path, "/")
	for i := range localityDomain {
		if len(localityDomainCandidate) <= i || localityDomain[i] != localityDomainCandidate[i] {
			ld[key].Path = strings.Join(localityDomain[:i], "/")
			return
		}
	}
}

// MatchPathInLocality determines if a given link is in the locality domain scope
// and returns the link with version matching the one of the matched locality
// domain.
func (ld localityDomain) MatchPathInLocality(link string, rhs resourcehandlers.Registry) (string, bool) {
	if rh := rhs.Get(link); rh != nil {
		var (
			key, path string
			err       error
		)
		if key, path, _, err = rh.GetLocalityDomainCandidate(link); err != nil {
			return link, false
		}
		localityDomain, ok := ld[key]
		if !ok {
			return link, false
		}

		var exclude, include bool
		// check if the link is not in locality scope by explicit exclude
		if len(localityDomain.Exclude) > 0 {
			for _, rx := range localityDomain.Exclude {
				if exclude, err = regexp.MatchString(rx, link); err != nil {
					klog.Warningf("exclude pattern match %s failed for %s\n", localityDomain.Exclude, link)
				}
				if exclude {
					break
				}
			}
		}
		// check if the link is in locality scope by explicit include
		if len(localityDomain.Include) > 0 {
			for _, rx := range localityDomain.Include {
				if include, err = regexp.MatchString(rx, link); err != nil {
					klog.Warningf("include pattern match %s failed for %s\n", localityDomain.Include, link)
				}
				if include {
					exclude = false
					break
				}
			}
		}
		if exclude {
			if link, err = rh.SetVersion(link, localityDomain.Version); err != nil {
				klog.Errorf("%v\n", err)
				return link, false
			}
			return link, true
		}

		prefix := localityDomain.Path
		// FIXME: this is tmp valid only for github urls
		if strings.HasPrefix(path, prefix) {
			if link, err = rh.SetVersion(link, localityDomain.Version); err != nil {
				klog.Errorf("%v\n", err)
				return link, false
			}
			return link, true
		}
		// check if in the same repo and then enforce verions rewrite
		_s := strings.Split(prefix, "/")
		_s = _s[:len(_s)-1]
		repoPrefix := strings.Join(_s, "/")
		if strings.HasPrefix(path, repoPrefix) {
			if link, err = rh.SetVersion(link, localityDomain.Version); err != nil {
				klog.Errorf("%v\n", err)
				return link, false
			}
		}
	}
	return link, false
}

// PathInLocality determines if a given link is in the locality domain scope
func (ld localityDomain) PathInLocality(link string, rhs resourcehandlers.Registry) bool {
	if rh := rhs.Get(link); rh != nil {
		var (
			key, path, version string
			err                error
		)
		if key, path, version, err = rh.GetLocalityDomainCandidate(link); err != nil {
			return false
		}
		localityDomain, ok := ld[key]
		if !ok {
			return false
		}
		klog.V(6).Infof("Path %s in locality domain %s: %v\n", path, localityDomain, strings.HasPrefix(path, localityDomain.Path))
		// TODO: locality domain to be constructed from key for comparison
		return reflect.DeepEqual(localityDomain, &localityDomainValue{
			version,
			path,
			localityDomain.Include,
			localityDomain.Exclude,
			localityDomain.LinkSubstitutes,
			localityDomain.DownloadSubstitutes,
		})
	}
	return false
}

func (ld localityDomain) SubstituteLink(link string) string {
	if len(link) > 0 {
		for _, d := range ld {
			if len(d.LinkSubstitutes) > 0 {
				if s, ok := d.LinkSubstitutes[link]; ok {
					return s
				}
			}
		}
	}
	return link
}

func (ld localityDomain) GetDownloadedResourceName(u *urls.URL) string {
	k := strings.TrimPrefix(u.Path, "/")
	id := uuid.New().String()
	for _, d := range ld {
		if len(d.DownloadSubstitutes) > 0 {
			for substituteMatcher, s := range d.DownloadSubstitutes {
				var (
					matched bool
					err     error
				)
				if matched, err = regexp.MatchString(substituteMatcher, k); err != nil {
					klog.Warningf("download subsitution pattern match %s failed for %s\n", substituteMatcher, k)
					break
				}
				if matched {
					s = strings.ReplaceAll(s, "$name", u.ResourceName)
					s = strings.ReplaceAll(s, "$uuid", id)
					s = strings.ReplaceAll(s, "$path", u.ResourcePath)
					s = strings.ReplaceAll(s, "$ext", u.Extension)
					return s
				}
			}
		}
	}
	if len(u.Extension) > 0 {
		s := fmt.Sprintf("%s.%s", id, u.Extension)
		return s
	}
	return id
}

// setLocalityDomainForNode visits all content selectors in the node and its
// descendants to build a localityDomain
func localityDomainFromNode(node *api.Node, rhs resourcehandlers.Registry) (localityDomain, error) {
	var localityDomains = make(localityDomain, 0)
	if err := csHandle(node.ContentSelectors, localityDomains, rhs); err != nil {
		return nil, err
	}
	if node.Nodes != nil {
		if err := fromNodes(node.Nodes, localityDomains, rhs); err != nil {
			return nil, err
		}
	}
	return localityDomains, nil
}

func csHandle(contentSelectors []api.ContentSelector, localityDomains localityDomain, rhs resourcehandlers.Registry) error {
	for _, cs := range contentSelectors {
		if rh := rhs.Get(cs.Source); rh != nil {
			key, path, version, err := rh.GetLocalityDomainCandidate(cs.Source)
			if err != nil {
				return err
			}
			localityDomains.Set(key, path, version)
		}
	}
	return nil
}

func fromNodes(nodes []*api.Node, localityDomains localityDomain, rhs resourcehandlers.Registry) error {
	for _, node := range nodes {
		csHandle(node.ContentSelectors, localityDomains, rhs)
		if err := fromNodes(node.Nodes, localityDomains, rhs); err != nil {
			return err
		}
	}
	return nil
}

// ResolveLocalityDomain resolves the actual locality domain for a node,
// considering the global one (if any) and locally defined one.
// If no localityDomain is defined on the node the function returns nil
func resolveLocalityDomain(node *api.Node, globalLD localityDomain) localityDomain {
	if nodeLD := node.LocalityDomain; nodeLD != nil {
		nodeLD := copy(nodeLD)
		if globalLD == nil {
			return copy(node.LocalityDomain)
		}
		ld := localityDomain{}
		for k, v := range globalLD {
			ld[k] = &localityDomainValue{
				v.Version,
				v.Path,
				v.Exclude,
				v.Include,
				v.LinkSubstitutes,
				v.DownloadSubstitutes,
			}
		}
		for k, nodeV := range nodeLD {
			if globalV, ok := ld[k]; ok {
				ld[k] = merge(globalV, nodeV)
				continue
			}
			ld[k] = nodeV
		}
		return ld
	}
	return globalLD
}

// replaces Version and Path from b in a if any
// merges Exclude and Include from b in a if any
// merges LinkSubstitutes and DownloadSubstitutes from b in a if any,
// replacing duplicate entries in a with entries from b.
func merge(a, b *localityDomainValue) *localityDomainValue {
	if len(b.Version) > 0 {
		a.Version = b.Version
	}
	if len(b.Path) > 0 {
		a.Path = b.Path
	}
	if len(b.Exclude) > 0 {
		_e := []string{}
		if len(a.Exclude) > 0 {
			_e = append(_e, a.Exclude...)
		}
		a.Exclude = append(_e, b.Exclude...)
	}
	if len(b.Include) > 0 {
		_e := []string{}
		if len(a.Include) > 0 {
			_e = append(_e, a.Include...)
		}
		a.Include = append(_e, b.Include...)
	}
	if len(b.LinkSubstitutes) > 0 {
		_e := a.LinkSubstitutes
		if _e == nil {
			_e = b.LinkSubstitutes
		} else {
			for k, v := range b.LinkSubstitutes {
				_e[k] = v
			}
		}
		a.LinkSubstitutes = _e
	}
	if len(b.DownloadSubstitutes) > 0 {
		_e := a.DownloadSubstitutes
		if _e == nil {
			_e = b.DownloadSubstitutes
		} else {
			for k, v := range b.DownloadSubstitutes {
				_e[k] = v
			}
		}
		a.DownloadSubstitutes = _e
	}
	return a
}
