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

func TestDecide_stoppedLineIsDarkEvenWhilePausedOnAFreshFeed(t *testing.T) {
	// Stage A's retro §1 clarification (3): a STOPPED line can carry
	// paused:true with a perfectly fresh feed. `running` has to darken the
	// beacon on its own — the pause is real but nobody is playing, so
	// spiralling on it would be a confident wrong light.
	//
	// The pre-existing stopped-line test also had a null seconds_since_gsi,
	// which darkens the beacon by itself, so it never isolated `running`.
	w := NewWatcher()
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

func TestDecide_stoppedLineDoesNotFlashAtDraftEnd(t *testing.T) {
	// The other half: a stopped line must not fire the one-shot either, even
	// though its game_state moved out of hero selection on a fresh feed.
	w := NewWatcher()
	now := time.Now()
	draft := draftFeed()
	draft.Running = false
	w.Decide(now, draft)

	live := liveFeed()
	live.Running = false
	if state, _ := w.Decide(now.Add(time.Second), live); state != StateIdle {
		t.Errorf("state = %q, want idle — a stopped line has no draft to end", state)
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
	lost := watchStatusLabel(WatchLost, 1)
	if !strings.Contains(strings.ToUpper(lost), "LOST") {
		t.Errorf("WatchLost label = %q, want it to say the feed is lost", lost)
	}
}

func TestWatchStatusLabel_givesEveryStatusItsOwnWords(t *testing.T) {
	// Five of these seven mean "the beacon is dark". If any two share a label,
	// the tray has stopped being able to tell the operator which one it is.
	all := []WatchStatus{WatchOff, WatchNoToken, WatchOK, WatchLost, WatchRejected, WatchNoLine, WatchStopped}
	seen := map[string]WatchStatus{}
	for _, status := range all {
		label := watchStatusLabel(status, 1)
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
	got := watchStatusLabel(WatchRejected, 3)
	if !strings.Contains(got, "token rejected") {
		t.Errorf("WatchRejected label = %q, want it to name the rejected token", got)
	}
}

func TestWatchStatusLabel_namesTheMissingLineByNumber(t *testing.T) {
	got := watchStatusLabel(WatchNoLine, 7)
	if !strings.Contains(got, "line 7 not found") {
		t.Errorf("WatchNoLine label = %q, want it to name line 7", got)
	}
}

func TestWatchStatusLabel_saysWhenNoTokenIsSet(t *testing.T) {
	got := watchStatusLabel(WatchNoToken, 1)
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
