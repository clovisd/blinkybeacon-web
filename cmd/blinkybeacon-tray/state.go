package main

import (
	"sync"
)

type StateValue string

const (
	StateIdle  StateValue = "idle"
	StateSpin  StateValue = "spin"
	StateFlash StateValue = "flash"
)

// Beacon is a local mirror of pkg/fsbeacon.Beacon. Defined here to avoid
// importing the go-hid CGO dependency in non-Windows test builds.
// Keep in sync with pkg/fsbeacon.Beacon. See beacon_check.go for the guard.
type Beacon interface {
	Flash() error
	Spin() error
	Stop() error
	Close() error
}

// AppState is the single source of truth for beacon connection and mode.
// All fields are protected by a single RWMutex so Get/Set are atomic.
type AppState struct {
	mu        sync.RWMutex
	state     StateValue
	connected bool
	beacon    Beacon
}

func NewAppState() *AppState {
	return &AppState{state: StateIdle}
}

// Get returns a snapshot of the current state.
func (a *AppState) Get() (StateValue, bool, Beacon) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.state, a.connected, a.beacon
}

// SetBeacon stores a new beacon reference. Pass nil to mark as disconnected.
func (a *AppState) SetBeacon(b Beacon) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.beacon = b
	a.connected = b != nil
	if b == nil {
		a.state = StateIdle // always reset mode when disconnecting
	}
}

// SetState updates the beacon mode without changing the connection status.
func (a *AppState) SetState(s StateValue) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.state = s
}
