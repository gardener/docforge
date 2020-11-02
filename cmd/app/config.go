// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"io/ioutil"

	"github.com/gardener/docforge/pkg/api"
)

// Manifest creates documentation model from configration file
func Manifest(filePath string) *api.Documentation {
	var (
		docs *api.Documentation
	)
	configBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	if docs, err = api.Parse(configBytes); err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	return docs
}
