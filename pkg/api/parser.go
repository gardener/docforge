// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"gopkg.in/yaml.v3"
)

// Parse is ...
func Parse(b []byte) (*Documentation, error) {
	var docs = &Documentation{}
	if err := yaml.Unmarshal(b, docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// Serialize is ...
func Serialize(docs *Documentation) (string, error) {
	var (
		err error
		b   []byte
	)
	if b, err = yaml.Marshal(docs); err != nil {
		return "", err
	}
	return string(b), nil
}
