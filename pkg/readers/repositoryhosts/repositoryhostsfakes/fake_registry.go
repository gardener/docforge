// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
// Code generated by counterfeiter. DO NOT EDIT.
package repositoryhostsfakes

import (
	"context"
	"sync"

	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
)

type FakeRegistry struct {
	GetStub        func(string) (repositoryhosts.RepositoryHost, error)
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 string
	}
	getReturns struct {
		result1 repositoryhosts.RepositoryHost
		result2 error
	}
	getReturnsOnCall map[int]struct {
		result1 repositoryhosts.RepositoryHost
		result2 error
	}
	LogRateLimitsStub        func(context.Context)
	logRateLimitsMutex       sync.RWMutex
	logRateLimitsArgsForCall []struct {
		arg1 context.Context
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRegistry) Get(arg1 string) (repositoryhosts.RepositoryHost, error) {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.GetStub
	fakeReturns := fake.getReturns
	fake.recordInvocation("Get", []interface{}{arg1})
	fake.getMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRegistry) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakeRegistry) GetCalls(stub func(string) (repositoryhosts.RepositoryHost, error)) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakeRegistry) GetArgsForCall(i int) string {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeRegistry) GetReturns(result1 repositoryhosts.RepositoryHost, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 repositoryhosts.RepositoryHost
		result2 error
	}{result1, result2}
}

func (fake *FakeRegistry) GetReturnsOnCall(i int, result1 repositoryhosts.RepositoryHost, result2 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 repositoryhosts.RepositoryHost
			result2 error
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 repositoryhosts.RepositoryHost
		result2 error
	}{result1, result2}
}

func (fake *FakeRegistry) LogRateLimits(arg1 context.Context) {
	fake.logRateLimitsMutex.Lock()
	fake.logRateLimitsArgsForCall = append(fake.logRateLimitsArgsForCall, struct {
		arg1 context.Context
	}{arg1})
	stub := fake.LogRateLimitsStub
	fake.recordInvocation("LogRateLimits", []interface{}{arg1})
	fake.logRateLimitsMutex.Unlock()
	if stub != nil {
		fake.LogRateLimitsStub(arg1)
	}
}

func (fake *FakeRegistry) LogRateLimitsCallCount() int {
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	return len(fake.logRateLimitsArgsForCall)
}

func (fake *FakeRegistry) LogRateLimitsCalls(stub func(context.Context)) {
	fake.logRateLimitsMutex.Lock()
	defer fake.logRateLimitsMutex.Unlock()
	fake.LogRateLimitsStub = stub
}

func (fake *FakeRegistry) LogRateLimitsArgsForCall(i int) context.Context {
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	argsForCall := fake.logRateLimitsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeRegistry) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRegistry) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ repositoryhosts.Registry = new(FakeRegistry)
