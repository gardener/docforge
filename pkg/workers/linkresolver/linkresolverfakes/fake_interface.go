// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
// Code generated by counterfeiter. DO NOT EDIT.
package linkresolverfakes

import (
	"sync"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
)

type FakeInterface struct {
	ResolveLinkStub        func(string, *manifest.Node, string) (string, bool, error)
	resolveLinkMutex       sync.RWMutex
	resolveLinkArgsForCall []struct {
		arg1 string
		arg2 *manifest.Node
		arg3 string
	}
	resolveLinkReturns struct {
		result1 string
		result2 bool
		result3 error
	}
	resolveLinkReturnsOnCall map[int]struct {
		result1 string
		result2 bool
		result3 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeInterface) ResolveLink(arg1 string, arg2 *manifest.Node, arg3 string) (string, bool, error) {
	fake.resolveLinkMutex.Lock()
	ret, specificReturn := fake.resolveLinkReturnsOnCall[len(fake.resolveLinkArgsForCall)]
	fake.resolveLinkArgsForCall = append(fake.resolveLinkArgsForCall, struct {
		arg1 string
		arg2 *manifest.Node
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.ResolveLinkStub
	fakeReturns := fake.resolveLinkReturns
	fake.recordInvocation("ResolveLink", []interface{}{arg1, arg2, arg3})
	fake.resolveLinkMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2, ret.result3
	}
	return fakeReturns.result1, fakeReturns.result2, fakeReturns.result3
}

func (fake *FakeInterface) ResolveLinkCallCount() int {
	fake.resolveLinkMutex.RLock()
	defer fake.resolveLinkMutex.RUnlock()
	return len(fake.resolveLinkArgsForCall)
}

func (fake *FakeInterface) ResolveLinkCalls(stub func(string, *manifest.Node, string) (string, bool, error)) {
	fake.resolveLinkMutex.Lock()
	defer fake.resolveLinkMutex.Unlock()
	fake.ResolveLinkStub = stub
}

func (fake *FakeInterface) ResolveLinkArgsForCall(i int) (string, *manifest.Node, string) {
	fake.resolveLinkMutex.RLock()
	defer fake.resolveLinkMutex.RUnlock()
	argsForCall := fake.resolveLinkArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeInterface) ResolveLinkReturns(result1 string, result2 bool, result3 error) {
	fake.resolveLinkMutex.Lock()
	defer fake.resolveLinkMutex.Unlock()
	fake.ResolveLinkStub = nil
	fake.resolveLinkReturns = struct {
		result1 string
		result2 bool
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeInterface) ResolveLinkReturnsOnCall(i int, result1 string, result2 bool, result3 error) {
	fake.resolveLinkMutex.Lock()
	defer fake.resolveLinkMutex.Unlock()
	fake.ResolveLinkStub = nil
	if fake.resolveLinkReturnsOnCall == nil {
		fake.resolveLinkReturnsOnCall = make(map[int]struct {
			result1 string
			result2 bool
			result3 error
		})
	}
	fake.resolveLinkReturnsOnCall[i] = struct {
		result1 string
		result2 bool
		result3 error
	}{result1, result2, result3}
}

func (fake *FakeInterface) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.resolveLinkMutex.RLock()
	defer fake.resolveLinkMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeInterface) recordInvocation(key string, args []interface{}) {
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

var _ linkresolver.Interface = new(FakeInterface)
