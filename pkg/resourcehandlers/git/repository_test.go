// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestRepository_Prepare(t *testing.T) {
	type fields struct {
		Auth          http.AuthMethod
		LocalPath     string
		RemoteURL     string
		State         State
		PreviousError error
	}
	type args struct {
		ctx    context.Context
		branch string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repository{
				Auth:          tt.fields.Auth,
				LocalPath:     tt.fields.LocalPath,
				RemoteURL:     tt.fields.RemoteURL,
				State:         tt.fields.State,
				PreviousError: tt.fields.PreviousError,
			}
			if err := r.Prepare(tt.args.ctx, tt.args.branch); (err != nil) != tt.wantErr {
				t.Errorf("Repository.Prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_prepare(t *testing.T) {
	type fields struct {
		Auth          http.AuthMethod
		LocalPath     string
		RemoteURL     string
		State         State
		PreviousError error
	}
	type args struct {
		ctx    context.Context
		branch string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Repository{
				Auth:          tt.fields.Auth,
				LocalPath:     tt.fields.LocalPath,
				RemoteURL:     tt.fields.RemoteURL,
				State:         tt.fields.State,
				PreviousError: tt.fields.PreviousError,
			}
			if err := r.prepare(tt.args.ctx, tt.args.branch); (err != nil) != tt.wantErr {
				t.Errorf("Repository.prepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
