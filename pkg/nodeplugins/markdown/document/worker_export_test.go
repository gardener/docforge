// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document

// SetFailOnStrippedContent exposes the unexported field for white-box testing from
// the document_test package. Only compiled during `go test`.
func (w *Worker) SetFailOnStrippedContent(v bool) {
	w.failOnStrippedContent = v
}
