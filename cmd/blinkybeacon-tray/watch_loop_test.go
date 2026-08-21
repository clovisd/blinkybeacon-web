package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubDashboard stands in for the dashboard's GET /api/v0/lines/{n}. The test
// can change the status and payload between polls, and read back what was sent.
type stubDashboard struct {
	mu       sync.Mutex
	status   int
	body     string
	polls    int
	lastURL  string
	lastAuth string
}

func newStubDashboard(body string) *stubDashboard {
	return &stubDashboard{status: http.StatusOK, body: body}
}

func (s *stubDashboard) set(body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = http.StatusOK, body
}

// answer makes every later poll get this status and body — a token being
// revoked, or a line disappearing, without restarting anything.
func (s *stubDashboard) answer(status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.body = status, body
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

func (s *stubDashboard) auth() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastAuth
}

func (s *stubDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.polls++
	s.lastURL = r.URL.Path
	s.lastAuth = r.Header.Get("Authorization")
	status, body := s.status, s.body
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write([]byte(body))
}

// The v0 projection, at the three levels the state machine cares about.
const pausedPayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567","game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":true,"seconds_since_gsi":0.6,"ts":1765500000}`
const livePayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567","game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":false,"seconds_since_gsi":0.6,"ts":1765500000}`

// draftPayload is a PRE-v3.99.0 dashboard mid-draft: eight fields, no
// draft_complete key at all. Kept exactly as it was, because "the operator is
// running an old dashboard" is now a case the tray has to handle out loud.
const draftPayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":null,"game_state":"DOTA_GAMERULES_STATE_HERO_SELECTION","paused":false,"seconds_since_gsi":0.6,"ts":1765500000}`

// draftingPayload and draftDonePayload are the two sides of the one edge that
// arms the flash: a draft block seen with picks outstanding, then the same
// match with all ten pick slots filled.
const draftingPayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567","game_state":"DOTA_GAMERULES_STATE_HERO_SELECTION","paused":false,"seconds_since_gsi":0.6,"ts":1765500000,"draft_complete":false}`
const draftDonePayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567","game_state":"DOTA_GAMERULES_STATE_HERO_SELECTION","paused":false,"seconds_since_gsi":0.6,"ts":1765500000,"draft_complete":true}`

// quietPayload is a line that WAS flowing and has stopped: running, with a
// seconds_since_gsi well past the staleness bound.
const quietPayload = `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567","game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":false,"seconds_since_gsi":120.4,"ts":1765500000,"draft_complete":true}`

// testToken is what every loop test binds with. The read API has no anonymous
// tier, so a Config without one polls nothing at all.
const testToken = "test-bearer-token"

// withRejectedBackoff shortens the 401 backoff for one test, so recovery can be
// exercised without a 30-second wait.
func withRejectedBackoff(t *testing.T, d time.Duration) {
	t.Helper()
	orig := rejectedBackoff
	rejectedBackoff = d
	t.Cleanup(func() { rejectedBackoff = orig })
}

// submitSettings drives the two steps a browser actually takes: GET the form to
// obtain its CSRF token, then POST with it. Anything that can skip the GET is,
// by definition, not the operator's browser.
func submitSettings(t *testing.T, h *settingsHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	get := httptest.NewRecorder()
	h.handleGet(get, httptest.NewRequest(http.MethodGet, "/settings", nil))
	m := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(get.Body.String())
	if m == nil {
		t.Fatalf("settings form carries no CSRF token:\n%s", get.Body.String())
	}
	form.Set("csrf", m[1])
	return postSettings(t, h, form)
}

// postSettings submits exactly the values given — no CSRF token is added, so a
// test can forge one, omit one, or send a stale one.
func postSettings(t *testing.T, h *settingsHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handlePost(w, req)
	return w
}

// startWatchLoop runs the watch loop for the duration of one test and JOINS the
// goroutine before the test's earlier cleanups run. Without the join a finished
// test's loop keeps polling — and keeps reading rejectedBackoff — while the next
// test is already restoring it, which is a real data race and not a test
// artefact: it is the same read the shipped loop does on every tick.
func startWatchLoop(t *testing.T, app *AppState, client *http.Client, cfg func() Config, interval time.Duration) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runWatchLoop(ctx, app, client, cfg, interval)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("runWatchLoop did not stop when its context was cancelled")
		}
	})
}

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

	cfg := Config{DashboardURL: srv.URL, LineNumber: 2, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})
	if app.WatchStatus() != WatchOK {
		t.Errorf("WatchStatus = %q, want ok", app.WatchStatus())
	}
	if got := stub.path(); got != "/api/v0/lines/2" {
		t.Errorf("polled %q, want /api/v0/lines/2", got)
	}

	// The pause lifts — the beacon must go dark without any transition
	// bookkeeping on our side.
	stub.set(livePayload)
	waitFor(t, "the beacon to stop", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle
	})
}

func TestWatchLoop_flashesWhenTheLastPickLands(t *testing.T) {
	stub := newStubDashboard(draftingPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the first poll", func() bool { return stub.pollCount() > 0 })
	stub.set(draftDonePayload)

	waitFor(t, "the beacon to flash at the last final pick", func() bool {
		state, _, _ := app.Get()
		return state == StateFlash
	})
}

func TestWatchLoop_flashesWhenHeroSelectionEndsOnAnUnreinstalledCfg(t *testing.T) {
	// The fallback, end to end: a dashboard that sends no draft_complete at all
	// — the cfg every existing install already has — still drives the beacon at
	// the end of the pick/ban phase, off the game state alone.
	stub := newStubDashboard(draftPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the first poll", func() bool { return stub.pollCount() > 0 })
	stub.set(livePayload) // hero selection ends

	waitFor(t, "the beacon to flash at the end of the pick/ban phase", func() bool {
		state, _, _ := app.Get()
		return state == StateFlash
	})
}

func TestWatchLoop_tellsTheTrayWhenNoDraftDataIsComing(t *testing.T) {
	// An old dashboard publishes no draft_complete at all. The operator has to
	// be told, before the draft, that the beacon cannot flash at it.
	stub := newStubDashboard(draftPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the tray to be warned about the missing draft data", func() bool {
		return app.WatchStatus() == WatchOK && app.WatchDetail().NoDraftData
	})

	// And it stops warning the moment a v3.99.0 dashboard answers.
	stub.set(draftingPayload)
	waitFor(t, "the warning to clear once draft data arrives", func() bool {
		return app.WatchStatus() == WatchOK && !app.WatchDetail().NoDraftData
	})
}

func TestWatchLoop_callsAQuietFeedQuietAndNotUnreachable(t *testing.T) {
	// The dashboard is answering perfectly well; it is Dota that went silent.
	// Reporting that as "unreachable" sends the operator to the wrong machine.
	stub := newStubDashboard(quietPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the feed to be called quiet", func() bool {
		return app.WatchStatus() == WatchQuiet
	})
	if got := app.WatchDetail().QuietSeconds; got != 120.4 {
		t.Errorf("QuietSeconds = %v, want 120.4 so the tray can say how long", got)
	}
	if state, _, _ := app.Get(); state != StateIdle {
		t.Errorf("state = %q, want idle — the words change, never the light", state)
	}
}

func TestWatchLoop_goesDarkWhenTheDashboardIsUnreachable(t *testing.T) {
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

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

	startWatchLoop(t, app, http.DefaultClient, func() Config { return Config{} }, 5*time.Millisecond)

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

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
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
	// Line 1 has picks outstanding; line 2's draft is already finished.
	// Retargeting must not read "line 1 was false, line 2 is true" as an edge.
	// The two lines are served by path, so the ONLY thing the test changes is
	// the configured line number — no payload/config ordering race.
	var seen sync.Mutex
	paths := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Lock()
		paths[r.URL.Path] = true
		seen.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v0/lines/1" {
			w.Write([]byte(draftingPayload))
			return
		}
		w.Write([]byte(draftDonePayload))
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
	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	get := func() Config {
		mu.Lock()
		defer mu.Unlock()
		return cfg
	}

	startWatchLoop(t, app, srv.Client(), get, 5*time.Millisecond)

	waitFor(t, "the first poll of line 1", func() bool { return polled("/api/v0/lines/1") })

	mu.Lock()
	cfg.LineNumber = 2
	mu.Unlock()

	waitFor(t, "the watcher to follow the new line", func() bool { return polled("/api/v0/lines/2") })
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
	w := submitSettings(t, h, form)

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
	submitSettings(t, h, form)

	if got := loadConfig(); got.LineNumber != defaultLineNumber {
		t.Errorf("LineNumber = %d, want the default %d", got.LineNumber, defaultLineNumber)
	}
}

func TestSettingsForm_escapesTheSavedValues(t *testing.T) {
	// /settings has no auth and can be bound to 0.0.0.0, so the stored values
	// are attacker-reachable. They land inside an HTML attribute.
	withTempConfig(t)
	const inject = `" onfocus="alert(1)` + `x`
	saveConfig(Config{Addr: inject, Port: defaultPort, DashboardURL: inject, LineNumber: 1})

	h := &settingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	h.handleGet(w, req)

	if strings.Contains(w.Body.String(), `onfocus="alert(1)`) {
		t.Errorf("settings form reflected an unescaped attribute break:\n%s", w.Body.String())
	}
}

func TestSettingsSavedPage_escapesTheBindAddress(t *testing.T) {
	withTempConfig(t)

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {`" onfocus="alert(1)`},
		"port":          {"1337"},
		"dashboard_url": {"http://dash.local"},
		"line_number":   {"1"},
	}
	w := submitSettings(t, h, form)

	if strings.Contains(w.Body.String(), `onfocus="alert(1)`) {
		t.Errorf("saved page reflected an unescaped attribute break:\n%s", w.Body.String())
	}
}

// ------------------------------------------- the token, and its failures

func TestWatchLoop_sendsTheConfiguredTokenOnEveryPoll(t *testing.T) {
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the first poll", func() bool { return stub.pollCount() > 0 })
	if got := stub.auth(); got != "Bearer "+testToken {
		t.Errorf("Authorization = %q, want %q", got, "Bearer "+testToken)
	}
}

func TestWatchLoop_doesNotPollAtAllWithoutAToken(t *testing.T) {
	// The read API has no anonymous tier. Polling without a token can only
	// produce 401s, so the watcher does not make the request at all — and says
	// exactly why, rather than reporting a feed it never asked for as "lost".
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)
	applyState(app, StateSpin) // driven by hand from the tray

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1} // no token
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the watcher to report a missing token", func() bool {
		return app.WatchStatus() == WatchNoToken
	})
	time.Sleep(40 * time.Millisecond)

	if n := stub.pollCount(); n != 0 {
		t.Errorf("dashboard was polled %d times without a token, want 0", n)
	}
	// Not bound means not driving: manual control stays the operator's.
	if state, _, _ := app.Get(); state != StateSpin {
		t.Errorf("state = %q, want spin — an unbound watcher must not stomp manual control", state)
	}
}

func TestWatchLoop_treatsAWhitespaceOnlyTokenAsNoToken(t *testing.T) {
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: "   "}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the watcher to report a missing token", func() bool {
		return app.WatchStatus() == WatchNoToken
	})
	if n := stub.pollCount(); n != 0 {
		t.Errorf("dashboard was polled %d times with a blank token, want 0", n)
	}
}

func TestWatchLoop_goesDarkAndSaysSoWhenTheTokenIsRejected(t *testing.T) {
	// A token revoked mid-pause: the last thing we saw was "paused", and the
	// beacon must NOT keep spinning on it.
	withRejectedBackoff(t, 5*time.Millisecond)
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})

	stub.answer(http.StatusUnauthorized, `{"v":0,"error":"unauthorized"}`)

	waitFor(t, "the beacon to go dark with a named cause", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchRejected
	})
}

func TestWatchLoop_stopsHammeringADashboardThatRejectsTheToken(t *testing.T) {
	// A revoked token is a standing condition. Retrying it at the poll cadence
	// would point a permanent stream of 401s at an internet-facing origin.
	withRejectedBackoff(t, time.Hour)
	stub := newStubDashboard(`{"v":0,"error":"unauthorized"}`)
	stub.answer(http.StatusUnauthorized, `{"v":0,"error":"unauthorized"}`)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the token to be rejected", func() bool {
		return app.WatchStatus() == WatchRejected
	})
	// At the 5ms poll interval this window is ~40 polls' worth of time.
	time.Sleep(200 * time.Millisecond)

	if n := stub.pollCount(); n > 1 {
		t.Errorf("dashboard was polled %d times after a 401, want 1 — the watcher must back off", n)
	}
}

func TestWatchLoop_recoversWhenTheTokenStartsWorkingAgain(t *testing.T) {
	// A new token is pasted into settings. No restart: the next poll after the
	// backoff sees a 200 and the beacon resumes from the level it finds.
	withRejectedBackoff(t, 20*time.Millisecond)
	stub := newStubDashboard(livePayload)
	stub.answer(http.StatusUnauthorized, `{"v":0,"error":"unauthorized"}`)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the token to be rejected", func() bool {
		return app.WatchStatus() == WatchRejected
	})

	stub.answer(http.StatusOK, pausedPayload)

	waitFor(t, "the watcher to recover onto a live pause", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin && app.WatchStatus() == WatchOK
	})
}

func TestWatchLoop_goesDarkAndNamesTheLineWhenItIsNotFound(t *testing.T) {
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 7, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})

	stub.answer(http.StatusNotFound, `{"v":0,"error":"not found"}`)

	waitFor(t, "the beacon to go dark with a named cause", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchNoLine
	})
	// The tray needs the number to put in the label.
	if got := app.WatchLine(); got != 7 {
		t.Errorf("WatchLine = %d, want 7", got)
	}
}

func TestWatchLoop_keepsPollingAfterAMissingLineAndRecovers(t *testing.T) {
	// Unlike a rejected token, a 404 is cheap and can be fixed on the dashboard
	// side without touching the tray — so it keeps polling at the normal rate.
	stub := newStubDashboard(livePayload)
	stub.answer(http.StatusNotFound, `{"v":0,"error":"not found"}`)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 4, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the line to be reported missing", func() bool {
		return app.WatchStatus() == WatchNoLine
	})
	before := stub.pollCount()
	waitFor(t, "polling to continue", func() bool { return stub.pollCount() > before+1 })

	stub.answer(http.StatusOK, pausedPayload)
	waitFor(t, "the watcher to recover onto a live pause", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin && app.WatchStatus() == WatchOK
	})
}

func TestWatchLoop_neverWritesTheTokenToTheLog(t *testing.T) {
	// Every named failure gets logged. The credential must not ride along into
	// a log file the operator may well paste into a bug report.
	const secret = "sk-live-do-not-log-me"
	var logs strings.Builder
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(origOut); log.SetFlags(origFlags) })

	withRejectedBackoff(t, 5*time.Millisecond)
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: secret}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "a healthy poll", func() bool { return app.WatchStatus() == WatchOK })
	// Now walk it through every logged failure.
	stub.answer(http.StatusUnauthorized, `{"v":0,"error":"unauthorized"}`)
	waitFor(t, "the token to be rejected", func() bool { return app.WatchStatus() == WatchRejected })
	stub.answer(http.StatusNotFound, `{"v":0,"error":"not found"}`)
	waitFor(t, "the line to be reported missing", func() bool { return app.WatchStatus() == WatchNoLine })
	srv.Close()
	waitFor(t, "the feed to be lost", func() bool { return app.WatchStatus() == WatchLost })

	if strings.Contains(logs.String(), secret) {
		t.Errorf("the token was written to the log:\n%s", logs.String())
	}
	if logs.Len() == 0 {
		t.Fatal("nothing was logged at all — this test would pass vacuously")
	}
}

func TestWatchLoop_publishesTheBoundLineForTheTray(t *testing.T) {
	stub := newStubDashboard(livePayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 5, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the bound line to be published", func() bool { return app.WatchLine() == 5 })
}

// ------------------------------------------------- the token in settings

func TestConfig_tokenRoundTrips(t *testing.T) {
	withTempConfig(t)
	want := Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 3, APIToken: "minted-token"}
	if err := saveConfig(want); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	if got := loadConfig(); got.APIToken != want.APIToken {
		t.Errorf("APIToken = %q, want %q", got.APIToken, want.APIToken)
	}
}

func TestConfig_tokenIsStoredUnderTheTokenKey(t *testing.T) {
	// The config file is documented and hand-editable; the key name is part of
	// the contract with anyone who opens it.
	withTempConfig(t)
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the config: %v", err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatalf("config is not JSON: %v", err)
	}
	if onDisk["token"] != "minted-token" {
		t.Errorf("config file has %v under \"token\", want the saved token", onDisk["token"])
	}
}

func TestConfig_noTokenByDefault(t *testing.T) {
	withTempConfig(t)
	if got := loadConfig(); got.APIToken != "" {
		t.Errorf("APIToken = %q, want empty by default", got.APIToken)
	}
}

func TestSettingsForm_hasAMaskedTokenField(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	h.handleGet(w, req)

	body := w.Body.String()
	if !strings.Contains(body, `name="token"`) {
		t.Error("settings form has no token field")
	}
	if !strings.Contains(body, `id="token" name="token" type="password"`) {
		t.Errorf("the token field is not masked:\n%s", body)
	}
	if strings.Contains(body, "%!") {
		t.Errorf("settings form has a broken format verb:\n%s", body)
	}
}

func TestSettingsForm_neverEchoesTheSavedToken(t *testing.T) {
	// /settings has no auth and can be bound to 0.0.0.0. Rendering the token
	// into the page — even inside a password input, whose masking is only ever
	// visual — would hand the dashboard credential to anyone who can curl it.
	withTempConfig(t)
	const token = "sk-live-must-not-be-rendered"
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "http://dash.local", LineNumber: 1, APIToken: token})

	h := &settingsHandler{}
	req := httptest.NewRequest(http.MethodGet, "/settings", nil)
	w := httptest.NewRecorder()
	h.handleGet(w, req)

	if strings.Contains(w.Body.String(), token) {
		t.Errorf("settings form echoed the saved token back:\n%s", w.Body.String())
	}
}

func TestSettingsForm_saysWhetherATokenIsSaved(t *testing.T) {
	// Not echoing it costs the operator the ability to see it. Saying whether
	// one exists is the part they actually need.
	withTempConfig(t)
	h := &settingsHandler{}

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1})
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	empty := w.Body.String()

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"})
	w = httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	saved := w.Body.String()

	if empty == saved {
		t.Error("the settings page looks identical with and without a saved token")
	}
	if !strings.Contains(saved, "saved") {
		t.Errorf("the settings page does not say a token is saved:\n%s", saved)
	}
}

func TestSettingsForm_escapesATokenSaveNotice(t *testing.T) {
	// Whatever the page says about the stored token, no part of the stored
	// value may reach the HTML unescaped — the same discipline as the URL and
	// bind-address fields.
	withTempConfig(t)
	const inject = `" onfocus="alert(1)` + `x`
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: inject, LineNumber: 1, APIToken: inject})

	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))

	if strings.Contains(w.Body.String(), `onfocus="alert(1)`) {
		t.Errorf("settings form reflected an unescaped attribute break:\n%s", w.Body.String())
	}
}

func TestSettingsPost_savesTheToken(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"3"},
		"token":         {"  minted-token  "},
	}
	submitSettings(t, h, form)

	if got := loadConfig(); got.APIToken != "minted-token" {
		t.Errorf("APIToken = %q, want the trimmed token", got.APIToken)
	}
}

func TestSettingsPost_blankTokenKeepsTheSavedOne(t *testing.T) {
	// The field renders empty because it is never echoed back. If an empty
	// submit wiped the token, changing the port would silently unbind the
	// watcher.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 3, APIToken: "minted-token"})

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"9999"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"3"},
		"token":         {""},
	}
	submitSettings(t, h, form)

	got := loadConfig()
	if got.APIToken != "minted-token" {
		t.Errorf("APIToken = %q, want the previously saved token", got.APIToken)
	}
	if got.Port != 9999 {
		t.Errorf("Port = %d, want 9999 — the rest of the form must still save", got.Port)
	}
}

func TestSettingsPost_clearingTheTokenUnbindsTheWatcher(t *testing.T) {
	// "No token" is a reachable state, not an accident: it is how the operator
	// stops the tray talking to the dashboard.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 3, APIToken: "minted-token"})

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"3"},
		"token":         {""},
		"token_clear":   {"1"},
	}
	submitSettings(t, h, form)

	if got := loadConfig(); got.APIToken != "" {
		t.Errorf("APIToken = %q, want it cleared", got.APIToken)
	}
}

func TestSettingsPost_aNewTokenBeatsTheClearCheckbox(t *testing.T) {
	// Typing a token and leaving the checkbox ticked is the operator saying
	// "replace it". Honour the value they typed.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "old-token"})

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"1"},
		"token":         {"new-token"},
		"token_clear":   {"1"},
	}
	submitSettings(t, h, form)

	if got := loadConfig(); got.APIToken != "new-token" {
		t.Errorf("APIToken = %q, want new-token", got.APIToken)
	}
}

func TestSettingsSavedPage_neverEchoesTheToken(t *testing.T) {
	withTempConfig(t)
	const token = "sk-live-must-not-be-rendered"
	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"1"},
		"token":         {token},
	}
	w := submitSettings(t, h, form)

	if strings.Contains(w.Body.String(), token) {
		t.Errorf("the saved page echoed the token back:\n%s", w.Body.String())
	}
}

func TestSettingsPost_neverLogsTheToken(t *testing.T) {
	withTempConfig(t)
	const token = "sk-live-do-not-log-me"
	var logs strings.Builder
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(origOut); log.SetFlags(origFlags) })

	h := &settingsHandler{}
	form := url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"1"},
		"token":         {token},
	}
	submitSettings(t, h, form)

	if strings.Contains(logs.String(), token) {
		t.Errorf("saving settings logged the token:\n%s", logs.String())
	}
}

// -------------------------------------------------------- tray watch line

func TestAppState_watchLineDefaultsToZero(t *testing.T) {
	if got := NewAppState().WatchLine(); got != 0 {
		t.Errorf("WatchLine = %d, want 0 before anything is bound", got)
	}
}

func TestAppState_watchLineRoundTrips(t *testing.T) {
	app := NewAppState()
	app.SetWatchLine(6)
	if got := app.WatchLine(); got != 6 {
		t.Errorf("WatchLine = %d, want 6", got)
	}
}

func TestWatchLoop_handsTheBeaconBackDarkWhenTheTokenIsCleared(t *testing.T) {
	// Unbinding mid-pause must not leave the beacon spinning on a level nobody
	// is watching any more. That is the confident wrong light, arrived at by
	// the back door.
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	var mu sync.Mutex
	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	get := func() Config {
		mu.Lock()
		defer mu.Unlock()
		return cfg
	}
	startWatchLoop(t, app, srv.Client(), get, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})

	mu.Lock()
	cfg.APIToken = "" // the operator ticks "forget the saved token"
	mu.Unlock()

	waitFor(t, "the beacon to be handed back dark", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchNoToken
	})
}

func TestWatchLoop_releasesTheBeaconOnceAndThenLeavesItAlone(t *testing.T) {
	// Handing the beacon back is a one-off, not a policy: once unbound, the
	// operator's manual control has to stick.
	stub := newStubDashboard(pausedPayload)
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)

	var mu sync.Mutex
	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	get := func() Config {
		mu.Lock()
		defer mu.Unlock()
		return cfg
	}
	startWatchLoop(t, app, srv.Client(), get, 5*time.Millisecond)

	waitFor(t, "the beacon to spin", func() bool {
		state, _, _ := app.Get()
		return state == StateSpin
	})

	mu.Lock()
	cfg.DashboardURL = "" // the operator blanks the URL
	mu.Unlock()

	waitFor(t, "the beacon to be handed back dark", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchOff
	})

	// Now the operator drives it by hand. The unbound watcher must not fight.
	applyState(app, StateSpin)
	time.Sleep(40 * time.Millisecond)
	if state, _, _ := app.Get(); state != StateSpin {
		t.Errorf("state = %q, want spin — an unbound watcher must not stomp manual control", state)
	}
	if b.stops != 1 {
		t.Errorf("Stop called %d times, want exactly 1 — the release is a one-off", b.stops)
	}
}

// ------------------------------------- forging a settings save from a page

func TestSettingsForm_carriesACSRFToken(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))

	m := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("settings form has no CSRF token:\n%s", w.Body.String())
	}
	if len(m[1]) < 32 {
		t.Errorf("CSRF token %q is too short to be unguessable", m[1])
	}
}

func TestSettingsPost_refusesASubmitWithNoCSRFToken(t *testing.T) {
	// The exploit this blocks: a page the operator happens to visit auto-POSTs
	// a form to http://127.0.0.1:1337/settings pointing dashboard_url at the
	// attacker's host, with the token field left blank so the saved token is
	// carried forward. The watcher would then send the dashboard credential to
	// the attacker in an Authorization header on its very next poll.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 1, APIToken: "minted-token"})

	h := &settingsHandler{}
	w := postSettings(t, h, url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://attacker.example"},
		"line_number":   {"1"},
		"token":         {""},
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a submit with no CSRF token", w.Code)
	}
	got := loadConfig()
	if got.DashboardURL != "https://dash.example.com" {
		t.Errorf("DashboardURL = %q — a forged submit repointed the watcher", got.DashboardURL)
	}
	if got.APIToken != "minted-token" {
		t.Errorf("APIToken = %q — the saved config was written by a forged submit", got.APIToken)
	}
}

func TestSettingsPost_refusesAGuessedCSRFToken(t *testing.T) {
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 1, APIToken: "minted-token"})

	h := &settingsHandler{}
	w := postSettings(t, h, url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://attacker.example"},
		"line_number":   {"1"},
		"csrf":          {"not-the-real-token"},
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a wrong CSRF token", w.Code)
	}
	if got := loadConfig(); got.DashboardURL != "https://dash.example.com" {
		t.Errorf("DashboardURL = %q — a guessed token was accepted", got.DashboardURL)
	}
}

func TestSettingsPost_doesNotCarryTheTokenToADifferentDashboard(t *testing.T) {
	// Defence in depth, and correct on its own terms: a token minted by one
	// dashboard is not a credential for another. Changing the URL without
	// pasting a token unbinds rather than redirecting the credential.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 1, APIToken: "minted-token"})

	h := &settingsHandler{}
	submitSettings(t, h, url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://somewhere-else.example"},
		"line_number":   {"1"},
		"token":         {""},
	})

	got := loadConfig()
	if got.DashboardURL != "https://somewhere-else.example" {
		t.Errorf("DashboardURL = %q, want the new one", got.DashboardURL)
	}
	if got.APIToken != "" {
		t.Errorf("APIToken = %q — the old dashboard's token followed the URL to a new host", got.APIToken)
	}
}

func TestSettingsPost_keepsTheTokenWhenOnlyTheLineChanges(t *testing.T) {
	// The flip side: switching lines on the SAME dashboard must not make the
	// operator re-paste the token every time.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort, DashboardURL: "https://dash.example.com", LineNumber: 1, APIToken: "minted-token"})

	h := &settingsHandler{}
	submitSettings(t, h, url.Values{
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dash.example.com"},
		"line_number":   {"4"},
		"token":         {""},
	})

	got := loadConfig()
	if got.APIToken != "minted-token" {
		t.Errorf("APIToken = %q, want it kept across a line change", got.APIToken)
	}
	if got.LineNumber != 4 {
		t.Errorf("LineNumber = %d, want 4", got.LineNumber)
	}
}

func TestSaveConfig_keepsTheTokenFileToTheOwner(t *testing.T) {
	// The config file used to hold an address, a port and a line number. It now
	// holds a bearer credential, so world-readable is no longer good enough —
	// the dashboard persists its own copy of this token at 0600 too.
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes are not the access-control mechanism on Windows")
	}
	withTempConfig(t)
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %#o, want 0600 — it holds a credential", mode)
	}
}

func TestSaveConfig_tightensAnAlreadyWorldReadableConfig(t *testing.T) {
	// Upgrading from a build that predates the token leaves a 0644 file on
	// disk. os.WriteFile does not change an existing file's mode, so saving
	// over it has to do that deliberately or the credential lands in a
	// world-readable file on every existing install.
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes are not the access-control mechanism on Windows")
	}
	withTempConfig(t)
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"addr":"127.0.0.1","port":1337,"line_number":1}`), 0o644); err != nil {
		t.Fatalf("seeding an old config: %v", err)
	}

	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %#o, want 0600 after re-saving over an old 0644 file", mode)
	}
}

func TestSaveConfig_neverWritesTheTokenIntoTheOldWorldReadableFile(t *testing.T) {
	// Narrowing the mode AFTER the write still exposes the credential for as
	// long as the write takes: os.WriteFile truncates the existing 0644 file in
	// place and puts the token in it, and only then does Chmod narrow it. The
	// fix is to write a fresh 0600 file and rename it over the old one, so the
	// old inode never holds the token at all — which is what this asserts.
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes and inodes are not the mechanism on Windows")
	}
	withTempConfig(t)
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"addr":"127.0.0.1","port":1337,"line_number":1}`), 0o644); err != nil {
		t.Fatalf("seeding an old config: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if os.SameFile(before, after) {
		t.Error("the world-readable file was written in place — the token was exposed for the length of the write")
	}
	if mode := after.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %#o, want 0600", mode)
	}
}

func TestSaveConfig_leavesNoTemporaryFilesBehind(t *testing.T) {
	withTempConfig(t)
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort, LineNumber: 1, APIToken: "minted-token"}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != filepath.Base(path) {
			t.Errorf("stray file left next to the config: %q", e.Name())
		}
	}
}
