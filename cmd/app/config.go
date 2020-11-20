// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gardener/docforge/pkg/api"
	"github.com/gardener/docforge/pkg/readers"
	"github.com/gardener/docforge/pkg/resourcehandlers"
)

type httpReaderRegistree struct {
	client *http.Client
	auth   string
	hosts  []string
}

type HttpClientOptions struct {
	Authorization string
	Hosts         []string
}

func NewHttpHandler(opts *HttpClientOptions) resourcehandlers.URIValidator {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    30 * time.Second,
			DisableCompression: true,
		},
	}
	h := &httpReaderRegistree{
		client: client,
		auth:   fmt.Sprintf("token %s", opts.Authorization),
		hosts:  opts.Hosts,
	}
	return h
}

// Accept returns true for valid URLs, with http(s) scheme
func (r *httpReaderRegistree) Accept(uri string) bool {
	if u, err := url.Parse(uri); err == nil {
		if strings.HasPrefix(u.Scheme, "http") {
			return true
		}
	}
	return false
}

func (r *httpReaderRegistree) Read(ctx context.Context, uri string) ([]byte, error) {
	req, err := http.NewRequest("GET", uri, nil)
	req.Header.Add("Authorization", r.auth)
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode > 399 {
		return nil, fmt.Errorf("Get %s failed: %s", uri, resp.Status)
	}
	return ioutil.ReadAll(resp.Body)
}

// WithManifestReaders amends registry with additional readers, adapting
// it to read manifests from various locations
func WithManifestReaders(o *HttpClientOptions, handlers ...resourcehandlers.URIValidator) resourcehandlers.Registry {
	httpReader := NewHttpHandler(o)
	readers := append(handlers, httpReader)
	return resourcehandlers.NewRegistry(readers...)
}

// Manifest creates documentation model from configration file
func Manifest(ctx context.Context, uri string, registry resourcehandlers.Registry) (*api.Documentation, error) {
	var (
		docs        *api.Documentation
		err         error
		configBytes []byte
		handler     resourcehandlers.URIValidator
	)
	uri = strings.TrimSpace(uri)
	if handler = registry.Get(uri); handler == nil {
		return nil, fmt.Errorf("no suitable reader found for %s. is this path correct?", uri)
	}
	if reader, ok := handler.(readers.ContextResourceReader); ok {
		if configBytes, err = reader.Read(ctx, uri); err != nil {
			return nil, err
		}
	}
	if docs, err = api.Parse(configBytes); err != nil {
		return nil, err
	}
	return docs, nil
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
