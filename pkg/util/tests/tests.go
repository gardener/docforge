// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"flag"
	"math/rand"
	"strconv"
	"time"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}

// SetKlogV sets the logging flags when unit tests are run
func SetKlogV(level int) {
	l := strconv.Itoa(level)
	if f := flag.Lookup("v"); f != nil {
		f.Value.Set(l)
	}
	if f := flag.Lookup("logtostderr"); f != nil {
		f.Value.Set("true")
	}
}

// StrPtr is a convenience one-liner for producing pointers
// to string values
func StrPtr(s string) *string { return &s }
