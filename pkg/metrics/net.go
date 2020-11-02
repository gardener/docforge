// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Client metrics

	clientInFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: Namespace,
		Name:      "client_in_flight_requests",
		Help:      "A gauge of in-flight requests for the wrapped client.",
	})

	clientCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: Namespace,
		Name:      "client_api_requests_total",
		Help:      "A counter for requests from the wrapped client.",
	},
		[]string{"code", "method"},
	)

	// dnsLatencyVec uses custom buckets based on expected dns durations.
	// It has an instance label "event", which is set in the
	// DNSStart and DNSDonehook functions defined in the
	// InstrumentTrace struct below.
	clientDNSLatencyVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "dns_duration_seconds",
		Help:      "Trace dns latency histogram.",
		Buckets:   []float64{.005, .01, .025, .05},
	},
		[]string{"event"},
	)

	// tlsLatencyVec uses custom buckets based on expected tls durations.
	// It has an instance label "event", which is set in the
	// TLSHandshakeStart and TLSHandshakeDone hook functions defined in the
	// InstrumentTrace struct below.
	clclientTLSLatencyVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "tls_duration_seconds",
		Help:      "Trace tls latency histogram.",
		Buckets:   []float64{.05, .1, .25, .5},
	},
		[]string{"event"},
	)

	// histVec has no labels, making it a zero-dimensional ObserverVec.
	clientHistVec = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: Namespace,
		Name:      "request_duration_seconds",
		Help:      "A histogram of request latencies.",
		Buckets:   prometheus.DefBuckets,
	},
		[]string{},
	)
)

// RegisterClientMetrics registers all of the metrics in the standard registry.
func RegisterClientMetrics(registry prometheus.Registerer) {
	ResetClientMetrics()
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}
	registry.MustRegister(clientCounter, clclientTLSLatencyVec, clientDNSLatencyVec, clientHistVec, clientInFlightGauge)
}

// ResetClientMetrics resets the HTTP client metrics. The function is useful for designing self-contained unit tests
// where the count of metrics matters.
func ResetClientMetrics() {
	clientCounter.Reset()
	clclientTLSLatencyVec.Reset()
	clientDNSLatencyVec.Reset()
	clientHistVec.Reset()
	clientInFlightGauge.Set(0.0)
}

// InstrumentClientRoundTripperDuration instruments the provided HTTP client for metering HTTP roundtrip duration
func InstrumentClientRoundTripperDuration(client *http.Client) *http.Client {
	// Define functions for the available httptrace.ClientTrace hook
	// functions that we want to instrument.
	trace := &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			clientDNSLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			clientDNSLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			clclientTLSLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			clclientTLSLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	// Wrap the default RoundTripper with middleware.
	roundTripper := promhttp.InstrumentRoundTripperInFlight(clientInFlightGauge,
		promhttp.InstrumentRoundTripperCounter(clientCounter,
			promhttp.InstrumentRoundTripperTrace(trace,
				promhttp.InstrumentRoundTripperDuration(clientHistVec, http.DefaultTransport),
			),
		),
	)

	// Set the RoundTripper on our client.
	client.Transport = roundTripper

	return client
}
