// Code generated by counterfeiter. DO NOT EDIT.
package fakes

import (
	"sync"

	"github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
)

type FakeWindowsDiskProtector struct {
	CommandExistsStub        func() bool
	commandExistsMutex       sync.RWMutex
	commandExistsArgsForCall []struct {
	}
	commandExistsReturns struct {
		result1 bool
	}
	commandExistsReturnsOnCall map[int]struct {
		result1 bool
	}
	ProtectPathStub        func(string) error
	protectPathMutex       sync.RWMutex
	protectPathArgsForCall []struct {
		arg1 string
	}
	protectPathReturns struct {
		result1 error
	}
	protectPathReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeWindowsDiskProtector) CommandExists() bool {
	fake.commandExistsMutex.Lock()
	ret, specificReturn := fake.commandExistsReturnsOnCall[len(fake.commandExistsArgsForCall)]
	fake.commandExistsArgsForCall = append(fake.commandExistsArgsForCall, struct {
	}{})
	stub := fake.CommandExistsStub
	fakeReturns := fake.commandExistsReturns
	fake.recordInvocation("CommandExists", []interface{}{})
	fake.commandExistsMutex.Unlock()
	if stub != nil {
		return stub()
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeWindowsDiskProtector) CommandExistsCallCount() int {
	fake.commandExistsMutex.RLock()
	defer fake.commandExistsMutex.RUnlock()
	return len(fake.commandExistsArgsForCall)
}

func (fake *FakeWindowsDiskProtector) CommandExistsCalls(stub func() bool) {
	fake.commandExistsMutex.Lock()
	defer fake.commandExistsMutex.Unlock()
	fake.CommandExistsStub = stub
}

func (fake *FakeWindowsDiskProtector) CommandExistsReturns(result1 bool) {
	fake.commandExistsMutex.Lock()
	defer fake.commandExistsMutex.Unlock()
	fake.CommandExistsStub = nil
	fake.commandExistsReturns = struct {
		result1 bool
	}{result1}
}

func (fake *FakeWindowsDiskProtector) CommandExistsReturnsOnCall(i int, result1 bool) {
	fake.commandExistsMutex.Lock()
	defer fake.commandExistsMutex.Unlock()
	fake.CommandExistsStub = nil
	if fake.commandExistsReturnsOnCall == nil {
		fake.commandExistsReturnsOnCall = make(map[int]struct {
			result1 bool
		})
	}
	fake.commandExistsReturnsOnCall[i] = struct {
		result1 bool
	}{result1}
}

func (fake *FakeWindowsDiskProtector) ProtectPath(arg1 string) error {
	fake.protectPathMutex.Lock()
	ret, specificReturn := fake.protectPathReturnsOnCall[len(fake.protectPathArgsForCall)]
	fake.protectPathArgsForCall = append(fake.protectPathArgsForCall, struct {
		arg1 string
	}{arg1})
	stub := fake.ProtectPathStub
	fakeReturns := fake.protectPathReturns
	fake.recordInvocation("ProtectPath", []interface{}{arg1})
	fake.protectPathMutex.Unlock()
	if stub != nil {
		return stub(arg1)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeWindowsDiskProtector) ProtectPathCallCount() int {
	fake.protectPathMutex.RLock()
	defer fake.protectPathMutex.RUnlock()
	return len(fake.protectPathArgsForCall)
}

func (fake *FakeWindowsDiskProtector) ProtectPathCalls(stub func(string) error) {
	fake.protectPathMutex.Lock()
	defer fake.protectPathMutex.Unlock()
	fake.ProtectPathStub = stub
}

func (fake *FakeWindowsDiskProtector) ProtectPathArgsForCall(i int) string {
	fake.protectPathMutex.RLock()
	defer fake.protectPathMutex.RUnlock()
	argsForCall := fake.protectPathArgsForCall[i]
	return argsForCall.arg1
}

func (fake *FakeWindowsDiskProtector) ProtectPathReturns(result1 error) {
	fake.protectPathMutex.Lock()
	defer fake.protectPathMutex.Unlock()
	fake.ProtectPathStub = nil
	fake.protectPathReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeWindowsDiskProtector) ProtectPathReturnsOnCall(i int, result1 error) {
	fake.protectPathMutex.Lock()
	defer fake.protectPathMutex.Unlock()
	fake.ProtectPathStub = nil
	if fake.protectPathReturnsOnCall == nil {
		fake.protectPathReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.protectPathReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeWindowsDiskProtector) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.commandExistsMutex.RLock()
	defer fake.commandExistsMutex.RUnlock()
	fake.protectPathMutex.RLock()
	defer fake.protectPathMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeWindowsDiskProtector) recordInvocation(key string, args []interface{}) {
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

var _ disk.WindowsDiskProtector = new(FakeWindowsDiskProtector)
