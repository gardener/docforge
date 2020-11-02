// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package units

// Byte size suffixes.
const (
	B  int64 = 1
	KB int64 = 1 << (10 * iota)
	MB
	GB
)
