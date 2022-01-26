// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package configuration_test

import (
	"errors"
	"github.com/gardener/docforge/cmd/configuration"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/pointer"
	"os"
	"path/filepath"
	"testing"
)

var (
	rename         bool
	userHomerDir   string
	defaultCfgFile string
)

func TestConfiguration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Configuration Suite")
}

// renames the default configuration file if exists
var _ = BeforeSuite(func() {
	var err error
	userHomerDir, err = os.UserHomeDir()
	Expect(err).NotTo(HaveOccurred())
	defaultCfgDir := filepath.Join(userHomerDir, configuration.DocforgeHomeDir)
	if err = os.Mkdir(defaultCfgDir, 0644); !errors.Is(err, os.ErrExist) {
		Expect(err).NotTo(HaveOccurred())
	}
	defaultCfgFile = filepath.Join(defaultCfgDir, configuration.DefaultConfigFileName)
	if _, err = os.Stat(defaultCfgFile); err == nil {
		Expect(os.Rename(defaultCfgFile, defaultCfgFile+"_org")).To(Succeed())
		rename = true
	}
})

// restores original default configuration file
var _ = AfterSuite(func() {
	if rename {
		Expect(os.Rename(defaultCfgFile+"_org", defaultCfgFile)).To(Succeed())
	}
})

var _ = Describe("Configuration Loader", func() {
	var (
		file   string
		setEnv bool
		loader configuration.Loader
		cfg    *configuration.Config
		err    error
	)
	BeforeEach(func() {
		loader = new(configuration.DefaultConfigurationLoader)
	})
	JustBeforeEach(func() {
		if setEnv {
			Expect(os.Setenv(configuration.DocforgeConfigEnv, file)).To(Succeed())
		}
		cfg, err = loader.Load()
	})
	JustAfterEach(func() {
		if setEnv {
			Expect(os.Unsetenv(configuration.DocforgeConfigEnv)).To(Succeed())
		}
	})
	When("environment not set", func() {
		BeforeEach(func() {
			setEnv = false
		})
		It("creates empty configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).To(Equal(&configuration.Config{}))
		})
	})
	When("configuration file name is empty", func() {
		BeforeEach(func() {
			setEnv = true
			file = ""
		})
		It("errors", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("DOCFORGECONFIG"))
			Expect(cfg).To(BeNil())
		})
	})
	When("configuration file name is directory", func() {
		BeforeEach(func() {
			setEnv = true
			file = "testdata"
		})
		It("errors", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("directory"))
			Expect(cfg).To(BeNil())
		})
	})
	When("configuration file is missing", func() {
		BeforeEach(func() {
			setEnv = true
			file = "testdata/missing.yaml"
		})
		It("creates empty configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).To(Equal(&configuration.Config{}))
		})
	})
	When("load configuration file", func() {
		BeforeEach(func() {
			setEnv = true
			file = "testdata/config_full.yaml"
		})
		It("creates the configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).To(Equal(&configuration.Config{
				CacheHome:        pointer.StringPtr("~/.docforge/cache_old"),
				Credentials:      []*configuration.Credentials{{Host: "github.com", Username: pointer.StringPtr("Bob"), OAuthToken: pointer.StringPtr("s0m3tok3n")}},
				ResourceMappings: map[string]string{"https://github.com/gardener/gardener/tree/master/docs": "/usr/user/home/git/github.com/gardener/gardener/docs"},
				Hugo:             &configuration.Hugo{Enabled: false, PrettyURLs: true, BaseURL: "/gardener", IndexFileNames: []string{"index.md"}},
				DefaultBranches:  map[string]string{"default": "master", "https://github.com/myrepo": "main"},
				LastNVersions:    map[string]int{"default": 0, "https://github.com/myrepo": 3},
			}))
		})
	})
	When("load default configuration file", func() {
		var (
			expCfg *configuration.Config
		)
		BeforeEach(func() {
			setEnv = false
			file = defaultCfgFile
			_, err = os.Stat(file)
			Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
			expCfg = &configuration.Config{Credentials: []*configuration.Credentials{{Host: "github.com", Username: pointer.StringPtr("Bob"), OAuthToken: pointer.StringPtr("s0m3tok3n")}}}
			var cnt []byte
			cnt, err = yaml.Marshal(expCfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(file, cnt, 0644)).To(Succeed())
		})
		AfterEach(func() {
			Expect(os.Remove(file)).To(Succeed())
		})
		It("creates the default configuration", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).To(Equal(expCfg))
		})
	})
})
