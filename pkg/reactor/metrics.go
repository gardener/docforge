// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor

import (
	"net/http"

	"github.com/gardener/docforge/pkg/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// ResetMetrics resets the metrics
func ResetMetrics() {
	metrics.ResetClientMetrics()
}

// RegisterMetrics registers the http client metrics.
// The second parameter `reg` can be used to provide a custom registry, e.g. for tests.
func RegisterMetrics(reg prometheus.Registerer) {
	ResetMetrics()
	metrics.RegisterClientMetrics(reg)
}

// InstrumentClient instruments the client provided as argument to measure and report networking metrics
func InstrumentClient(client *http.Client) *http.Client {
	return metrics.InstrumentClientRoundTripperDuration(client)
}
