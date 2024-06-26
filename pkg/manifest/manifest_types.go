// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifest

// FileType represent a file node
type FileType struct {
	// File is the renaming of the file from source. If Source is empty then File should contain the url
	File string `yaml:"file,omitempty"`
	// Source is the source of file. If empty File must be the url
	Source string `yaml:"source,omitempty"`
	// MultiSource is a file build from multiple sources
	MultiSource []string `yaml:"multiSource,omitempty"`
}

// DirType represents a directory node
type DirType struct {
	// Dir name of dir
	Dir string `yaml:"dir,omitempty"`
	// Structure is the node content of dir
	Structure []*Node `yaml:"structure,omitempty"`
}

// FilesTreeType represents a fileTree node
type FilesTreeType struct {
	// FileTree is a tree url of a repo
	FileTree string `yaml:"fileTree,omitempty"`
	// ExcludeFiles files to be excluded
	ExcludeFiles []string `yaml:"excludeFiles,omitempty"`
}

// ManifType represents a manifest node
type ManifType struct {
	// Manifest is the manifest url
	Manifest string `yaml:"manifest,omitempty"`
}
