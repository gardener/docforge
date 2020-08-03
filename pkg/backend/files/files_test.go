// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package files

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/util/tests"
	"github.com/stretchr/testify/assert"
)

func init() {
	tests.SetGlogV(6)
}

// FileWorker tests
func TestFileWorker(t *testing.T) {
	var (
		actual               bool
		err                  error
		backendRequestsCount int
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendRequestsCount++
		defer r.Body.Close()
		if _, err = ioutil.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		actual = true
		w.Write([]byte("123"))
	}))
	defer backend.Close()
	w := &FileWorker{}
	task := &FileTask{}

	workerError := w.Work(context.Background(), task)

	assert.Nil(t, err)
	assert.Nil(t, workerError)
	assert.True(t, actual)
	assert.Equal(t, 1, backendRequestsCount)
}

func TestFileWorkerResponseFault(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()
	w := &FileWorker{}

	err := w.Work(context.Background(), &FileTask{})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("sending task to resource %s failed with response code 500", backend.URL), err.Error())
}

func TestFileWorkerCtxTimeout(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
	}))
	defer backend.Close()
	w := &FileWorker{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Work(ctx, &FileTask{})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("Get %q: context deadline exceeded", backend.URL), err.Error())
}

func TestFileWorkerCtxCancel(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
	}))
	defer backend.Close()
	w := &FileWorker{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := w.Work(ctx, &FileTask{})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("Get %q: context canceled", backend.URL), err.Error())
}
