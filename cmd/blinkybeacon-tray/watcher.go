package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// gameStateHeroSelection is the dashboard's own constant for the draft. The
// draft has ENDED when game_state leaves this value — mirroring the "Draft
// Ended" marker DOTA-DASHBOARD logs in gsi/ingest.py::_log_stage_transition.
const gameStateHeroSelection = "DOTA_GAMERULES_STATE_HERO_SELECTION"

// flashDuration is the one timed element in this watcher. Everything else is a
// function of the latest poll.
const flashDuration = 5 * time.Second

// feedLostAfterSeconds is how quiet seconds_since_gsi may get before we stop
// believing the payload. Deliberately well above the ~10s heartbeat Dota sends
// during a pause, so a legitimately paused line is never graded as lost.
const feedLostAfterSeconds = 30.0

// pollInterval is how often the dashboard is asked for the current level.
const pollInterval = 2 * time.Second

// LineState is the subset of GET /line/<N>/state this watcher reads.
// Verified against DOTA-DASHBOARD gsi/view_models.py::_snapshot.
//
// game_state and seconds_since_gsi are both nullable in the real payload:
// a null game_state decodes to "", and seconds_since_gsi is a pointer so
// "never heard from" stays distinguishable from "heard from 0s ago".
type LineState struct {
	Paused          bool     `json:"paused"`
	GameState       string   `json:"game_state"`
	SecondsSinceGSI *float64 `json:"seconds_since_gsi"`
	Running         bool     `json:"running"`
}

// WatchStatus is what the tray says about the dashboard feed.
type WatchStatus string

const (
	WatchOff     WatchStatus = "off"     // no dashboard URL configured
	WatchOK      WatchStatus = "ok"      // polling, feed believed
	WatchLost    WatchStatus = "lost"    // unreachable, rejected, or gone quiet
	WatchStopped WatchStatus = "stopped" // reached, but the line is not running
)

// stateSourceURL builds the URL the watcher polls.
//
// THIS IS THE ONE FUNCTION STAGE B1 SWAPS. B0 deliberately uses the dashboard's
// EXISTING viewer-tier route and needs no dashboard-side change whatsoever.
func stateSourceURL(dashboardURL string, line int) (string, error) {
	raw := strings.TrimSpace(dashboardURL)
	if raw == "" {
		return "", errors.New("dashboard URL is not set")
	}
	if line < 1 {
		return "", fmt.Errorf("line number must be 1 or greater, got %d", line)
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil {
		return "", fmt.Errorf("bad dashboard URL %q: %w", dashboardURL, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("bad dashboard URL %q: no host in it", dashboardURL)
	}
	u.Path = strings.TrimRight(u.Path, "/") + fmt.Sprintf("/line/%d/state", line)
	return u.String(), nil
}

// fetchLineState polls the dashboard once and decodes the current level.
//
// Redirects are NOT followed on purpose: an unauthenticated safe GET is sent to
// the landing page with a 302, and following it would hand us a 200 full of
// HTML — a failed poll wearing a success code.
func fetchLineState(ctx context.Context, client *http.Client, dashboardURL string, line int) (*LineState, error) {
	target, err := stateSourceURL(dashboardURL, line)
	if err != nil {
		return nil, err
	}

	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := noFollow.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polling %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("polling %s: dashboard answered %s", target, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", target, err)
	}

	var ls LineState
	if err := json.Unmarshal(body, &ls); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", target, err)
	}
	return &ls, nil
}

// Watcher turns a stream of polls into a beacon mode.
//
// LEVEL-DRIVEN: every decision is taken from the CURRENT poll. The only memory
// it keeps is the previous game_state (to notice the draft ending at all) and
// the deadline of the 5-second flash. A missed poll, a duplicate poll or a
// restart mid-pause all converge on the right light.
type Watcher struct {
	prevGameState string
	flashUntil    time.Time
}

func NewWatcher() *Watcher { return &Watcher{} }

// Decide returns the beacon mode and the feed status for one poll.
// A nil LineState means the poll itself failed.
func (w *Watcher) Decide(now time.Time, ls *LineState) (StateValue, WatchStatus) {
	if ls == nil || !ls.Running || feedIsStale(ls) {
		// Blind. Drop everything we thought we knew: on recovery we resync from
		// the level we can actually see, rather than firing a transition whose
		// moment has passed. Never a confident wrong light.
		w.prevGameState = ""
		w.flashUntil = time.Time{}
		if ls != nil && !ls.Running {
			return StateIdle, WatchStopped
		}
		return StateIdle, WatchLost
	}

	// The draft ended if it WAS hero selection and now is not. An empty
	// game_state is "unknown", not "not hero selection", so it never fires.
	if w.prevGameState == gameStateHeroSelection &&
		ls.GameState != gameStateHeroSelection && ls.GameState != "" {
		w.flashUntil = now.Add(flashDuration)
	}
	w.prevGameState = ls.GameState

	switch {
	case ls.Paused:
		// A pause outranks the flash: it is the longer-lived truth.
		return StateSpin, WatchOK
	case !now.After(w.flashUntil):
		return StateFlash, WatchOK
	default:
		return StateIdle, WatchOK
	}
}

// feedIsStale reports whether the payload is too old to act on. A null
// seconds_since_gsi means the line has never heard from Dota at all.
func feedIsStale(ls *LineState) bool {
	return ls.SecondsSinceGSI == nil || *ls.SecondsSinceGSI > feedLostAfterSeconds
}

// applyState drives the beacon to the wanted mode, and only when it is not
// already there — the poll loop calls this every couple of seconds, so
// re-issuing the same USB command each time would be pure noise.
func applyState(app *AppState, want StateValue) {
	current, connected, beacon := app.Get()
	if !connected || current == want {
		return
	}

	var err error
	switch want {
	case StateSpin:
		err = beacon.Spin()
	case StateFlash:
		err = beacon.Flash()
	default:
		err = beacon.Stop()
	}
	if err != nil {
		// Same contract as the HTTP handlers: a failed command means the USB
		// device is gone, so drop it and let the reconnect loop find it again.
		app.SetBeacon(nil)
		return
	}
	app.SetState(want)
}

// watchStatusLabel is what the tray says about the dashboard feed. A lost feed
// is called out by name: the beacon goes dark either way, and "dark because the
// dashboard is gone" must not read as "dark because nothing is happening".
func watchStatusLabel(ws WatchStatus) string {
	switch ws {
	case WatchOK:
		return "● Dashboard: watching"
	case WatchLost:
		return "▲ Dashboard: FEED LOST"
	case WatchStopped:
		return "○ Dashboard: line not running"
	default:
		return "○ Dashboard: not configured"
	}
}
