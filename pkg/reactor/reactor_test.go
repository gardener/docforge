package reactor

import (
	"fmt"
	"testing"

	"github.com/gardener/docode/pkg/api"
	"github.com/gardener/docode/pkg/backend/github"
	"github.com/gardener/docode/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

var nodes = &api.Node{
	Source: []string{
		"http://gthub.com/owner/repo/tree/resourceA",
	},
	Nodes: []*api.Node{
		&api.Node{
			Source: []string{
				"http://gthub.com/owner/repo/tree/resourceB",
			},
		},
		&api.Node{
			Source: []string{
				"http://gthub.com/owner/repo/tree/resourceC",
				"http://gthub.com/owner/repo/tree/resourceD#{h1:first-of-type}",
			},
			Nodes: []*api.Node{
				&api.Node{
					Source: []string{
						"http://gthub.com/owner/repo/tree/resourceB",
					},
				},
			},
		},
	},
}

func TestSource(t *testing.T) {
	resourcePathsSet := make(map[string]struct{})
	sources(nodes, resourcePathsSet)
	for k := range resourcePathsSet {
		gh := &github.GitHub{}
		fmt.Println(gh.DownloadUrl(k))
	}
}
