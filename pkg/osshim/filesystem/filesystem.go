// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package filesystem

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Interface is the interface for working with the file system
type Interface interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(filename string, data []byte, perm int) error
	IsNotExist(err error) bool
	IsDir(path string) (bool, error)
	MkdirAll(path string, perm int) error
	FilePathsInDir(dirPath string) ([]string, error)
	Join(elem ...string) string
}

// Local file system
type Local struct{}

// ReadFile see os.ReadFile
func (sh *Local) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile see os.WriteFile
func (sh *Local) WriteFile(filename string, data []byte, perm int) error {
	return os.WriteFile(filename, data, os.FileMode(perm))
}

// IsNotExist see os.IsNotExist
func (sh *Local) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// IsDir checks if a given path is a dir
func (sh *Local) IsDir(path string) (bool, error) {
	lstat, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return lstat.IsDir(), nil
}

// MkdirAll see os.MkdirAll
func (sh *Local) MkdirAll(path string, perm int) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

// FilePathsInDir returns files paths in a given directory
func (sh *Local) FilePathsInDir(dirPath string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, strings.TrimPrefix(strings.TrimPrefix(path, dirPath), "/"))
		}
		return nil
	})
	if err != nil {
		return []string{}, fmt.Errorf("error getting directory %s files paths: %w", dirPath, err)
	}
	return files, nil
}

// Join joins any number of path elements into a single path
func (sh *Local) Join(elem ...string) string {
	return filepath.Join(elem...)
}
