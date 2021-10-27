// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package httpclient

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import "net/http"

//counterfeiter:generate . Client
type Client interface {
	Do(req *http.Request) (resp *http.Response, err error)
}
