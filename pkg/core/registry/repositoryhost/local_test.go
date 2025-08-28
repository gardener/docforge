package repositoryhost_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Local cache test", func() {
	testRepositoryHost(repositoryhost.NewLocal("https://github.com/gardener/docforge", "internal/local_test"))
})
