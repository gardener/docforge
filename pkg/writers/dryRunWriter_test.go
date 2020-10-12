package writers

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormat(t *testing.T) {
	var (
		b     bytes.Buffer
		bytes []byte
		err   error
	)
	in := []string{
		"dev/doc",
		"dev/doc/aws_provider.md",
		"dev/doc/gardener",
		"dev/doc/gardener/_index.md",
		"dev/doc/gardener/concepts",
		"dev/doc/gardener/concepts/apiserver.md",
		"dev/doc/gardener/deployment",
		"dev/doc/gardener/deployment/aks.md",
		"dev/doc/gardener/deployment/deploy_gardenlet.md",
		"dev/doc/gardener/deployment/feature_gates.md",
		"dev/doc/gardener/proposals",
		"dev/doc/gardener/proposals/00-template.md",
		"dev/doc/gardener/proposals/01-extensibility.md",
		"dev/doc/gardener/proposals/_index.md",
		"dev/doc/gardener/testing",
		"dev/doc/gardener/testing/integration_tests.md",
		"dev/doc/gardener/usage",
		"dev/doc/gardener/usage/configuration.md",
		"dev/doc/gardener/usage/control_plane_migration.md",
	}
	out := `dev
  doc
    aws_provider.md
    gardener
      _index.md
      concepts
        apiserver.md
      deployment
        aks.md
        deploy_gardenlet.md
        feature_gates.md
      proposals
        00-template.md
        01-extensibility.md
        _index.md
      testing
        integration_tests.md
      usage
        configuration.md
        control_plane_migration.md
`

	files := []*file{}
	for _, p := range in {
		files = append(files, &file{
			path: p,
		})
	}
	format(files, &b)

	if bytes, err = ioutil.ReadAll(&b); err != nil {
		t.Error(err.Error())
	}
	assert.Equal(t, out, string(bytes))
}
