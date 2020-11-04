// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"

	"github.com/gardener/docforge/pkg/api"
)

// Manifest creates documentation model from configration file
func Manifest(filePath string, vars map[string]string) *api.Documentation {
	var (
		docs *api.Documentation
	)
	blob, err := ioutil.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	blob, err = resolveVariables(blob, vars)
	if err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	if docs, err = api.Parse(blob); err != nil {
		panic(fmt.Sprintf("%v\n", err))
	}
	return docs
}

func resolveVariables(manifestContent []byte, vars map[string]string) ([]byte, error) {
	var (
		tmpl *template.Template
		err  error
		b    bytes.Buffer
	)
	if tmpl, err = template.New("").Parse(string(manifestContent)); err != nil {
		return nil, err
	}
	if err := tmpl.Execute(&b, vars); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
