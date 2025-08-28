// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package osshim

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../../license_prefix.txt

import (
	"os"
)

// Os is shim for methods from os package
//
//counterfeiter:generate . Os
type Os interface {
	ReadFile(name string) ([]byte, error)
	IsNotExist(err error) bool
	IsDir(path string) (bool, error)
	MkdirAll(path string, perm int) error
	WriteFile(name string, data []byte, perm int) error
}

// OsShim is default Os implementation
type OsShim struct{}

// ReadFile see os.ReadFile
func (sh *OsShim) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// IsNotExist see os.IsNotExist
func (sh *OsShim) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// IsDir checks if a given path is a dir
func (sh *OsShim) IsDir(path string) (bool, error) {
	lstat, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	return lstat.IsDir(), nil
}

// MkdirAll creates a directory named path, along with any necessary parents
func (sh *OsShim) MkdirAll(path string, perm int) error {
	return os.MkdirAll(path, os.FileMode(perm))
}

// WriteFile writes data to the named file
func (sh *OsShim) WriteFile(name string, data []byte, perm int) error {
	return os.WriteFile(name, data, os.FileMode(perm))
}
