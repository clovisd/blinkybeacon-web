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
	watchDetail atomic.Value // stores WatchDetail
	watchLine   atomic.Int64 // the line number the watcher is bound to
	watchLabel  atomic.Value // stores boundLabel
}

// boundLabel is the dashboard's own name for a line, carried together with the
// line it names. The two move as one so a name can never outlive the binding it
// came from: retarget to another line and the name goes quiet until that line's
// first poll answers, rather than sitting over the wrong line.
type boundLabel struct {
	line  int
	label string
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

// SetWatchDetail records the extra facts the tray's words need beyond the
// status value — how quiet the feed has gone, and whether any draft data is
// coming at all.
func (a *AppState) SetWatchDetail(d WatchDetail) {
	a.watchDetail.Store(d)
}

// WatchDetail returns those facts; the zero detail until the watcher runs.
func (a *AppState) WatchDetail() WatchDetail {
	if v := a.watchDetail.Load(); v != nil {
		return v.(WatchDetail)
	}
	return WatchDetail{}
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

// SetWatchLabel records what the dashboard calls a line, from a poll that
// actually answered. Sticky on purpose: the label is the last one we were told,
// so a single failed poll does not blank the tray's row — the status row beside
// it is what reports the failure.
func (a *AppState) SetWatchLabel(line int, label string) {
	a.watchLabel.Store(boundLabel{line: line, label: label})
}

// WatchLabel returns the bound line's name, or "" when we have not been told it
// — which includes having been told it for some OTHER line.
func (a *AppState) WatchLabel() string {
	v, ok := a.watchLabel.Load().(boundLabel)
	if !ok || v.line != a.WatchLine() {
		return ""
	}
	return v.label
}
