package main

import (
	"sync"
	"sync/atomic"
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
	mu          sync.RWMutex
	state       StateValue
	connected   bool
	beacon      Beacon
	listenAddr  atomic.Value // stores string
	watchStatus atomic.Value // stores WatchStatus
	watchLine   atomic.Int64 // the line number the watcher is bound to
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

// SetListenAddr stores the current HTTP listen address (e.g. "127.0.0.1:1337").
func (a *AppState) SetListenAddr(addr string) {
	a.listenAddr.Store(addr)
}

// ListenAddr returns the current HTTP listen address.
func (a *AppState) ListenAddr() string {
	if v := a.listenAddr.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// SetWatchStatus records what the dashboard watcher currently believes about
// the feed, so the tray can say so.
func (a *AppState) SetWatchStatus(s WatchStatus) {
	a.watchStatus.Store(s)
}

// WatchStatus returns the watcher's feed status; WatchOff until it runs.
func (a *AppState) WatchStatus() WatchStatus {
	if v := a.watchStatus.Load(); v != nil {
		return v.(WatchStatus)
	}
	return WatchOff
}

// SetWatchLine records which line the watcher is currently bound to, so the
// tray can name it — "line 7 not found" beats "line not found".
func (a *AppState) SetWatchLine(n int) {
	a.watchLine.Store(int64(n))
}

// WatchLine returns the bound line number; 0 before anything is bound.
func (a *AppState) WatchLine() int {
	return int(a.watchLine.Load())
}
