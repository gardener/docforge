package reactor

import (
	"fmt"
	"strings"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/resourcehandlers"
)

// LocalityDomain holds the
type LocalityDomain map[string]string

// SetLocalityDomain ...
func (ld LocalityDomain) SetLocalityDomain(key, path string) {
	var (
		existingLD string
		ok         bool
	)
	if existingLD, ok = ld[key]; !ok {
		ld[key] = path
		return
	}

	localityDomain := strings.Split(existingLD, "/")
	localityDomainCandidate := strings.Split(path, "/")
	for i := range localityDomain {
		if len(localityDomainCandidate) <= i || localityDomain[i] != localityDomainCandidate[i] {
			ld[key] = strings.Join(localityDomain[:i], "/")
			return
		}
	}
}

// PathInLocality determines if a given path is in the locality domain scope
func (ld LocalityDomain) PathInLocality(key, path string) bool {
	localityDomain, ok := ld[key]
	if !ok {
		return false
	}

	fmt.Printf("Is path: %s in locality: %v \n", path, localityDomain)
	// TODO: locality domain to be constructed from key for comparison
	return strings.HasPrefix(path, localityDomain)
}

func defineLocalityDomains(docStruct *api.Node) (LocalityDomain, error) {
	var localityDomains = make(LocalityDomain, 0)
	if err := csHandle(docStruct.ContentSelectors, localityDomains); err != nil {
		return nil, err
	}
	if err := fromNodes(docStruct.Nodes, localityDomains); err != nil {
		return nil, err
	}

	return localityDomains, nil
}

func csHandle(contentSelectors []api.ContentSelector, localityDomains LocalityDomain) error {
	for _, cs := range contentSelectors {
		rh := resourcehandlers.Get(cs.Source)
		key, path, err := rh.GetLocalityDomainCandidate(cs.Source)
		if err != nil {
			return err
		}
		localityDomains.SetLocalityDomain(key, path)
	}
	return nil
}

func fromNodes(nodes []*api.Node, localityDomains LocalityDomain) error {
	for _, node := range nodes {
		csHandle(node.ContentSelectors, localityDomains)
		if err := fromNodes(node.Nodes, localityDomains); err != nil {
			return err
		}
	}
	return nil
}
