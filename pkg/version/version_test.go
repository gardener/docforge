// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	. "github.com/gardener/docforge/pkg/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("version", func() {
	Describe("version", func() {
		It("should not return a specific version number", func() {
			Expect(Version).To(Equal("binary was not built properly"))
		})
	})
})
