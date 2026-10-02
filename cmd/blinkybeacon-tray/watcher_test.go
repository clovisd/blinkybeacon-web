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

func TestStateSourceURL_usesTheTokenGatedReadAPI(t *testing.T) {
	got, err := stateSourceURL("https://dash.example.com", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://dash.example.com/api/v0/lines/3"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestStateSourceURL_toleratesTrailingSlash(t *testing.T) {
	got, err := stateSourceURL("http://dash.local/", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://dash.local/api/v0/lines/1" {
		t.Errorf("got %q", got)
	}
}

func TestStateSourceURL_addsSchemeWhenMissing(t *testing.T) {
	got, err := stateSourceURL("192.168.1.50:8080", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://192.168.1.50:8080/api/v0/lines/2" {
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

// v0Projection is the exact payload of the design spec's §4.1 example — the
// eight fields the dashboard's GET /api/v0/lines/{n} promises, byte for byte.
const v0Projection = `{"v": 0, "n": 3, "label": "Line C", "running": true,
 "match_id": null,
 "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
 "paused": false, "seconds_since_gsi": 0.8, "ts": 1765500000}`

// unauthorizedBody stands in for the dashboard's one constant 401 body. Its
// content is deliberately uninformative — §4.2's no-oracle rule.
const unauthorizedBody = `{"v":0,"error":"unauthorized"}`

func TestFetchLineState_decodesTheV0Projection(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(v0Projection))
	}))
	defer srv.Close()

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 3, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v0/lines/3" {
		t.Errorf("requested %q, want /api/v0/lines/3", gotPath)
	}
	if ls.V != 0 {
		t.Errorf("V = %d, want 0", ls.V)
	}
	if ls.N != 3 {
		t.Errorf("N = %d, want 3", ls.N)
	}
	if ls.Label != "Line C" {
		t.Errorf("Label = %q, want %q", ls.Label, "Line C")
	}
	if !ls.Running {
		t.Error("expected Running=true")
	}
	if ls.MatchID != nil {
		t.Errorf("MatchID = %v, want nil", *ls.MatchID)
	}
	if ls.GameState != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" {
		t.Errorf("GameState = %q", ls.GameState)
	}
	if ls.Paused {
		t.Error("expected Paused=false")
	}
	if ls.SecondsSinceGSI == nil || *ls.SecondsSinceGSI != 0.8 {
		t.Errorf("SecondsSinceGSI = %v, want 0.8", ls.SecondsSinceGSI)
	}
	if ls.TS != 1765500000 {
		t.Errorf("TS = %d, want 1765500000", ls.TS)
	}
}

func TestFetchLineState_decodesAPopulatedMatchID(t *testing.T) {
	// match_id is nullable but carries the dashboard's own string ids when set.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567",` +
			`"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":true,` +
			`"seconds_since_gsi":0.83,"ts":1765500000}`))
	}))
	defer srv.Close()

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.MatchID == nil || *ls.MatchID != "7891234567" {
		t.Errorf("MatchID = %v, want 7891234567", ls.MatchID)
	}
	if !ls.Paused {
		t.Error("expected Paused=true")
	}
}

func TestFetchLineState_sendsTheTokenAsABearerHeader(t *testing.T) {
	var gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.RawQuery
		w.Write([]byte(v0Projection))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 3, "s3cr3t-token"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer s3cr3t-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer s3cr3t-token")
	}
	// The credential rides the header and only the header: a token in a URL
	// lands in every reverse proxy's access log between here and the origin.
	if gotQuery != "" {
		t.Errorf("request carried a query string %q — the token must never travel in a URL", gotQuery)
	}
}

func TestFetchLineState_reportsARejectedTokenDistinctly(t *testing.T) {
	// A 401 is not "the network is down": it is a specific, actionable failure
	// that the tray has to be able to name.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(unauthorizedBody))
	}))
	defer srv.Close()

	_, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "wrong-token")
	if err == nil {
		t.Fatal("expected an error on 401")
	}
	var pe *pollError
	if !errors.As(err, &pe) {
		t.Fatalf("error %v is not a *pollError — the loop cannot label it", err)
	}
	if pe.Status != http.StatusUnauthorized {
		t.Errorf("pollError.Status = %d, want 401", pe.Status)
	}
}

func TestFetchLineState_reportsAnUnknownLineDistinctly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"v":0,"error":"not found"}`))
	}))
	defer srv.Close()

	_, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 9, "tok")
	if err == nil {
		t.Fatal("expected an error on 404")
	}
	var pe *pollError
	if !errors.As(err, &pe) {
		t.Fatalf("error %v is not a *pollError", err)
	}
	if pe.Status != http.StatusNotFound {
		t.Errorf("pollError.Status = %d, want 404", pe.Status)
	}
}

func TestFetchLineState_neverNamesTheTokenInItsErrors(t *testing.T) {
	// Every fetch error ends up in the log. The token must not ride along.
	const token = "tok-must-never-be-logged"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(unauthorizedBody))
	}))
	defer srv.Close()

	_, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, token)
	if err == nil {
		t.Fatal("expected an error on 401")
	}
	if strings.Contains(err.Error(), token) {
		t.Errorf("fetch error leaks the token: %v", err)
	}
}

func TestFetchLineState_acceptsNullSecondsSinceGSI(t *testing.T) {
	// A line that has never received GSI publishes nulls, not zeros.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"v":0,"n":1,"label":"Line A","running":true,"match_id":null,` +
			`"game_state":null,"paused":false,"seconds_since_gsi":null,"ts":1765500000}`))
	}))
	defer srv.Close()

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok")
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
	// A dashboard that has not shipped the read API yet answers the browser
	// gate: 302 to the landing page. Following it yields 200 + HTML, which must
	// NOT be mistaken for a good poll.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		w.Write([]byte("<html>Access Restricted</html>"))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok"); err == nil {
		t.Fatal("expected an error when the dashboard redirects to the landing page")
	}
}

func TestFetchLineState_errorsOnForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok"); err == nil {
		t.Fatal("expected an error on 403")
	}
}

func TestFetchLineState_errorsOnNonJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not the dashboard</html>"))
	}))
	defer srv.Close()

	if _, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok"); err == nil {
		t.Fatal("expected an error on a non-JSON body")
	}
}

// ------------------------------------------------- level-driven decisions

func TestDecide_idleWhileDraftIsStillRunning(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	state, status := w.Decide(now, draftFeed())
	if state != StateIdle {
		t.Errorf("state = %q, want idle during the draft", state)
	}
	if status != WatchOK {
		t.Errorf("status = %q, want ok", status)
	}
}

func TestDecide_flashesAfterAMissedPollAcrossTheLastPick(t *testing.T) {
	// The edge is between two polls, not between two clock ticks: a gap that
	// swallows several polls still leaves false on one side and true on the
	// other, and the draft still ended.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())

	if state, _ := w.Decide(now.Add(30*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("state = %q, want flash after a missed poll across the last pick", state)
	}
}

func TestDecide_pollingAnUnfinishedDraftNeverFlashes(t *testing.T) {
	// false on its own is not an edge, however many times it arrives.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	for i := 0; i < 5; i++ {
		state, _ := w.Decide(now.Add(time.Duration(i)*time.Second), draftingFeed())
		if state != StateIdle {
			t.Fatalf("poll %d: state = %q, want idle — picks are still outstanding", i, state)
		}
	}
}

func TestDecide_spinsWhilePaused(t *testing.T) {
	w := NewWatcher(defaultSettings())
	ls := liveFeed()
	ls.Paused = true
	if state, status := w.Decide(time.Now(), ls); state != StateSpin || status != WatchOK {
		t.Errorf("state = %q status = %q, want spin/ok", state, status)
	}
}

func TestDecide_stopsWhenThePauseEnds(t *testing.T) {
	w := NewWatcher(defaultSettings())
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
	w := NewWatcher(defaultSettings())
	ls := liveFeed()
	ls.Paused = true
	if state, _ := w.Decide(time.Now(), ls); state != StateSpin {
		t.Errorf("state = %q, want spin on the first poll after a restart mid-pause", state)
	}
}

func TestDecide_flashDoesNotResumeAfterAPauseSwallowsIt(t *testing.T) {
	// The pause covered the whole flash window; when it lifts, the flash is
	// long expired and must not come back.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed()) // flashing until t+15s

	paused := draftDoneFeed()
	paused.Paused = true
	w.Decide(now.Add(2*time.Second), paused)

	if state, _ := w.Decide(now.Add(30*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle", state)
	}
}

func TestDecide_restartMidDraftDoesNotFlashOnItsFirstPoll(t *testing.T) {
	// First poll ever lands on GAME_IN_PROGRESS. We have no evidence a draft
	// just ended, so we must not invent a flash.
	w := NewWatcher(defaultSettings())
	if state, _ := w.Decide(time.Now(), liveFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle on a cold first poll", state)
	}
}

// ------------------------------------------------------------- feed loss

func TestDecide_feedLostWhenThePollFails(t *testing.T) {
	w := NewWatcher(defaultSettings())
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
	w := NewWatcher(defaultSettings())
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

func TestDecide_staleGSIOutranksAPause(t *testing.T) {
	w := NewWatcher(defaultSettings())
	ls := liveFeed()
	ls.Paused = true
	ls.SecondsSinceGSI = secs(feedLostAfterSeconds + 1)
	if state, _ := w.Decide(time.Now(), ls); state != StateIdle {
		t.Errorf("state = %q, want idle — a stale pause is a lie", state)
	}
}

func TestDecide_feedLossCancelsAPendingFlash(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed()) // flashing until t+15s

	if state, _ := w.Decide(now.Add(2*time.Second), nil); state != StateIdle {
		t.Fatal("expected dark on feed loss")
	}
	// Feed comes back inside the original flash window — the flash is gone.
	if state, _ := w.Decide(now.Add(3*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — a cancelled flash must not resume", state)
	}
}

func TestDecide_recoversToSpinIfTheLineIsStillPaused(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, nil)

	paused := liveFeed()
	paused.Paused = true
	if state, status := w.Decide(now.Add(time.Second), paused); state != StateSpin || status != WatchOK {
		t.Errorf("state = %q status = %q, want spin/ok on recovery into a live pause", state, status)
	}
}

func TestDecide_stoppedLineIsDarkEvenWhilePausedOnAFreshFeed(t *testing.T) {
	// Stage A's retro §1 clarification (3): a STOPPED line can carry
	// paused:true with a perfectly fresh feed. `running` has to darken the
	// beacon on its own — the pause is real but nobody is playing, so
	// spiralling on it would be a confident wrong light.
	//
	// The pre-existing stopped-line test also had a null seconds_since_gsi,
	// which darkens the beacon by itself, so it never isolated `running`.
	w := NewWatcher(defaultSettings())
	ls := &LineState{
		Running:         false,
		Paused:          true,
		GameState:       "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
		SecondsSinceGSI: secs(0.4), // fresh: nothing else here says "go dark"
	}
	state, status := w.Decide(time.Now(), ls)
	if state != StateIdle {
		t.Errorf("state = %q, want idle — a stopped line must be dark however paused it claims to be", state)
	}
	if status != WatchStopped {
		t.Errorf("status = %q, want stopped", status)
	}
}

func TestDecide_notRunningLineIsNotAFeedLoss(t *testing.T) {
	// `running: false` means the line is not accepting GSI. It is a calm idle,
	// not a broken feed — but it must never light the beacon.
	w := NewWatcher(defaultSettings())
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

func TestWatchStatusLabel_givesEveryStatusItsOwnWords(t *testing.T) {
	// Eight of these nine mean "the beacon is dark". If any two share a label,
	// the tray has stopped being able to tell the operator which one it is.
	all := []WatchStatus{WatchOff, WatchNoToken, WatchOK, WatchLost, WatchRejected,
		WatchNoLine, WatchStopped, WatchNoGameYet, WatchQuiet}
	seen := map[string]WatchStatus{}
	for _, status := range all {
		label := watchStatusLabel(status, 1, WatchDetail{})
		if label == "" {
			t.Errorf("%q has an empty label", status)
		}
		if other, dup := seen[label]; dup {
			t.Errorf("%q and %q share the label %q", status, other, label)
		}
		seen[label] = status
	}
}

func TestWatchStatusLabel_saysTheTokenWasRejected(t *testing.T) {
	// "token rejected" is the whole point: it tells the operator to go and mint
	// a new one, which no amount of "feed lost" ever would.
	got := watchStatusLabel(WatchRejected, 3, WatchDetail{})
	if !strings.Contains(got, "token rejected") {
		t.Errorf("WatchRejected label = %q, want it to name the rejected token", got)
	}
}

func TestWatchStatusLabel_namesTheMissingLineByNumber(t *testing.T) {
	got := watchStatusLabel(WatchNoLine, 7, WatchDetail{})
	if !strings.Contains(got, "line 7 not found") {
		t.Errorf("WatchNoLine label = %q, want it to name line 7", got)
	}
}

func TestWatchStatusLabel_saysWhenNoTokenIsSet(t *testing.T) {
	got := watchStatusLabel(WatchNoToken, 1, WatchDetail{})
	if !strings.Contains(got, "no token set") {
		t.Errorf("WatchNoToken label = %q, want it to say no token is set", got)
	}
}

// ------------------------------------------------- naming a failed poll

func TestPollFailureStatus_callsA401ARejectedToken(t *testing.T) {
	err := &pollError{Status: http.StatusUnauthorized, msg: "401"}
	if got := pollFailureStatus(err); got != WatchRejected {
		t.Errorf("pollFailureStatus(401) = %q, want rejected", got)
	}
}

func TestPollFailureStatus_callsA404AMissingLine(t *testing.T) {
	err := &pollError{Status: http.StatusNotFound, msg: "404"}
	if got := pollFailureStatus(err); got != WatchNoLine {
		t.Errorf("pollFailureStatus(404) = %q, want no-line", got)
	}
}

func TestPollFailureStatus_callsAnythingElseALostFeed(t *testing.T) {
	// A 500, a 403, a dead socket: all of them are "we cannot see the game",
	// and all of them keep the pre-existing feed-lost wording.
	for _, err := range []error{
		&pollError{Status: http.StatusInternalServerError, msg: "500"},
		&pollError{Status: http.StatusForbidden, msg: "403"},
		errors.New("dial tcp: connection refused"),
	} {
		if got := pollFailureStatus(err); got != WatchLost {
			t.Errorf("pollFailureStatus(%v) = %q, want lost", err, got)
		}
	}
}

// ------------------------------------------------------------- backoff

func TestPollDelay_backsOffOnlyOnARejectedToken(t *testing.T) {
	const interval = 2 * time.Second
	if got := pollDelay(interval, WatchRejected); got != rejectedBackoff {
		t.Errorf("pollDelay(rejected) = %v, want %v — a revoked token must not be hammered", got, rejectedBackoff)
	}
	for _, status := range []WatchStatus{WatchOK, WatchLost, WatchNoLine, WatchStopped, WatchOff, WatchNoToken} {
		if got := pollDelay(interval, status); got != interval {
			t.Errorf("pollDelay(%q) = %v, want the configured %v", status, got, interval)
		}
	}
}

func TestPollDelay_neverSpeedsUpASlowerInterval(t *testing.T) {
	slow := rejectedBackoff + time.Minute
	if got := pollDelay(slow, WatchRejected); got != slow {
		t.Errorf("pollDelay = %v, want the configured %v — backoff is a floor, not a target", got, slow)
	}
}

// ------------------------------------------------- the draft-complete signal

// v0ProjectionWithDraft is the §4.1 payload as the dashboard publishes it from
// v3.99.0 on: one additive field, draft_complete, appended last. Built from the
// frozen contract text, not from the dashboard's own fixtures.
const v0ProjectionWithDraft = `{"v": 0, "n": 3, "label": "Line C", "running": true,
 "match_id": "7891234567",
 "game_state": "DOTA_GAMERULES_STATE_HERO_SELECTION",
 "paused": false, "seconds_since_gsi": 0.8, "ts": 1765500000,
 "draft_complete": true}`

// servePayload answers every request with one fixed JSON body.
func servePayload(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// boolPtr is a helper for the *bool draft_complete field.
func boolPtr(b bool) *bool { return &b }

// draftingFeed is a live line whose draft block has been seen and is not
// finished: some of the ten pick slots are still empty.
func draftingFeed() *LineState {
	ls := liveFeed()
	ls.GameState = gameStateHeroSelection
	ls.MatchID = strPtr("7891234567")
	ls.DraftComplete = boolPtr(false)
	return ls
}

// draftDoneFeed is that same line one poll later: the last pick has landed and
// all ten slots are filled. This is the moment the owner wants the flash.
func draftDoneFeed() *LineState {
	ls := draftingFeed()
	ls.DraftComplete = boolPtr(true)
	return ls
}

func strPtr(s string) *string { return &s }

func TestFetchLineState_decodesACompletedDraft(t *testing.T) {
	srv := servePayload(t, v0ProjectionWithDraft)

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 3, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.DraftComplete == nil {
		t.Fatal("DraftComplete = nil, want a decoded true")
	}
	if !*ls.DraftComplete {
		t.Errorf("DraftComplete = %v, want true", *ls.DraftComplete)
	}
}

func TestFetchLineState_decodesAnUnfinishedDraft(t *testing.T) {
	srv := servePayload(t, `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"789",`+
		`"game_state":"DOTA_GAMERULES_STATE_HERO_SELECTION","paused":false,`+
		`"seconds_since_gsi":0.6,"ts":1765500000,"draft_complete":false}`)

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.DraftComplete == nil {
		t.Fatal("DraftComplete = nil, want a decoded false")
	}
	if *ls.DraftComplete {
		t.Errorf("DraftComplete = %v, want false", *ls.DraftComplete)
	}
}

func TestFetchLineState_nullDraftCompleteIsNotFalse(t *testing.T) {
	// null is "no draft block seen for this match", which the contract says is
	// never to be treated as false. A *bool keeps that difference alive; a
	// plain bool would silently flatten it into "the draft is not finished".
	srv := servePayload(t, `{"v":0,"n":1,"label":"Line A","running":true,"match_id":null,`+
		`"game_state":null,"paused":false,"seconds_since_gsi":null,"ts":1765500000,`+
		`"draft_complete":null}`)

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 1, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.DraftComplete != nil {
		t.Errorf("DraftComplete = %v, want nil for an explicit null", *ls.DraftComplete)
	}
}

func TestFetchLineState_decodesTheDashboardsOwnTenKeyExample(t *testing.T) {
	// Byte for byte from the dashboard side's spec §2.2 ("The projection — ten
	// keys, and the absences are a fence"), as it stands on wt/draft-complete.
	// Their §2.2 example carries draft_complete null, which is the case this
	// tray is most likely to get wrong: null must survive as null.
	const theirs = `{"v": 0, "n": 3, "label": "Line C", "running": true,
 "match_id": null,
 "game_state": "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS",
 "paused": false, "seconds_since_gsi": 0.8, "ts": 1765500000,
 "draft_complete": null}`
	srv := servePayload(t, theirs)

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 3, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.DraftComplete != nil {
		t.Errorf("DraftComplete = %v, want nil", *ls.DraftComplete)
	}
	// The other nine keys still land, so a tenth field cannot have shifted
	// anything underneath it.
	if ls.V != 0 || ls.N != 3 || ls.Label != "Line C" || !ls.Running {
		t.Errorf("envelope decoded wrong: %+v", ls)
	}
	if ls.MatchID != nil {
		t.Errorf("MatchID = %q, want nil", *ls.MatchID)
	}
	if ls.GameState != "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS" || ls.Paused {
		t.Errorf("game_state/paused decoded wrong: %+v", ls)
	}
	if ls.SecondsSinceGSI == nil || *ls.SecondsSinceGSI != 0.8 || ls.TS != 1765500000 {
		t.Errorf("seconds_since_gsi/ts decoded wrong: %+v", ls)
	}
}

func TestFetchLineState_absentDraftCompleteDecodesTheSameAsNull(t *testing.T) {
	// An older dashboard does not publish the key at all. That tray must reach
	// exactly the same conclusion as it does for an explicit null — unknown —
	// rather than deciding the draft is unfinished and arming an edge.
	srv := servePayload(t, v0Projection) // the pre-v3.99.0 eight-field payload

	ls, err := fetchLineState(context.Background(), srv.Client(), srv.URL, 3, "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ls.DraftComplete != nil {
		t.Errorf("DraftComplete = %v, want nil when the key is absent", *ls.DraftComplete)
	}
}

// ------------------------------------------- flashing at the last final pick

func TestDecide_flashesWhenTheLastPickCompletesTheDraft(t *testing.T) {
	// The owner's ruling: "'Draft ended' means when all picks and bans have
	// completed" — the false→true edge, not the end of hero selection.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	if state, _ := w.Decide(now, draftingFeed()); state != StateIdle {
		t.Fatalf("state = %q, want idle while picks are still outstanding", state)
	}

	if state, _ := w.Decide(now.Add(2*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("state = %q, want flash at the last final pick", state)
	}
}

func TestDecide_flashLastsFifteenSecondsThenIdles(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed()) // the flash is armed at `now`

	if state, _ := w.Decide(now.Add(14*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("at t+14s: state = %q, want flash", state)
	}
	if state, _ := w.Decide(now.Add(15*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("at t+15s exactly: state = %q, want flash", state)
	}
	if state, _ := w.Decide(now.Add(15100*time.Millisecond), draftDoneFeed()); state != StateIdle {
		t.Errorf("after the 15s flash: state = %q, want idle", state)
	}
}

func TestDecide_lateStartOnAnAlreadyCompletedDraftDoesNotFlash(t *testing.T) {
	// A tray that starts polling after the draft finished has never seen false.
	// null→true is not an edge: flashing here would announce a moment that
	// passed before the tray was even watching.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	unknown := liveFeed()
	unknown.MatchID = strPtr("7891234567")
	unknown.DraftComplete = nil
	if state, _ := w.Decide(now, unknown); state != StateIdle {
		t.Fatalf("state = %q, want idle on a first poll with no draft data", state)
	}

	if state, _ := w.Decide(now.Add(2*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — null→true is not the draft completing", state)
	}
}

func TestDecide_aColdStartOnACompletedDraftDoesNotFlash(t *testing.T) {
	// The very first poll the watcher ever takes already says true. There is no
	// previous value at all, so there is no edge.
	w := NewWatcher(defaultSettings())
	if state, _ := w.Decide(time.Now(), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle on a cold first poll of a completed draft", state)
	}
}

func TestDecide_aStickyCompletedDraftDoesNotReFlash(t *testing.T) {
	// draft_complete stays true for the rest of the match. Every later poll is
	// true→true, which must not re-arm the flash.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed()) // flashing until t+15s

	for i := 1; i <= 5; i++ {
		w.Decide(now.Add(time.Duration(i)*time.Second), draftDoneFeed())
	}
	if state, _ := w.Decide(now.Add(15100*time.Millisecond), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — a sticky true must not re-arm the flash", state)
	}
}

func TestDecide_aNewMatchResetsTheDraftEdge(t *testing.T) {
	// Match A was abandoned mid-draft; match B is already drafted by the time
	// we see it. Carrying A's false across the boundary would fire a flash for
	// a draft this tray never watched.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed()) // match 7891234567, false

	matchB := draftDoneFeed()
	matchB.MatchID = strPtr("7891234599")
	if state, _ := w.Decide(now.Add(2*time.Second), matchB); state != StateIdle {
		t.Errorf("state = %q, want idle — a new match starts the edge detector over", state)
	}
}

func TestDecide_aMatchAbandonedMidDraftThenTheNextFirstSeenDraftedWithANullIDFlashesOnce(t *testing.T) {
	// Today's behaviour at this boundary, pinned on purpose and written down
	// in the user guide. The same two matches as above, except that match B's
	// first poll carries no match_id yet. Null means "not named yet", never "a
	// different match" — the reading that keeps a draft which completes on the
	// very poll its id first appears from losing its flash — so A's false is
	// still the level remembered, and B's true reads as the last pick landing.
	// The light flashes once, for a draft this tray never watched. Telling the
	// two apart needs the dashboard to publish more than it does.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed()) // match 7891234567, picks outstanding, then abandoned

	matchB := draftDoneFeed()
	matchB.MatchID = nil // the next match: not named yet, every pick already in
	if state, _ := w.Decide(now.Add(2*time.Second), matchB); state != StateFlash {
		t.Errorf("state = %q, want flash — the boundary is invisible under a null match_id", state)
	}
	if state, _ := w.Decide(now.Add(20*time.Second), matchB); state != StateIdle {
		t.Errorf("state = %q, want idle — it flashes once, not again", state)
	}
}

func TestDecide_aNewMatchStillFlashesAtItsOwnDraftEnd(t *testing.T) {
	// The reset must forget the old match, not deafen the watcher.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftDoneFeed()) // match 7891234567 finished its draft

	drafting := draftingFeed()
	drafting.MatchID = strPtr("7891234599")
	w.Decide(now.Add(2*time.Second), drafting)

	done := draftDoneFeed()
	done.MatchID = strPtr("7891234599")
	if state, _ := w.Decide(now.Add(4*time.Second), done); state != StateFlash {
		t.Errorf("state = %q, want flash at the new match's own draft end", state)
	}
}

func TestDecide_leavingHeroSelectionFlashesWhenNothingElseWill(t *testing.T) {
	// This trigger is back, as the fallback. A line whose cfg sends no draft
	// block has exactly one observable end-of-draft moment, and it is this one:
	// the same transition the dashboard's action log calls "Draft → Strategy".
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftFeed()) // game_state HERO_SELECTION, no draft_complete key

	if state, _ := w.Decide(now.Add(time.Second), liveFeed()); state != StateFlash {
		t.Errorf("state = %q, want flash — leaving hero selection is the draft ending, as the dashboard sees it", state)
	}
}

func TestDecide_pauseOutranksTheDraftFlash(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed()) // flashing until t+15s

	paused := draftDoneFeed()
	paused.Paused = true
	if state, _ := w.Decide(now.Add(2*time.Second), paused); state != StateSpin {
		t.Errorf("state = %q, want spin — a pause outranks the flash", state)
	}
}

func TestDecide_feedLossSpanningTheLastPickDoesNotFlashLate(t *testing.T) {
	// The gap swallowed the moment the draft completed. Firing on recovery
	// would tell the desk "the draft just ended" a minute late.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now.Add(2*time.Second), nil) // the feed dies mid-draft

	if state, _ := w.Decide(now.Add(90*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — no retroactive flash after a feed gap", state)
	}
}

func TestDecide_stoppedLineDoesNotFlashWhenItsDraftCompletes(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	drafting := draftingFeed()
	drafting.Running = false
	w.Decide(now, drafting)

	done := draftDoneFeed()
	done.Running = false
	if state, _ := w.Decide(now.Add(2*time.Second), done); state != StateIdle {
		t.Errorf("state = %q, want idle — a stopped line has no draft to finish", state)
	}
}

// ------------------------------------------- telling the quiet cases apart

func TestDecide_neverHeardFromDotaIsNotAFeedLoss(t *testing.T) {
	// A line that has never heard from Dota since it started is idle, not
	// broken. The light is the same dark; only the words change.
	w := NewWatcher(defaultSettings())
	ls := liveFeed()
	ls.SecondsSinceGSI = nil
	state, status := w.Decide(time.Now(), ls)
	if state != StateIdle {
		t.Errorf("state = %q, want idle", state)
	}
	if status != WatchNoGameYet {
		t.Errorf("status = %q, want no-game-yet when seconds_since_gsi is null", status)
	}
}

func TestDecide_aFeedThatWasFlowingAndStoppedIsCalledQuiet(t *testing.T) {
	w := NewWatcher(defaultSettings())
	ls := liveFeed()
	ls.SecondsSinceGSI = secs(feedLostAfterSeconds + 1)
	state, status := w.Decide(time.Now(), ls)
	if state != StateIdle {
		t.Errorf("state = %q, want idle", state)
	}
	if status != WatchQuiet {
		t.Errorf("status = %q, want quiet for a feed that went silent", status)
	}
}

func TestWatchDetail_carriesTheQuietSecondsAndTheMissingDraftFlag(t *testing.T) {
	ls := liveFeed()
	ls.SecondsSinceGSI = secs(46.4)
	ls.DraftComplete = nil
	d := watchDetail(ls)
	if d.QuietSeconds != 46.4 {
		t.Errorf("QuietSeconds = %v, want 46.4", d.QuietSeconds)
	}
	if !d.NoDraftData {
		t.Error("NoDraftData = false, want true when draft_complete is null on a live line")
	}
}

func TestWatchDetail_isEmptyForAFailedPoll(t *testing.T) {
	if d := watchDetail(nil); d != (WatchDetail{}) {
		t.Errorf("watchDetail(nil) = %+v, want the zero detail", d)
	}
}

func TestWatchDetail_doesNotFlagDraftDataThatIsFlowing(t *testing.T) {
	if d := watchDetail(draftingFeed()); d.NoDraftData {
		t.Error("NoDraftData = true, want false when draft_complete is present")
	}
}

func TestWatchStatusLabel_saysIdleRatherThanLostWhenNoGameHasStarted(t *testing.T) {
	got := watchStatusLabel(WatchNoGameYet, 1, WatchDetail{})
	if !strings.Contains(got, "idle") || !strings.Contains(got, "no game data yet") {
		t.Errorf("WatchNoGameYet label = %q, want it to say idle — no game data yet", got)
	}
	if strings.Contains(strings.ToUpper(got), "LOST") {
		t.Errorf("WatchNoGameYet label = %q, must not call an idle line lost", got)
	}
}

func TestWatchStatusLabel_saysHowLongTheFeedHasBeenQuiet(t *testing.T) {
	got := watchStatusLabel(WatchQuiet, 1, WatchDetail{QuietSeconds: 46.4})
	if !strings.Contains(got, "feed went quiet") {
		t.Errorf("WatchQuiet label = %q, want it to say the feed went quiet", got)
	}
	if !strings.Contains(got, "46s") {
		t.Errorf("WatchQuiet label = %q, want it to carry how long, e.g. 46s", got)
	}
}

func TestWatchStatusLabel_callsAFailedPollUnreachable(t *testing.T) {
	got := watchStatusLabel(WatchLost, 1, WatchDetail{})
	if !strings.Contains(got, "unreachable") {
		t.Errorf("WatchLost label = %q, want it to say the dashboard is unreachable", got)
	}
}

func TestWatchStatusLabel_namesTheDraftTimingSourceWithoutCallingItBroken(t *testing.T) {
	// A line with no draft block is no longer degraded: it flashes off the game
	// state instead. The label says which of the two is in play and stops there
	// — no warning, and above all no instruction to reinstall anything.
	got := watchStatusLabel(WatchOK, 1, WatchDetail{NoDraftData: true})
	if !strings.Contains(got, "watching") {
		t.Errorf("label = %q, want it to still say the watcher is watching", got)
	}
	if !strings.Contains(got, "draft timing: game state") {
		t.Errorf("label = %q, want it to name the game state as the timing source", got)
	}
	if strings.Contains(got, "reinstall") || strings.Contains(got, "no draft data") {
		t.Errorf("label = %q, must not tell the operator to reinstall the cfg", got)
	}
}

func TestWatchStatusLabel_saysTheTimingComesFromThePicksWhenDraftDataIsFlowing(t *testing.T) {
	got := watchStatusLabel(WatchOK, 1, WatchDetail{})
	if !strings.Contains(got, "draft timing: picks") {
		t.Errorf("label = %q, want it to name the picks as the timing source", got)
	}
	if strings.Contains(got, "reinstall") {
		t.Errorf("label = %q, must not tell the operator to reinstall anything", got)
	}
}

// ------------------------- the draft ending as the dashboard already sees it

// heroSelectionFeed is a live line in the pick/ban phase on a dashboard that
// sends no draft block at all — the pre-v3.99.0 cfg every existing install
// already has. match_id is null, which is what the projection publishes until
// Dota has named the match.
func heroSelectionFeed() *LineState {
	ls := liveFeed()
	ls.N = 1
	ls.GameState = gameStateHeroSelection
	ls.MatchID = nil
	ls.DraftComplete = nil
	return ls
}

// strategyTimeFeed is that same line one poll later: the pick/ban phase is
// over and Dota has moved on. This is the moment the dashboard's own action
// log records as "Draft → Strategy".
func strategyTimeFeed() *LineState {
	ls := heroSelectionFeed()
	ls.GameState = "DOTA_GAMERULES_STATE_STRATEGY_TIME"
	return ls
}

func TestDecide_flashesWhenGameStateLeavesHeroSelectionWithoutDraftData(t *testing.T) {
	// (i) The whole point of the fallback: a line whose cfg predates
	// draft_complete still flashes, off the state transition the dashboard has
	// always been able to see. No reinstall, no draft block, no new field.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	if state, _ := w.Decide(now, heroSelectionFeed()); state != StateIdle {
		t.Fatalf("state = %q, want idle while the pick/ban phase is still running", state)
	}

	armed := now.Add(2 * time.Second)
	if state, _ := w.Decide(armed, strategyTimeFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash when game_state leaves hero selection", state)
	}
	if state, _ := w.Decide(armed.Add(15*time.Second), strategyTimeFeed()); state != StateFlash {
		t.Errorf("at t+15s exactly: state = %q, want flash — the flash is 15s", state)
	}
	if state, _ := w.Decide(armed.Add(15100*time.Millisecond), strategyTimeFeed()); state != StateIdle {
		t.Errorf("after the 15s flash: state = %q, want idle", state)
	}
}

func TestDecide_theDraftCompleteEdgeSpendsTheOnlyFlashOfTheMatch(t *testing.T) {
	// (ii) draft_complete is the earlier and more precise of the two triggers.
	// Having fired, the state transition that follows it in the SAME match is
	// the second trigger, and is ignored.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	w.Decide(now, draftingFeed())
	if state, _ := w.Decide(now.Add(2*time.Second), draftDoneFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash at the last final pick", state)
	}
	if state, _ := w.Decide(now.Add(20*time.Second), draftDoneFeed()); state != StateIdle {
		t.Fatalf("state = %q, want idle once the 15s are spent", state)
	}

	exit := draftDoneFeed() // same match, now leaving hero selection
	exit.GameState = "DOTA_GAMERULES_STATE_STRATEGY_TIME"
	if state, _ := w.Decide(now.Add(22*time.Second), exit); state != StateIdle {
		t.Errorf("state = %q, want idle — one flash per match, and it has been spent", state)
	}
}

func TestDecide_aLateDraftCompleteDoesNotFlashAfterTheStateTransitionDid(t *testing.T) {
	// (iii) The other order, and the harder one: the transition fired while the
	// match was still unnamed, and the draft block only turns up afterwards.
	// Learning the match's name is not the same as a new match.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	w.Decide(now, heroSelectionFeed())
	if state, _ := w.Decide(now.Add(2*time.Second), strategyTimeFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash at the state transition", state)
	}
	if state, _ := w.Decide(now.Add(20*time.Second), strategyTimeFeed()); state != StateIdle {
		t.Fatalf("state = %q, want idle once the 15s are spent", state)
	}

	drafting := strategyTimeFeed()
	drafting.MatchID = strPtr("7891234567")
	drafting.DraftComplete = boolPtr(false)
	w.Decide(now.Add(22*time.Second), drafting)

	done := strategyTimeFeed()
	done.MatchID = strPtr("7891234567")
	done.DraftComplete = boolPtr(true)
	if state, _ := w.Decide(now.Add(24*time.Second), done); state != StateIdle {
		t.Errorf("state = %q, want idle — this match has already had its flash", state)
	}
}

func TestDecide_aNewMatchReArmsTheStateTransitionFlash(t *testing.T) {
	// (iv) The guard is per match, not per tray lifetime.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	matchA := heroSelectionFeed()
	matchA.MatchID = strPtr("7891234567")
	exitA := strategyTimeFeed()
	exitA.MatchID = strPtr("7891234567")
	w.Decide(now, matchA)
	if state, _ := w.Decide(now.Add(2*time.Second), exitA); state != StateFlash {
		t.Fatalf("state = %q, want flash at match A's draft end", state)
	}
	if state, _ := w.Decide(now.Add(20*time.Second), exitA); state != StateIdle {
		t.Fatalf("state = %q, want idle once match A's flash is spent", state)
	}

	matchB := heroSelectionFeed()
	matchB.MatchID = strPtr("7891234599")
	exitB := strategyTimeFeed()
	exitB.MatchID = strPtr("7891234599")
	w.Decide(now.Add(22*time.Second), matchB)
	if state, _ := w.Decide(now.Add(24*time.Second), exitB); state != StateFlash {
		t.Errorf("state = %q, want flash — a new match_id re-arms the one flash", state)
	}
}

func TestDecide_aTrayThatStartsPastBothEventsNeverFlashes(t *testing.T) {
	// (v) No transition to see and no false to edge off. The moment it would be
	// announcing passed before this tray was watching.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	for i := range 5 {
		at := now.Add(time.Duration(i) * 2 * time.Second)
		if state, _ := w.Decide(at, liveFeed()); state != StateIdle {
			t.Fatalf("poll %d: state = %q, want idle", i, state)
		}
	}

	late := liveFeed()
	late.MatchID = strPtr("7891234567")
	late.DraftComplete = boolPtr(true)
	if state, _ := w.Decide(now.Add(12*time.Second), late); state != StateIdle {
		t.Errorf("state = %q, want idle — a late start stays dark for that match", state)
	}
}

func TestDecide_pauseOutranksTheStateTransitionFlash(t *testing.T) {
	// (vi) Unchanged precedence: a pause is the longer-lived truth.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	w.Decide(now, heroSelectionFeed())
	if state, _ := w.Decide(now.Add(2*time.Second), strategyTimeFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash", state)
	}

	paused := strategyTimeFeed()
	paused.Paused = true
	if state, _ := w.Decide(now.Add(4*time.Second), paused); state != StateSpin {
		t.Errorf("state = %q, want spin — a pause outranks the flash", state)
	}
}

func TestDecide_aNullGameStateIsNeverATransition(t *testing.T) {
	// (vii) game_state decodes to "" when the projection publishes null: the
	// dashboard cannot see Dota's state. That is an absence of news, not the
	// draft ending — in either direction, and not across either.
	blank := func() *LineState {
		ls := heroSelectionFeed()
		ls.GameState = ""
		return ls
	}

	t.Run("hero selection to null", func(t *testing.T) {
		w := NewWatcher(defaultSettings())
		now := time.Now()
		w.Decide(now, heroSelectionFeed())
		if state, _ := w.Decide(now.Add(2*time.Second), blank()); state != StateIdle {
			t.Errorf("state = %q, want idle — a null game_state is not somewhere to have gone", state)
		}
	})

	t.Run("null to strategy time", func(t *testing.T) {
		w := NewWatcher(defaultSettings())
		now := time.Now()
		w.Decide(now, blank())
		if state, _ := w.Decide(now.Add(2*time.Second), strategyTimeFeed()); state != StateIdle {
			t.Errorf("state = %q, want idle — we never saw a draft to see it end", state)
		}
	})

	t.Run("a null blip is not resumed across", func(t *testing.T) {
		w := NewWatcher(defaultSettings())
		now := time.Now()
		w.Decide(now, heroSelectionFeed())
		w.Decide(now.Add(2*time.Second), blank())
		if state, _ := w.Decide(now.Add(4*time.Second), strategyTimeFeed()); state != StateIdle {
			t.Errorf("state = %q, want idle — the watcher was blind across the moment it would announce", state)
		}
	})
}

func TestDecide_feedLossSpanningTheStateTransitionDoesNotFlashLate(t *testing.T) {
	// Same rule the draft_complete edge has always had: a gap that swallowed
	// the moment is not a reason to announce it once the feed returns.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	w.Decide(now, heroSelectionFeed())
	w.Decide(now.Add(2*time.Second), nil) // the dashboard goes away mid-draft

	if state, _ := w.Decide(now.Add(90*time.Second), strategyTimeFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle — no retroactive flash after a feed gap", state)
	}
}
