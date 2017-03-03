// This file was generated by counterfeiter
package fakes

import (
	"sync"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/route-emitter/routing_table"
	"code.cloudfoundry.org/route-emitter/watcher"
)

type FakeRouteHandler struct {
	HandleEventStub        func(event models.Event)
	handleEventMutex       sync.RWMutex
	handleEventArgsForCall []struct {
		event models.Event
	}
	SyncStub        func(desired []*models.DesiredLRPSchedulingInfo, runningActual []*routing_table.ActualLRPRoutingInfo, domains models.DomainSet)
	syncMutex       sync.RWMutex
	syncArgsForCall []struct {
		desired       []*models.DesiredLRPSchedulingInfo
		runningActual []*routing_table.ActualLRPRoutingInfo
		domains       models.DomainSet
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRouteHandler) HandleEvent(event models.Event) {
	fake.handleEventMutex.Lock()
	fake.handleEventArgsForCall = append(fake.handleEventArgsForCall, struct {
		event models.Event
	}{event})
	fake.recordInvocation("HandleEvent", []interface{}{event})
	fake.handleEventMutex.Unlock()
	if fake.HandleEventStub != nil {
		fake.HandleEventStub(event)
	}
}

func (fake *FakeRouteHandler) HandleEventCallCount() int {
	fake.handleEventMutex.RLock()
	defer fake.handleEventMutex.RUnlock()
	return len(fake.handleEventArgsForCall)
}

func (fake *FakeRouteHandler) HandleEventArgsForCall(i int) models.Event {
	fake.handleEventMutex.RLock()
	defer fake.handleEventMutex.RUnlock()
	return fake.handleEventArgsForCall[i].event
}

func (fake *FakeRouteHandler) Sync(desired []*models.DesiredLRPSchedulingInfo, runningActual []*routing_table.ActualLRPRoutingInfo, domains models.DomainSet) {
	var desiredCopy []*models.DesiredLRPSchedulingInfo
	if desired != nil {
		desiredCopy = make([]*models.DesiredLRPSchedulingInfo, len(desired))
		copy(desiredCopy, desired)
	}
	var runningActualCopy []*routing_table.ActualLRPRoutingInfo
	if runningActual != nil {
		runningActualCopy = make([]*routing_table.ActualLRPRoutingInfo, len(runningActual))
		copy(runningActualCopy, runningActual)
	}
	fake.syncMutex.Lock()
	fake.syncArgsForCall = append(fake.syncArgsForCall, struct {
		desired       []*models.DesiredLRPSchedulingInfo
		runningActual []*routing_table.ActualLRPRoutingInfo
		domains       models.DomainSet
	}{desiredCopy, runningActualCopy, domains})
	fake.recordInvocation("Sync", []interface{}{desiredCopy, runningActualCopy, domains})
	fake.syncMutex.Unlock()
	if fake.SyncStub != nil {
		fake.SyncStub(desired, runningActual, domains)
	}
}

func (fake *FakeRouteHandler) SyncCallCount() int {
	fake.syncMutex.RLock()
	defer fake.syncMutex.RUnlock()
	return len(fake.syncArgsForCall)
}

func (fake *FakeRouteHandler) SyncArgsForCall(i int) ([]*models.DesiredLRPSchedulingInfo, []*routing_table.ActualLRPRoutingInfo, models.DomainSet) {
	fake.syncMutex.RLock()
	defer fake.syncMutex.RUnlock()
	return fake.syncArgsForCall[i].desired, fake.syncArgsForCall[i].runningActual, fake.syncArgsForCall[i].domains
}

func (fake *FakeRouteHandler) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.handleEventMutex.RLock()
	defer fake.handleEventMutex.RUnlock()
	fake.syncMutex.RLock()
	defer fake.syncMutex.RUnlock()
	return fake.invocations
}

func (fake *FakeRouteHandler) recordInvocation(key string, args []interface{}) {
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

var _ watcher.RouteHandler = new(FakeRouteHandler)