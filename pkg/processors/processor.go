// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package processors

import (
	"github.com/gardener/docforge/pkg/api"
)

// Processor is used by extensions to transform a document
type Processor interface {
	Process(documentBlob []byte, node *api.Node) ([]byte, error)
}

// ProcessorChain is a registry of ordered document processors
// that implements Processor#Process
type ProcessorChain struct {
	Processors []Processor
}

// Process implements Processor#Process invoking the registered chain of Processors sequentially
func (p *ProcessorChain) Process(documentBlob []byte, node *api.Node) ([]byte, error) {
	var err error
	for _, p := range p.Processors {
		documentBlob, err = p.Process(documentBlob, node)
	}
	return documentBlob, err
}
