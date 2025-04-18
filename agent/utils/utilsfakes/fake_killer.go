// Code generated by counterfeiter. DO NOT EDIT.
package utilsfakes

import (
	"sync"

	"github.com/cloudfoundry/bosh-agent/v2/agent/utils"
)

type FakeKiller struct {
	KillAgentStub        func()
	killAgentMutex       sync.RWMutex
	killAgentArgsForCall []struct {
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeKiller) KillAgent() {
	fake.killAgentMutex.Lock()
	fake.killAgentArgsForCall = append(fake.killAgentArgsForCall, struct {
	}{})
	stub := fake.KillAgentStub
	fake.recordInvocation("KillAgent", []interface{}{})
	fake.killAgentMutex.Unlock()
	if stub != nil {
		fake.KillAgentStub()
	}
}

func (fake *FakeKiller) KillAgentCallCount() int {
	fake.killAgentMutex.RLock()
	defer fake.killAgentMutex.RUnlock()
	return len(fake.killAgentArgsForCall)
}

func (fake *FakeKiller) KillAgentCalls(stub func()) {
	fake.killAgentMutex.Lock()
	defer fake.killAgentMutex.Unlock()
	fake.KillAgentStub = stub
}

func (fake *FakeKiller) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.killAgentMutex.RLock()
	defer fake.killAgentMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeKiller) recordInvocation(key string, args []interface{}) {
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

var _ utils.Killer = new(FakeKiller)
