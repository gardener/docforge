// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

// Stat represents a category recorded by StatsRecorder
type Stat struct {
	Title   string
	Figures string
	Details []string
}
