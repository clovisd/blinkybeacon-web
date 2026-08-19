package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubDashboard serves a payload the test can change between polls.
type stubDashboard struct {
	mu      sync.Mutex
	body    string
	polls   int
	lastURL string
}

func newStubDashboard(body string) *stubDashboard { return &stubDashboard{body: body} }

func (s *stubDashboard) set(body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body = body
}

func (s *stubDashboard) pollCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.polls
}

func (s *stubDashboard) path() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastURL
}

func (s *stubDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.polls++
	s.lastURL = r.URL.Path
	body := s.body
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(body))
}

const pausedPayload = `{"paused":true,"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","seconds_since_gsi":0.6,"running":true}`
const livePayload = `{"paused":false,"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","seconds_since_gsi":0.6,"running":true}`
const draftPayload = `{"paused":false,"game_state":"DOTA_GAMERULES_STATE_HERO_SELECTION","seconds_since_gsi":0.6,"running":true}`

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestWatchLoop_spinsWhileTheDashboardSaysPaused(t *testing.T) {
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 2}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatchLoop(ctx, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})
	if app.WatchStatus() != WatchOK {
		t.Errorf("WatchStatus = %q, want ok", app.WatchStatus())
	}
	if got := stub.path(); got != "/line/2/state" {
		t.Errorf("polled %q, want /line/2/state", got)
	}

	// The pause lifts — the beacon must go dark without any transition
	// bookkeeping on our side.
	stub.set(livePayload)
	waitFor(t, "the beacon to stop", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle
	})
}

func TestWatchLoop_flashesWhenTheDraftEnds(t *testing.T) {
	stub := newStubDashboard(draftPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatchLoop(ctx, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the first poll", func() bool { return stub.pollCount() > 0 })
	stub.set(livePayload)

	waitFor(t, "the beacon to flash at draft end", func() bool {
		state, _, _ := app.Get()
		return state == StateFlash
	})
}

func TestWatchLoop_goesDarkWhenTheDashboardIsUnreachable(t *testing.T) {
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatchLoop(ctx, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})

	srv.Close() // the dashboard dies mid-pause

	waitFor(t, "the beacon to go dark", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchLost
	})
}

func TestWatchLoop_leavesTheBeaconAloneWhenNoDashboardIsConfigured(t *testing.T) {
	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)
	// The user drove the beacon by hand from the tray.
	applyState(app, StateSpin)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatchLoop(ctx, app, http.DefaultClient, func() Config { return Config{} }, 5*time.Millisecond)

	waitFor(t, "the watcher to report itself off", func() bool {
		return app.WatchStatus() == WatchOff
	})
	time.Sleep(30 * time.Millisecond)

	if state, _, _ := app.Get(); state != StateSpin {
		t.Errorf("state = %q, want spin — an unconfigured watcher must not stomp manual control", state)
	}
	if b.stops != 0 {
		t.Errorf("Stop called %d times, want 0", b.stops)
	}
}

func TestWatchLoop_stopsWhenTheContextIsCancelled(t *testing.T) {
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runWatchLoop(ctx, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)
		close(done)
	}()

	waitFor(t, "the first poll", func() bool { return stub.pollCount() > 0 })
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runWatchLoop did not return after its context was cancelled")
	}
}

func TestWatchLoop_switchingLineForgetsTheOldLinesDraft(t *testing.T) {
	// Line 1 is mid-draft; line 2 is already live. Retargeting must not read
	// "line 1 was drafting, line 2 is live" as a draft ending.
	// The two lines are served by path, so the ONLY thing the test changes is
	// the configured line number — no payload/config ordering race.
	var seen sync.Mutex
	paths := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Lock()
		paths[r.URL.Path] = true
		seen.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/line/1/state" {
			w.Write([]byte(draftPayload))
			return
		}
		w.Write([]byte(livePayload))
	}))
	defer srv.Close()

	polled := func(path string) bool {
		seen.Lock()
		defer seen.Unlock()
		return paths[path]
	}

	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)

	var mu sync.Mutex
	cfg := Config{DashboardURL: srv.URL, LineNumber: 1}
	get := func() Config {
		mu.Lock()
		defer mu.Unlock()
		return cfg
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runWatchLoop(ctx, app, srv.Client(), get, 5*time.Millisecond)

	waitFor(t, "the first poll of line 1", func() bool { return polled("/line/1/state") })

	mu.Lock()
	cfg.LineNumber = 2
	mu.Unlock()

	waitFor(t, "the watcher to follow the new line", func() bool { return polled("/line/2/state") })
	time.Sleep(40 * time.Millisecond)

	if b.flashes != 0 {
		t.Errorf("Flash called %d times, want 0 — the draft belonged to the old line", b.flashes)
	}
}

// withTempConfig points the config file at a disposable directory for the
// duration of one test.
func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := configDir
	configDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { configDir = orig })
}

// ------------------------------------------------------------------ config

func TestConfig_watcherDefaults(t *testing.T) {
	withTempConfig(t)
	cfg := loadConfig()
	if cfg.DashboardURL != "" {
		t.Errorf("DashboardURL = %q, want empty by default", cfg.DashboardURL)
	}
	if cfg.LineNumber != defaultLineNumber {
		t.Errorf("LineNumber = %d, want %d", cfg.LineNumber, defaultLineNumber)
	}
}

func TestConfig_watcherSettingsRoundTrip(t *testing.T) {
	withTempConfig(t)
	want := Config{Addr: "0.0.0.0", Port: 1337, DashboardURL: "http://192.168.1.50:8080", LineNumber: 3}
	if err := saveConfig(want); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	got := loadConfig()
	if got.DashboardURL != want.DashboardURL || got.LineNumber != want.LineNumber {
		t.Errorf("round trip gave %+v, want %+v", got, want)
	}
}

func TestSettingsForm_showsTheWatcherFields(t *testing.T) {
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "http://dash.local:8080", LineNumber: 4})

	h := &settingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	h.handleGet(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "http://dash.local:8080") {
		t.Error("settings form does not show the saved dashboard URL")
	}
	if !strings.Contains(body, `name="dashboard_url"`) {
		t.Error("settings form has no dashboard_url field")
	}
	if !strings.Contains(body, `name="line_number"`) {
		t.Error("settings form has no line_number field")
	}
	if !strings.Contains(body, `value="4"`) {
		t.Error("settings form does not show the saved line number")
	}
	// A format verb that lost its argument renders as %!s(MISSING) rather than
	// failing loudly, so check the page came out whole.
	if strings.Contains(body, "%!") {
		t.Errorf("settings form has a broken format verb:\n%s", body)
	}
}

func TestSettingsPost_savesTheWatcherFields(t *testing.T) {
	withTempConfig(t)

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {" http://192.168.1.50:8080 "},
		"line_number":   {"2"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handlePost(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	got := loadConfig()
	if got.DashboardURL != "http://192.168.1.50:8080" {
		t.Errorf("DashboardURL = %q, want the trimmed URL", got.DashboardURL)
	}
	if got.LineNumber != 2 {
		t.Errorf("LineNumber = %d, want 2", got.LineNumber)
	}
}

func TestSettingsPost_rejectsAnImpossibleLineNumber(t *testing.T) {
	withTempConfig(t)

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"http://dash.local"},
		"line_number":   {"0"},
	}
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handlePost(w, req)

	if got := loadConfig(); got.LineNumber != defaultLineNumber {
		t.Errorf("LineNumber = %d, want the default %d", got.LineNumber, defaultLineNumber)
	}
}
