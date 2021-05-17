package configuration

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigFileName = "config"
	DocforgeHomeDir       = ".docforge"
)

type ConfigurationLoader interface {
	Load() (*Config, error)
}

type DefaultConfigurationLoader func() (*Config, error)

func (d *DefaultConfigurationLoader) Load() (*Config, error) {
	if configFilePath, found := os.LookupEnv("DOCFORGECONFIG"); found {
		if configFilePath == "" {
			return nil, fmt.Errorf("the provided environment variable DOCFORGECONFIG is set to empty string")
		}
		return load(configFilePath)
	}

	userHomerDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %v", err)
	}

	configFilePath := filepath.Join(userHomerDir, DocforgeHomeDir, defaultConfigFileName)
	return load(configFilePath)
}

func load(configFilePath string) (*Config, error) {
	stat, err := os.Stat(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for configuration file path %s: %v", configFilePath, err)
	}
	if stat.IsDir() {
		panic(fmt.Errorf("the config file path %s is directory, instead of file", configFilePath))
	}
	configFile, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}

	config := &Config{}
	if err := yaml.Unmarshal(configFile, config); err != nil {
		return nil, err
	}
	return config, nil
}
