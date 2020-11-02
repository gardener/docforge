// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package writers

import "github.com/gardener/docforge/pkg/api"

// Writer writes blobs with name to a given path
type Writer interface {
	Write(name, path string, resourceContent []byte, node *api.Node) error
}
