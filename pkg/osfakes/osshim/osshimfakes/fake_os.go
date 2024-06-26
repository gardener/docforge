// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
// Code generated by counterfeiter. DO NOT EDIT.
package osshimfakes

import (
	"sync"

	"github.com/gardener/docforge/pkg/osfakes/osshim"
)

type FakeOs struct {
	IsDirStub        func(string) (bool, error)
	isDirMutex       sync.RWMutex
	isDirArgsForCall []struct {
		arg1 string
	}
	isDirReturns struct {
		result1 bool
		result2 error
	}
	isDirReturnsOnCall map[int]struct {
		result1 bool
		result2 error
	}
	IsNotExistStub        func(error) bool
	isNotExistMutex       sync.RWMutex
	isNotExistArgsForCall []struct {
		arg1 error
	}
	isNotExistReturns struct {
		result1 bool
	}
	isNotExistReturnsOnCall map[int]struct {
		result1 bool
	}
	ReadFileStub        func(string) ([]byte, error)
	readFileMutex       sync.RWMutex
	readFileArgsForCall []struct {
		arg1 string
	}
	readFileReturns struct {
		result1 []byte
		result2 error
	}
	readFileReturnsOnCall map[int]struct {
		result1 []byte
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeOs) IsDir(arg1 string) (bool, error) {
	fake.isDirMutex.Lock()
	ret, specificReturn := fake.isDirReturnsOnCall[len(fake.isDirArgsForCall)]
	fake.isDirArgsForCall = append(fake.isDirArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.IsDirStub
	fakeReturns := fake.isDirReturns
	fake.recordInvocation("IsDir", []interface{}{arg1})
	fake.isDirMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeOs) IsDirCallCount() int {
	fake.isDirMutex.RLock()
	defer fake.isDirMutex.RUnlock()
	return len(fake.isDirArgsForCall)
}

func (fake *FakeOs) IsDirCalls(stub func(string) (bool, error)) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = stub
}

func (fake *FakeOs) IsDirArgsForCall(i int) string {
	fake.isDirMutex.RLock()
	defer fake.isDirMutex.RUnlock()
	argsForCall := fake.isDirArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeOs) IsDirReturns(result1 bool, result2 error) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = nil
	fake.isDirReturns = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeOs) IsDirReturnsOnCall(i int, result1 bool, result2 error) {
	fake.isDirMutex.Lock()
	defer fake.isDirMutex.Unlock()
	fake.IsDirStub = nil
	if fake.isDirReturnsOnCall == nil {
		fake.isDirReturnsOnCall = make(map[int]struct {
			result1 bool
			result2 error
		})
	}
	fake.isDirReturnsOnCall[i] = struct {
		result1 bool
		result2 error
	}{result1, result2}
}

func (fake *FakeOs) IsNotExist(arg1 error) bool {
	fake.isNotExistMutex.Lock()
	ret, specificReturn := fake.isNotExistReturnsOnCall[len(fake.isNotExistArgsForCall)]
	fake.isNotExistArgsForCall = append(fake.isNotExistArgsForCall, struct {
		arg1 error
	}{arg1})
	stub := fake.IsNotExistStub
	fakeReturns := fake.isNotExistReturns
	fake.recordInvocation("IsNotExist", []interface{}{arg1})
	fake.isNotExistMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeOs) IsNotExistCallCount() int {
	fake.isNotExistMutex.RLock()
	defer fake.isNotExistMutex.RUnlock()
	return len(fake.isNotExistArgsForCall)
}

func (fake *FakeOs) IsNotExistCalls(stub func(error) bool) {
	fake.isNotExistMutex.Lock()
	defer fake.isNotExistMutex.Unlock()
	fake.IsNotExistStub = stub
}

func (fake *FakeOs) IsNotExistArgsForCall(i int) error {
	fake.isNotExistMutex.RLock()
	defer fake.isNotExistMutex.RUnlock()
	argsForCall := fake.isNotExistArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeOs) IsNotExistReturns(result1 bool) {
	fake.isNotExistMutex.Lock()
	defer fake.isNotExistMutex.Unlock()
	fake.IsNotExistStub = nil
	fake.isNotExistReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeOs) IsNotExistReturnsOnCall(i int, result1 bool) {
	fake.isNotExistMutex.Lock()
	defer fake.isNotExistMutex.Unlock()
	fake.IsNotExistStub = nil
	if fake.isNotExistReturnsOnCall == nil {
		fake.isNotExistReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.isNotExistReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakeOs) ReadFile(arg1 string) ([]byte, error) {
	fake.readFileMutex.Lock()
	ret, specificReturn := fake.readFileReturnsOnCall[len(fake.readFileArgsForCall)]
	fake.readFileArgsForCall = append(fake.readFileArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ReadFileStub
	fakeReturns := fake.readFileReturns
	fake.recordInvocation("ReadFile", []interface{}{arg1})
	fake.readFileMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeOs) ReadFileCallCount() int {
	fake.readFileMutex.RLock()
	defer fake.readFileMutex.RUnlock()
	return len(fake.readFileArgsForCall)
}

func (fake *FakeOs) ReadFileCalls(stub func(string) ([]byte, error)) {
	fake.readFileMutex.Lock()
	defer fake.readFileMutex.Unlock()
	fake.ReadFileStub = stub
}

func (fake *FakeOs) ReadFileArgsForCall(i int) string {
	fake.readFileMutex.RLock()
	defer fake.readFileMutex.RUnlock()
	argsForCall := fake.readFileArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeOs) ReadFileReturns(result1 []byte, result2 error) {
	fake.readFileMutex.Lock()
	defer fake.readFileMutex.Unlock()
	fake.ReadFileStub = nil
	fake.readFileReturns = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeOs) ReadFileReturnsOnCall(i int, result1 []byte, result2 error) {
	fake.readFileMutex.Lock()
	defer fake.readFileMutex.Unlock()
	fake.ReadFileStub = nil
	if fake.readFileReturnsOnCall == nil {
		fake.readFileReturnsOnCall = make(map[int]struct {
			result1 []byte
			result2 error
		})
	}
	fake.readFileReturnsOnCall[i] = struct {
		result1 []byte
		result2 error
	}{result1, result2}
}

func (fake *FakeOs) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.isDirMutex.RLock()
	defer fake.isDirMutex.RUnlock()
	fake.isNotExistMutex.RLock()
	defer fake.isNotExistMutex.RUnlock()
	fake.readFileMutex.RLock()
	defer fake.readFileMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeOs) recordInvocation(key string, args []interface{}) {
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

var _ osshim.Os = new(FakeOs)
