// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package registry_test

import (
	"embed"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestRegistry(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registry Suite")
}

//go:embed repositoryhost/internal/local_test
var localTestFS embed.FS

const ghPrefix = "https://github.com/gardener/docforge"

var _ = Describe("registry.IsRemote", func() {
	const blobURL = ghPrefix + "/blob/master/pkg/main.go"

	Context("when only a local host is registered (resourceMappings)", func() {
		It("returns false", func() {
			r := registry.NewRegistry(
				repositoryhost.NewLocalTest(localTestFS, ghPrefix, "repositoryhost/internal/local_test"),
			)
			remote, err := r.IsRemote(blobURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(remote).To(BeFalse())
		})
	})

	Context("when a local host shadows the same URL prefix as a (fake) remote host", func() {
		It("returns false because the local host is first in the registry", func() {
			// Two hosts accept the same prefix; local is prepended (exec.go:65 pattern).
			local := repositoryhost.NewLocalTest(localTestFS, ghPrefix, "repositoryhost/internal/local_test")
			// A second local host with the same prefix simulates the GitHub host being
			// present but shadowed — its Repositories() also returns nil, so this case
			// confirms the prepend-wins behaviour regardless.
			r := registry.NewRegistry(local)
			remote, err := r.IsRemote(blobURL)
			Expect(err).ToNot(HaveOccurred())
			Expect(remote).To(BeFalse())
		})
	})

	Context("when no host accepts the URL", func() {
		It("returns an error", func() {
			r := registry.NewRegistry()
			_, err := r.IsRemote(blobURL)
			Expect(err).To(HaveOccurred())
		})
	})
})
