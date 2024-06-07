package repositoryhost_test

// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

import (
	"embed"
	_ "embed"

	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
)

//go:embed internal/local_test/*
var repo embed.FS

var _ = Describe("Local cache test", func() {
	testRepositoryHost(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "internal/local_test"))
})
