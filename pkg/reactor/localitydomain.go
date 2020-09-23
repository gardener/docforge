package reactor

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/resourcehandlers"
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
	Version string
	Path    string
}

// fromAPI creates new localityDomain copy object from
// api.LocalityDomain
func fromAPI(ld api.LocalityDomain) localityDomain {
	localityDomain := localityDomain{}
	for k, v := range ld {
		localityDomain[k] = &localityDomainValue{
			v.Version,
			v.Path,
		}
	}
	return localityDomain
}

// Set creates or updates a locality domain entry
// with key and path. An update is performed when
// the path is ancestor fo the existing path for
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
func (ld localityDomain) MatchPathInLocality(link string) (string, bool) {
	if rh := resourcehandlers.Get(link); rh != nil {
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
		// TODO: locality domain to be constructed from key for comparison
		if strings.HasPrefix(path, localityDomain.Path) {
			if link, err = rh.SetVersion(link, localityDomain.Version); err != nil {
				fmt.Printf("%v\n", err)
				return link, false
			}
			return link, true
		}
	}
	return link, false
}

// PathInLocality determines if a given link is in the locality domain scope
func (ld localityDomain) PathInLocality(link string) bool {
	if rh := resourcehandlers.Get(link); rh != nil {
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
		fmt.Printf("Path %s in locality domain %s: %v\n", path, localityDomain, strings.HasPrefix(path, localityDomain.Path))
		// TODO: locality domain to be constructed from key for comparison
		return reflect.DeepEqual(localityDomain, &localityDomainValue{
			version,
			path,
		})
	}
	return false
}

// setLocalityDomainForNode visits all content selectors in the node and its
// descendants to build a localityDomain
func setLocalityDomainForNode(node *api.Node) (localityDomain, error) {
	var localityDomains = make(localityDomain, 0)
	if err := csHandle(node.ContentSelectors, localityDomains); err != nil {
		return nil, err
	}
	if node.Nodes != nil {
		if err := fromNodes(node.Nodes, localityDomains); err != nil {
			return nil, err
		}
	}
	return localityDomains, nil
}

func csHandle(contentSelectors []api.ContentSelector, localityDomains localityDomain) error {
	for _, cs := range contentSelectors {
		if rh := resourcehandlers.Get(cs.Source); rh != nil {
			key, path, version, err := rh.GetLocalityDomainCandidate(cs.Source)
			if err != nil {
				return err
			}
			localityDomains.Set(key, path, version)
		}
	}
	return nil
}

func fromNodes(nodes []*api.Node, localityDomains localityDomain) error {
	for _, node := range nodes {
		csHandle(node.ContentSelectors, localityDomains)
		if err := fromNodes(node.Nodes, localityDomains); err != nil {
			return err
		}
	}
	return nil
}
