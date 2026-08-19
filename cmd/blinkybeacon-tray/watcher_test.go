package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// errTestBeaconGone stands in for a USB write failure.
var errTestBeaconGone = errors.New("beacon gone")

// secs is a helper for the *float64 seconds_since_gsi field.
func secs(f float64) *float64 { return &f }

// liveFeed is a LineState that is fresh, unpaused and mid-game — the baseline
// every test mutates one field of.
func liveFeed() *LineState {
	return &LineState{
		Paused:          false,
		GameState:       "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
		SecondsSinceGSI: secs(0.7),
		Running:         true,
	}
}

func draftFeed() *LineState {
	ls := liveFeed()
	ls.GameState = gameStateHeroSelection
	return ls
}

// ---------------------------------------------------------------- URL shape

func TestStateSourceURL_usesExistingLineRoute(t *testing.T) {
	got, err := stateSourceURL("http://192.168.1.50:8080", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "http://192.168.1.50:8080/line/3/state"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStateSourceURL_tolerdatesTrailingSlash(t *testing.T) {
	got, err := stateSourceURL("http://dash.local/", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://dash.local/line/1/state" {
		t.Errorf("got %q", got)
	}
}

func TestStateSourceURL_addsSchemeWhenMissing(t *testing.T) {
	got, err := stateSourceURL("192.168.1.50:8080", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://192.168.1.50:8080/line/2/state" {
		t.Errorf("got %q", got)
	}
}

func TestStateSourceURL_rejectsEmptyURL(t *testing.T) {
	if _, err := stateSourceURL("", 1); err == nil {
		t.Error("expected an error for an empty dashboard URL")
	}
}

func TestStateSourceURL_rejectsBadLineNumber(t *testing.T) {
	if _, err := stateSourceURL("http://dash.local", 0); err == nil {
		t.Error("expected an error for line number 0")
	}
}

// ---------------------------------------------------------------- fetching

func TestFetchLineState_decodesTheRealDashboardContract(t *testing.T) {
	// Field names, types and nesting copied from DOTA-DASHBOARD
	// gsi/view_models.py::_snapshot — the payload GET /line/<N>/state returns.
	const payload = `{
	  "label": "Line A",
	  "match_id": "7891234567",
	  "paused": true,
	  "pause_active_seconds": 42.5,
	  "pause_team_assigned": "radiant",
	  "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
	  "game_time": 1234,
	  "clock_time": 1200,
	  "last_gsi_at": 1755600000.0,
	  "seconds_since_gsi": 0.83,
	  "stale_telemetry": false,
	  "telemetry_state": "live",
	  "running": true
	}`

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/line/1/state" {
		t.Errorf("requested %q, want /line/1/state", gotPath)
	}
	if !ls.Paused {
		t.Error("expected Paused=true")
	}
	if ls.GameState != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" {
		t.Errorf("GameState = %q", ls.GameState)
	}
	if ls.SecondsSinceGSI == nil || *ls.SecondsSinceGSI != 0.83 {
		t.Errorf("SecondsSinceGSI = %v", ls.SecondsSinceGSI)
	}
}

func TestFetchLineState_acceptsNullSecondsSinceGSI(t *testing.T) {
	// A line that has never received GSI publishes nulls, not zeros.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"paused": false, "game_state": null, "seconds_since_gsi": null}`))
	}))
	defer srv.Close()

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.SecondsSinceGSI != nil {
		t.Errorf("expected nil SecondsSinceGSI, got %v", *ls.SecondsSinceGSI)
	}
	if ls.GameState != "" {
		t.Errorf("expected empty GameState for null, got %q", ls.GameState)
	}
}

func TestFetchLineState_errorsOnRedirectToLanding(t *testing.T) {
	// The dashboard sends an unauthenticated safe GET to the landing page with
	// a 302 (auth/middleware.py). Following it yields 200 + HTML, which must
	// NOT be mistaken for a good poll.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/line/1/state" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Write([]byte("<html>Access Restricted</html>"))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1); err == nil {
		t.Fatal("expected an error when the dashboard redirects to the landing page")
	}
}

func TestFetchLineState_errorsOnForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1); err == nil {
		t.Fatal("expected an error on 403")
	}
}

func TestFetchLineState_errorsOnUnknownLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"unknown line"}`))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 9); err == nil {
		t.Fatal("expected an error on 404")
	}
}

func TestFetchLineState_errorsOnNonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not the dashboard</html>"))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1); err == nil {
		t.Fatal("expected an error on a non-JSON body")
	}
}

// ------------------------------------------------- level-driven decisions

func TestDecide_idleWhileDraftIsStillRunning(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	state, status := w.Decide(now, draftFeed())
	if state != StateIdle {
		t.Errorf("state = %q, want idle during the draft", state)
	}
	if status != WatchOK {
		t.Errorf("status = %q, want ok", status)
	}
}

func TestDecide_flashesWhenDraftEnds(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())

	state, _ := w.Decide(now.Add(time.Second), liveFeed())
	if state != StateFlash {
		t.Errorf("state = %q, want flash at draft end", state)
	}
}

func TestDecide_flashLastsFiveSecondsThenIdles(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), liveFeed()) // draft ends here

	if state, _ := w.Decide(now.Add(5*time.Second), liveFeed()); state != StateFlash {
		t.Errorf("at t+4s of the flash: state = %q, want flash", state)
	}
	if state, _ := w.Decide(now.Add(6*time.Second), liveFeed()); state != StateFlash {
		t.Errorf("at t+5s exactly: state = %q, want flash", state)
	}
	if state, _ := w.Decide(now.Add(6100*time.Millisecond), liveFeed()); state != StateIdle {
		t.Errorf("after the 5s flash: state = %q, want idle", state)
	}
}

func TestDecide_flashesWhenDraftEndsStraightIntoPreGame(t *testing.T) {
	// Draft end is leaving HERO_SELECTION, whatever comes next.
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())

	ls := liveFeed()
	ls.GameState = "DOTA_GAMERULES_STATE_STRATEGY_TIME"
	if state, _ := w.Decide(now.Add(time.Second), ls); state != StateFlash {
		t.Errorf("state = %q, want flash entering strategy time", state)
	}
}

func TestDecide_flashesWhenAMissedPollSkipsStrategyTime(t *testing.T) {
	// A poll gap can take us straight from HERO_SELECTION to GAME_IN_PROGRESS.
	// The draft still ended; the beacon must still flash.
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())

	ls := liveFeed()
	ls.GameState = "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS"
	if state, _ := w.Decide(now.Add(30*time.Second), ls); state != StateFlash {
		t.Errorf("state = %q, want flash after a missed poll across the draft end", state)
	}
}

func TestDecide_duplicatePollDoesNotRestartTheFlash(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), liveFeed()) // draft ends; flash until t+6s

	// Four more polls of the identical level must not extend the flash.
	for i := 2; i <= 5; i++ {
		w.Decide(now.Add(time.Duration(i)*time.Second), liveFeed())
	}
	if state, _ := w.Decide(now.Add(6100*time.Millisecond), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — duplicate polls must not re-arm the flash", state)
	}
}

func TestDecide_duplicateDraftPollsDoNotFlash(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	for i := 0; i < 5; i++ {
		state, _ := w.Decide(now.Add(time.Duration(i)*time.Second), draftFeed())
		if state != StateIdle {
			t.Fatalf("poll %d: state = %q, want idle — the draft has not ended", i, state)
		}
	}
}

func TestDecide_spinsWhilePaused(t *testing.T) {
	w := NewWatcher()
	ls := liveFeed()
	ls.Paused = true
	if state, status := w.Decide(time.Now(), ls); state != StateSpin || status != WatchOK {
		t.Errorf("state = %q status = %q, want spin/ok", state, status)
	}
}

func TestDecide_stopsWhenThePauseEnds(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	paused := liveFeed()
	paused.Paused = true
	w.Decide(now, paused)

	if state, _ := w.Decide(now.Add(time.Second), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle once the pause ends", state)
	}
}

func TestDecide_restartMidPauseSpinsImmediately(t *testing.T) {
	// A freshly started watcher has seen no transitions at all. Level-driven
	// means the very first poll of a paused line must already spin.
	w := NewWatcher()
	ls := liveFeed()
	ls.Paused = true
	if state, _ := w.Decide(time.Now(), ls); state != StateSpin {
		t.Errorf("state = %q, want spin on the first poll after a restart mid-pause", state)
	}
}

func TestDecide_pauseDuringTheFlashWins(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), liveFeed()) // flashing until t+6s

	paused := liveFeed()
	paused.Paused = true
	if state, _ := w.Decide(now.Add(2*time.Second), paused); state != StateSpin {
		t.Errorf("state = %q, want spin — a pause outranks the flash", state)
	}
}

func TestDecide_flashDoesNotResumeAfterAPauseSwallowsIt(t *testing.T) {
	// The pause covered the whole flash window; when it lifts, the flash is
	// long expired and must not come back.
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), liveFeed())

	paused := liveFeed()
	paused.Paused = true
	w.Decide(now.Add(2*time.Second), paused)

	if state, _ := w.Decide(now.Add(30*time.Second), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle", state)
	}
}

func TestDecide_restartMidDraftDoesNotFlashOnItsFirstPoll(t *testing.T) {
	// First poll ever lands on GAME_IN_PROGRESS. We have no evidence a draft
	// just ended, so we must not invent a flash.
	w := NewWatcher()
	if state, _ := w.Decide(time.Now(), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle on a cold first poll", state)
	}
}

// ------------------------------------------------------------- feed loss

func TestDecide_feedLostWhenThePollFails(t *testing.T) {
	w := NewWatcher()
	state, status := w.Decide(time.Now(), nil)
	if state != StateIdle {
		t.Errorf("state = %q, want idle (dark) on a failed poll", state)
	}
	if status != WatchLost {
		t.Errorf("status = %q, want lost", status)
	}
}

func TestDecide_feedLostDuringAPauseGoesDark(t *testing.T) {
	// Never a confident wrong light: a stale `paused` must not keep spinning.
	w := NewWatcher()
	now := time.Now()
	paused := liveFeed()
	paused.Paused = true
	if state, _ := w.Decide(now, paused); state != StateSpin {
		t.Fatal("precondition: expected spin while paused")
	}

	state, status := w.Decide(now.Add(time.Second), nil)
	if state != StateIdle {
		t.Errorf("state = %q, want idle when the feed dies mid-pause", state)
	}
	if status != WatchLost {
		t.Errorf("status = %q, want lost", status)
	}
}

func TestDecide_feedLostWhenGSIIsStale(t *testing.T) {
	w := NewWatcher()
	ls := liveFeed()
	ls.SecondsSinceGSI = secs(feedLostAfterSeconds + 1)
	state, status := w.Decide(time.Now(), ls)
	if state != StateIdle || status != WatchLost {
		t.Errorf("state = %q status = %q, want idle/lost on stale GSI", state, status)
	}
}

func TestDecide_staleGSIOutranksAPause(t *testing.T) {
	w := NewWatcher()
	ls := liveFeed()
	ls.Paused = true
	ls.SecondsSinceGSI = secs(feedLostAfterSeconds + 1)
	if state, _ := w.Decide(time.Now(), ls); state != StateIdle {
		t.Errorf("state = %q, want idle — a stale pause is a lie", state)
	}
}

func TestDecide_feedLostWhenNoGSIHasEverArrived(t *testing.T) {
	w := NewWatcher()
	ls := liveFeed()
	ls.SecondsSinceGSI = nil
	state, status := w.Decide(time.Now(), ls)
	if state != StateIdle || status != WatchLost {
		t.Errorf("state = %q status = %q, want idle/lost when seconds_since_gsi is null", state, status)
	}
}

func TestDecide_feedLossCancelsAPendingFlash(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), liveFeed()) // flashing until t+6s

	if state, _ := w.Decide(now.Add(2*time.Second), nil); state != StateIdle {
		t.Fatal("expected dark on feed loss")
	}
	// Feed comes back inside the original flash window — the flash is gone.
	if state, _ := w.Decide(now.Add(3*time.Second), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — a cancelled flash must not resume", state)
	}
}

func TestDecide_feedLossSpanningTheDraftEndDoesNotFlashLate(t *testing.T) {
	// The gap swallowed the moment the draft ended. Firing the flash on
	// recovery would tell the desk "the draft just ended" a minute late.
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, draftFeed())
	w.Decide(now.Add(time.Second), nil) // feed dies during the draft

	if state, _ := w.Decide(now.Add(90*time.Second), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — no retroactive flash after a feed gap", state)
	}
}

func TestDecide_recoversToSpinIfTheLineIsStillPaused(t *testing.T) {
	w := NewWatcher()
	now := time.Now()
	w.Decide(now, nil)

	paused := liveFeed()
	paused.Paused = true
	if state, status := w.Decide(now.Add(time.Second), paused); state != StateSpin || status != WatchOK {
		t.Errorf("state = %q status = %q, want spin/ok on recovery into a live pause", state, status)
	}
}

func TestDecide_notRunningLineIsNotAFeedLoss(t *testing.T) {
	// `running: false` means the line is not accepting GSI. It is a calm idle,
	// not a broken feed — but it must never light the beacon.
	w := NewWatcher()
	ls := &LineState{Paused: false, GameState: "", SecondsSinceGSI: nil, Running: false}
	if state, _ := w.Decide(time.Now(), ls); state != StateIdle {
		t.Errorf("state = %q, want idle for a stopped line", state)
	}
}

// --------------------------------------------------- applying to the beacon

// countingBeacon records how many times each command was issued so the apply
// layer can be checked for idempotence.
type countingBeacon struct {
	spins, flashes, stops int
	err                   error
}

func (b *countingBeacon) Spin() error  { b.spins++; return b.err }
func (b *countingBeacon) Flash() error { b.flashes++; return b.err }
func (b *countingBeacon) Stop() error  { b.stops++; return b.err }
func (b *countingBeacon) Close() error { return nil }

func TestApplyState_issuesTheCommandOnce(t *testing.T) {
	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)

	applyState(app, StateSpin)
	applyState(app, StateSpin)
	applyState(app, StateSpin)

	if b.spins != 1 {
		t.Errorf("Spin called %d times, want 1 — the apply layer must be idempotent", b.spins)
	}
	if state, _, _ := app.Get(); state != StateSpin {
		t.Errorf("AppState = %q, want spin", state)
	}
}

func TestApplyState_switchesBetweenModes(t *testing.T) {
	app := NewAppState()
	b := &countingBeacon{}
	app.SetBeacon(b)

	applyState(app, StateFlash)
	applyState(app, StateSpin)
	applyState(app, StateIdle)

	if b.flashes != 1 || b.spins != 1 || b.stops != 1 {
		t.Errorf("flashes=%d spins=%d stops=%d, want 1/1/1", b.flashes, b.spins, b.stops)
	}
}

func TestApplyState_noBeaconIsNotACrash(t *testing.T) {
	app := NewAppState()
	applyState(app, StateSpin) // no beacon connected
	if state, _, _ := app.Get(); state != StateIdle {
		t.Errorf("AppState = %q, want idle — nothing to drive", state)
	}
}

func TestApplyState_marksTheBeaconGoneOnAWriteError(t *testing.T) {
	app := NewAppState()
	b := &countingBeacon{err: errTestBeaconGone}
	app.SetBeacon(b)

	applyState(app, StateSpin)

	if _, connected, _ := app.Get(); connected {
		t.Error("expected the beacon to be marked disconnected after a failed command")
	}
}

// ---------------------------------------------------------- watch status

func TestAppState_watchStatusDefaultsToOff(t *testing.T) {
	app := NewAppState()
	if got := app.WatchStatus(); got != WatchOff {
		t.Errorf("WatchStatus = %q, want off", got)
	}
}

func TestAppState_watchStatusRoundTrips(t *testing.T) {
	app := NewAppState()
	app.SetWatchStatus(WatchLost)
	if got := app.WatchStatus(); got != WatchLost {
		t.Errorf("WatchStatus = %q, want lost", got)
	}
}

func TestWatchStatusLabel_namesALostFeedExplicitly(t *testing.T) {
	// The beacon is dark for both "nothing happening" and "the dashboard is
	// gone". The tray is the only place that difference can be seen.
	lost := watchStatusLabel(WatchLost)
	if !strings.Contains(strings.ToUpper(lost), "LOST") {
		t.Errorf("WatchLost label = %q, want it to say the feed is lost", lost)
	}

	labels := map[WatchStatus]string{
		WatchOff:     watchStatusLabel(WatchOff),
		WatchOK:      watchStatusLabel(WatchOK),
		WatchLost:    lost,
		WatchStopped: watchStatusLabel(WatchStopped),
	}
	seen := map[string]WatchStatus{}
	for status, label := range labels {
		if label == "" {
			t.Errorf("%q has an empty label", status)
		}
		if other, dup := seen[label]; dup {
			t.Errorf("%q and %q share the label %q", status, other, label)
		}
		seen[label] = status
	}
}
