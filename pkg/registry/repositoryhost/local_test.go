package repositoryhost_test

// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
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
