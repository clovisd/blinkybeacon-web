package main

import (
	"sync"
	"testing"
)

func TestNewAppState_defaults(t *testing.T) {
	s := NewAppState()
	state, connected, beacon := s.Get()
	if state != StateIdle {
		t.Errorf("expected StateIdle, got %q", state)
	}
	if connected {
		t.Error("expected connected=false on new state")
	}
	if beacon != nil {
		t.Error("expected nil beacon on new state")
	}
}

func TestAppState_SetBeacon_nil_clears(t *testing.T) {
	s := NewAppState()
	s.SetBeacon(nil)
	_, connected, beacon := s.Get()
	if connected {
		t.Error("expected connected=false after SetBeacon(nil)")
	}
	if beacon != nil {
		t.Error("expected nil beacon after SetBeacon(nil)")
	}
}

func TestAppState_SetState(t *testing.T) {
	s := NewAppState()
	s.SetState(StateSpin)
	state, _, _ := s.Get()
	if state != StateSpin {
		t.Errorf("expected StateSpin, got %q", state)
	}
}

func TestAppState_concurrent(t *testing.T) {
	s := NewAppState()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); s.SetState(StateSpin) }()
		go func() { defer wg.Done(); s.Get() }()
	}
	wg.Wait()
}
