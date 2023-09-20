// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import "github.com/gardener/docforge/pkg/manifest"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

// Writer writes blobs with name to a given path
//
//counterfeiter:generate . Writer
type Writer interface {
	Write(name, path string, resourceContent []byte, node *manifest.Node) error
}
