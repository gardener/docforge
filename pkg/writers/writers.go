// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate -header ../../license_prefix.txt

import "github.com/gardener/docforge/pkg/manifestadapter"

// Writer writes blobs with name to a given path
//
//counterfeiter:generate . Writer
type Writer interface {
	Write(name, path string, resourceContent []byte, node *manifestadapter.Node) error
}
