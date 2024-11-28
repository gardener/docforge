// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
// Code generated by counterfeiter. DO NOT EDIT.
package registryfakes

import (
	"context"
	"sync"

	"github.com/gardener/docforge/pkg/osfakes/httpclient"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
)

type FakeInterface struct {
	ClientStub        func(string) httpclient.Client
	clientMutex       sync.RWMutex
	clientArgsForCall []struct {
		arg1 string
	}
	clientReturns struct {
		result1 httpclient.Client
	}
	clientReturnsOnCall map[int]struct {
		result1 httpclient.Client
	}
	LoadRepositoryStub        func(context.Context, string) error
	loadRepositoryMutex       sync.RWMutex
	loadRepositoryArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	loadRepositoryReturns struct {
		result1 error
	}
	loadRepositoryReturnsOnCall map[int]struct {
		result1 error
	}
	LogRateLimitsStub        func(context.Context)
	logRateLimitsMutex       sync.RWMutex
	logRateLimitsArgsForCall []struct {
		arg1 context.Context
	}
	ReadStub        func(context.Context, string) ([]byte, error)
	readMutex       sync.RWMutex
	readArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	readReturns struct {
		result1 []byte
		result2 error
	}
	readReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	ReadGitInfoStub        func(context.Context, string) ([]byte, error)
	readGitInfoMutex       sync.RWMutex
	readGitInfoArgsForCall []struct {
		arg1 context.Context
		arg2 string
	}
	readGitInfoReturns struct {
		result1 []byte
		result2 error
	}
	readGitInfoReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	ResolveRelativeLinkStub        func(string, string) (string, error)
	resolveRelativeLinkMutex       sync.RWMutex
	resolveRelativeLinkArgsForCall []struct {
		arg1 string
		arg2 string
	}
	resolveRelativeLinkReturns struct {
		result1 string
		result2 error
	}
	resolveRelativeLinkReturnsOnCall map[int]struct {
		result1 string
		result2 error
	}
	ResourceURLStub        func(string) (*repositoryhost.URL, error)
	resourceURLMutex       sync.RWMutex
	resourceURLArgsForCall []struct {
		arg1 string
	}
	resourceURLReturns struct {
		result1 *repositoryhost.URL
		result2 error
	}
	resourceURLReturnsOnCall map[int]struct {
		result1 *repositoryhost.URL
		result2 error
	}
	TreeStub        func(string) ([]string, error)
	treeMutex       sync.RWMutex
	treeArgsForCall []struct {
		arg1 string
	}
	treeReturns struct {
		result1 []string
		result2 error
	}
	treeReturnsOnCall map[int]struct {
		result1 []string
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeInterface) Client(arg1 string) httpclient.Client {
	fake.clientMutex.Lock()
	ret, specificReturn := fake.clientReturnsOnCall[len(fake.clientArgsForCall)]
	fake.clientArgsForCall = append(fake.clientArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ClientStub
	fakeReturns := fake.clientReturns
	fake.recordInvocation("Client", []interface{}{arg1})
	fake.clientMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeInterface) ClientCallCount() int {
	fake.clientMutex.RLock()
	defer fake.clientMutex.RUnlock()
	return len(fake.clientArgsForCall)
}

func (fake *FakeInterface) ClientCalls(stub func(string) httpclient.Client) {
	fake.clientMutex.Lock()
	defer fake.clientMutex.Unlock()
	fake.ClientStub = stub
}

func (fake *FakeInterface) ClientArgsForCall(i int) string {
	fake.clientMutex.RLock()
	defer fake.clientMutex.RUnlock()
	argsForCall := fake.clientArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeInterface) ClientReturns(result1 httpclient.Client) {
	fake.clientMutex.Lock()
	defer fake.clientMutex.Unlock()
	fake.ClientStub = nil
	fake.clientReturns = struct {
		result1 httpclient.Client
	}{result1}
}

func (fake *FakeInterface) ClientReturnsOnCall(i int, result1 httpclient.Client) {
	fake.clientMutex.Lock()
	defer fake.clientMutex.Unlock()
	fake.ClientStub = nil
	if fake.clientReturnsOnCall == nil {
		fake.clientReturnsOnCall = make(map[int]struct {
			result1 httpclient.Client
		})
	}
	fake.clientReturnsOnCall[i] = struct {
		result1 httpclient.Client
	}{result1}
}

func (fake *FakeInterface) LoadRepository(arg1 context.Context, arg2 string) error {
	fake.loadRepositoryMutex.Lock()
	ret, specificReturn := fake.loadRepositoryReturnsOnCall[len(fake.loadRepositoryArgsForCall)]
	fake.loadRepositoryArgsForCall = append(fake.loadRepositoryArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.LoadRepositoryStub
	fakeReturns := fake.loadRepositoryReturns
	fake.recordInvocation("LoadRepository", []interface{}{arg1, arg2})
	fake.loadRepositoryMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeInterface) LoadRepositoryCallCount() int {
	fake.loadRepositoryMutex.RLock()
	defer fake.loadRepositoryMutex.RUnlock()
	return len(fake.loadRepositoryArgsForCall)
}

func (fake *FakeInterface) LoadRepositoryCalls(stub func(context.Context, string) error) {
	fake.loadRepositoryMutex.Lock()
	defer fake.loadRepositoryMutex.Unlock()
	fake.LoadRepositoryStub = stub
}

func (fake *FakeInterface) LoadRepositoryArgsForCall(i int) (context.Context, string) {
	fake.loadRepositoryMutex.RLock()
	defer fake.loadRepositoryMutex.RUnlock()
	argsForCall := fake.loadRepositoryArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeInterface) LoadRepositoryReturns(result1 error) {
	fake.loadRepositoryMutex.Lock()
	defer fake.loadRepositoryMutex.Unlock()
	fake.LoadRepositoryStub = nil
	fake.loadRepositoryReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeInterface) LoadRepositoryReturnsOnCall(i int, result1 error) {
	fake.loadRepositoryMutex.Lock()
	defer fake.loadRepositoryMutex.Unlock()
	fake.LoadRepositoryStub = nil
	if fake.loadRepositoryReturnsOnCall == nil {
		fake.loadRepositoryReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.loadRepositoryReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeInterface) LogRateLimits(arg1 context.Context) {
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

func (fake *FakeInterface) LogRateLimitsCallCount() int {
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	return len(fake.logRateLimitsArgsForCall)
}

func (fake *FakeInterface) LogRateLimitsCalls(stub func(context.Context)) {
	fake.logRateLimitsMutex.Lock()
	defer fake.logRateLimitsMutex.Unlock()
	fake.LogRateLimitsStub = stub
}

func (fake *FakeInterface) LogRateLimitsArgsForCall(i int) context.Context {
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	argsForCall := fake.logRateLimitsArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeInterface) Read(arg1 context.Context, arg2 string) ([]byte, error) {
	fake.readMutex.Lock()
	ret, specificReturn := fake.readReturnsOnCall[len(fake.readArgsForCall)]
	fake.readArgsForCall = append(fake.readArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.ReadStub
	fakeReturns := fake.readReturns
	fake.recordInvocation("Read", []interface{}{arg1, arg2})
	fake.readMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInterface) ReadCallCount() int {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	return len(fake.readArgsForCall)
}

func (fake *FakeInterface) ReadCalls(stub func(context.Context, string) ([]byte, error)) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = stub
}

func (fake *FakeInterface) ReadArgsForCall(i int) (context.Context, string) {
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	argsForCall := fake.readArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeInterface) ReadReturns(result1 []byte, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	fake.readReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ReadReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.readMutex.Lock()
	defer fake.readMutex.Unlock()
	fake.ReadStub = nil
	if fake.readReturnsOnCall == nil {
		fake.readReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.readReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ReadGitInfo(arg1 context.Context, arg2 string) ([]byte, error) {
	fake.readGitInfoMutex.Lock()
	ret, specificReturn := fake.readGitInfoReturnsOnCall[len(fake.readGitInfoArgsForCall)]
	fake.readGitInfoArgsForCall = append(fake.readGitInfoArgsForCall, struct {
		arg1 context.Context
		arg2 string
	}{arg1, arg2})
	stub := fake.ReadGitInfoStub
	fakeReturns := fake.readGitInfoReturns
	fake.recordInvocation("ReadGitInfo", []interface{}{arg1, arg2})
	fake.readGitInfoMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInterface) ReadGitInfoCallCount() int {
	fake.readGitInfoMutex.RLock()
	defer fake.readGitInfoMutex.RUnlock()
	return len(fake.readGitInfoArgsForCall)
}

func (fake *FakeInterface) ReadGitInfoCalls(stub func(context.Context, string) ([]byte, error)) {
	fake.readGitInfoMutex.Lock()
	defer fake.readGitInfoMutex.Unlock()
	fake.ReadGitInfoStub = stub
}

func (fake *FakeInterface) ReadGitInfoArgsForCall(i int) (context.Context, string) {
	fake.readGitInfoMutex.RLock()
	defer fake.readGitInfoMutex.RUnlock()
	argsForCall := fake.readGitInfoArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeInterface) ReadGitInfoReturns(result1 []byte, result2 error) {
	fake.readGitInfoMutex.Lock()
	defer fake.readGitInfoMutex.Unlock()
	fake.ReadGitInfoStub = nil
	fake.readGitInfoReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ReadGitInfoReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.readGitInfoMutex.Lock()
	defer fake.readGitInfoMutex.Unlock()
	fake.ReadGitInfoStub = nil
	if fake.readGitInfoReturnsOnCall == nil {
		fake.readGitInfoReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.readGitInfoReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ResolveRelativeLink(arg1 string, arg2 string) (string, error) {
	fake.resolveRelativeLinkMutex.Lock()
	ret, specificReturn := fake.resolveRelativeLinkReturnsOnCall[len(fake.resolveRelativeLinkArgsForCall)]
	fake.resolveRelativeLinkArgsForCall = append(fake.resolveRelativeLinkArgsForCall, struct {
		arg1 string
		arg2 string
	}{arg1, arg2})
	stub := fake.ResolveRelativeLinkStub
	fakeReturns := fake.resolveRelativeLinkReturns
	fake.recordInvocation("ResolveRelativeLink", []interface{}{arg1, arg2})
	fake.resolveRelativeLinkMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInterface) ResolveRelativeLinkCallCount() int {
	fake.resolveRelativeLinkMutex.RLock()
	defer fake.resolveRelativeLinkMutex.RUnlock()
	return len(fake.resolveRelativeLinkArgsForCall)
}

func (fake *FakeInterface) ResolveRelativeLinkCalls(stub func(string, string) (string, error)) {
	fake.resolveRelativeLinkMutex.Lock()
	defer fake.resolveRelativeLinkMutex.Unlock()
	fake.ResolveRelativeLinkStub = stub
}

func (fake *FakeInterface) ResolveRelativeLinkArgsForCall(i int) (string, string) {
	fake.resolveRelativeLinkMutex.RLock()
	defer fake.resolveRelativeLinkMutex.RUnlock()
	argsForCall := fake.resolveRelativeLinkArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2
}

func (fake *FakeInterface) ResolveRelativeLinkReturns(result1 string, result2 error) {
	fake.resolveRelativeLinkMutex.Lock()
	defer fake.resolveRelativeLinkMutex.Unlock()
	fake.ResolveRelativeLinkStub = nil
	fake.resolveRelativeLinkReturns = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ResolveRelativeLinkReturnsOnCall(i int, result1 string, result2 error) {
	fake.resolveRelativeLinkMutex.Lock()
	defer fake.resolveRelativeLinkMutex.Unlock()
	fake.ResolveRelativeLinkStub = nil
	if fake.resolveRelativeLinkReturnsOnCall == nil {
		fake.resolveRelativeLinkReturnsOnCall = make(map[int]struct {
			result1 string
			result2 error
		})
	}
	fake.resolveRelativeLinkReturnsOnCall[i] = struct {
		result1 string
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ResourceURL(arg1 string) (*repositoryhost.URL, error) {
	fake.resourceURLMutex.Lock()
	ret, specificReturn := fake.resourceURLReturnsOnCall[len(fake.resourceURLArgsForCall)]
	fake.resourceURLArgsForCall = append(fake.resourceURLArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ResourceURLStub
	fakeReturns := fake.resourceURLReturns
	fake.recordInvocation("ResourceURL", []interface{}{arg1})
	fake.resourceURLMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInterface) ResourceURLCallCount() int {
	fake.resourceURLMutex.RLock()
	defer fake.resourceURLMutex.RUnlock()
	return len(fake.resourceURLArgsForCall)
}

func (fake *FakeInterface) ResourceURLCalls(stub func(string) (*repositoryhost.URL, error)) {
	fake.resourceURLMutex.Lock()
	defer fake.resourceURLMutex.Unlock()
	fake.ResourceURLStub = stub
}

func (fake *FakeInterface) ResourceURLArgsForCall(i int) string {
	fake.resourceURLMutex.RLock()
	defer fake.resourceURLMutex.RUnlock()
	argsForCall := fake.resourceURLArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeInterface) ResourceURLReturns(result1 *repositoryhost.URL, result2 error) {
	fake.resourceURLMutex.Lock()
	defer fake.resourceURLMutex.Unlock()
	fake.ResourceURLStub = nil
	fake.resourceURLReturns = struct {
		result1 *repositoryhost.URL
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) ResourceURLReturnsOnCall(i int, result1 *repositoryhost.URL, result2 error) {
	fake.resourceURLMutex.Lock()
	defer fake.resourceURLMutex.Unlock()
	fake.ResourceURLStub = nil
	if fake.resourceURLReturnsOnCall == nil {
		fake.resourceURLReturnsOnCall = make(map[int]struct {
			result1 *repositoryhost.URL
			result2 error
		})
	}
	fake.resourceURLReturnsOnCall[i] = struct {
		result1 *repositoryhost.URL
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) Tree(arg1 string) ([]string, error) {
	fake.treeMutex.Lock()
	ret, specificReturn := fake.treeReturnsOnCall[len(fake.treeArgsForCall)]
	fake.treeArgsForCall = append(fake.treeArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.TreeStub
	fakeReturns := fake.treeReturns
	fake.recordInvocation("Tree", []interface{}{arg1})
	fake.treeMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeInterface) TreeCallCount() int {
	fake.treeMutex.RLock()
	defer fake.treeMutex.RUnlock()
	return len(fake.treeArgsForCall)
}

func (fake *FakeInterface) TreeCalls(stub func(string) ([]string, error)) {
	fake.treeMutex.Lock()
	defer fake.treeMutex.Unlock()
	fake.TreeStub = stub
}

func (fake *FakeInterface) TreeArgsForCall(i int) string {
	fake.treeMutex.RLock()
	defer fake.treeMutex.RUnlock()
	argsForCall := fake.treeArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeInterface) TreeReturns(result1 []string, result2 error) {
	fake.treeMutex.Lock()
	defer fake.treeMutex.Unlock()
	fake.TreeStub = nil
	fake.treeReturns = struct {
		result1 []string
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) TreeReturnsOnCall(i int, result1 []string, result2 error) {
	fake.treeMutex.Lock()
	defer fake.treeMutex.Unlock()
	fake.TreeStub = nil
	if fake.treeReturnsOnCall == nil {
		fake.treeReturnsOnCall = make(map[int]struct {
			result1 []string
			result2 error
		})
	}
	fake.treeReturnsOnCall[i] = struct {
		result1 []string
		result2 error
	}{result1, result2}
}

func (fake *FakeInterface) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.clientMutex.RLock()
	defer fake.clientMutex.RUnlock()
	fake.loadRepositoryMutex.RLock()
	defer fake.loadRepositoryMutex.RUnlock()
	fake.logRateLimitsMutex.RLock()
	defer fake.logRateLimitsMutex.RUnlock()
	fake.readMutex.RLock()
	defer fake.readMutex.RUnlock()
	fake.readGitInfoMutex.RLock()
	defer fake.readGitInfoMutex.RUnlock()
	fake.resolveRelativeLinkMutex.RLock()
	defer fake.resolveRelativeLinkMutex.RUnlock()
	fake.resourceURLMutex.RLock()
	defer fake.resourceURLMutex.RUnlock()
	fake.treeMutex.RLock()
	defer fake.treeMutex.RUnlock()
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

var _ registry.Interface = new(FakeInterface)