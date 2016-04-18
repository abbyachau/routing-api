// This file was generated by counterfeiter
package fake_routing_api

import (
	"sync"

	"github.com/cloudfoundry-incubator/routing-api"
)

type FakeEventSource struct {
	NextStub        func() (routing_api.Event, error)
	nextMutex       sync.RWMutex
	nextArgsForCall []struct{}
	nextReturns     struct {
		result1 routing_api.Event
		result2 error
	}
	CloseStub        func() error
	closeMutex       sync.RWMutex
	closeArgsForCall []struct{}
	closeReturns     struct {
		result1 error
	}
}

func (fake *FakeEventSource) Next() (routing_api.Event, error) {
	fake.nextMutex.Lock()
	fake.nextArgsForCall = append(fake.nextArgsForCall, struct{}{})
	fake.nextMutex.Unlock()
	if fake.NextStub != nil {
		return fake.NextStub()
	} else {
		return fake.nextReturns.result1, fake.nextReturns.result2
	}
}

func (fake *FakeEventSource) NextCallCount() int {
	fake.nextMutex.RLock()
	defer fake.nextMutex.RUnlock()
	return len(fake.nextArgsForCall)
}

func (fake *FakeEventSource) NextReturns(result1 routing_api.Event, result2 error) {
	fake.NextStub = nil
	fake.nextReturns = struct {
		result1 routing_api.Event
		result2 error
	}{result1, result2}
}

func (fake *FakeEventSource) Close() error {
	fake.closeMutex.Lock()
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct{}{})
	fake.closeMutex.Unlock()
	if fake.CloseStub != nil {
		return fake.CloseStub()
	} else {
		return fake.closeReturns.result1
	}
}

func (fake *FakeEventSource) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakeEventSource) CloseReturns(result1 error) {
	fake.CloseStub = nil
	fake.closeReturns = struct {
		result1 error
	}{result1}
}

var _ routing_api.EventSource = new(FakeEventSource)
