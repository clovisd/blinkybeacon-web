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

// LineState is the dashboard's v0 line projection: GET /api/v0/lines/{n},
// all eight fields of the S79 design spec §4.1.
//
// Nullability is load-bearing, not decoration. game_state decodes to "" when
// null; seconds_since_gsi and match_id are pointers so "never heard from Dota"
// stays distinguishable from "heard from 0 seconds ago", and "no match" from
// match 0. Only Paused, GameState, SecondsSinceGSI and Running feed the state
// machine — V, N, Label, MatchID and TS are decoded because the contract
// promises them, and a decoder that silently drops promised fields is a
// decoder that will not notice when the contract moves.
type LineState struct {
	V               int      `json:"v"`
	N               int      `json:"n"`
	Label           string   `json:"label"`
	Running         bool     `json:"running"`
	MatchID         *string  `json:"match_id"`
	GameState       string   `json:"game_state"`
	Paused          bool     `json:"paused"`
	SecondsSinceGSI *float64 `json:"seconds_since_gsi"`
	TS              int64    `json:"ts"`
}

// WatchStatus is what the tray says about the dashboard feed.
type WatchStatus string

const (
	WatchOff      WatchStatus = "off"      // no dashboard URL configured
	WatchNoToken  WatchStatus = "no-token" // URL set, but no API token: nothing to poll with
	WatchOK       WatchStatus = "ok"       // polling, feed believed
	WatchLost     WatchStatus = "lost"     // unreachable or gone quiet
	WatchRejected WatchStatus = "rejected" // reached, but the token was refused (401)
	WatchNoLine   WatchStatus = "no-line"  // reached and authorised, but no such line (404)
	WatchStopped  WatchStatus = "stopped"  // reached, but the line is not running
)

// rejectedBackoff is how long the watcher waits between polls once the
// dashboard has refused the token. A revoked token is a standing condition,
// not a blip: retrying at the normal cadence would turn every stale install
// into a permanent stream of 401s against an internet-facing origin.
//
// A var, not a const, so tests can exercise recovery without a 30s wait.
var rejectedBackoff = 30 * time.Second

// pollDelay is how long to wait before the next poll, given what the last one
// found. Everything except a refused token polls at the configured cadence.
func pollDelay(interval time.Duration, status WatchStatus) time.Duration {
	if status == WatchRejected && interval < rejectedBackoff {
		return rejectedBackoff
	}
	return interval
}

// pollError is a poll that reached the dashboard and was answered with a
// status we can name. The loop turns the status into a tray label, so
// "your token was revoked" never has to masquerade as "the network is down".
//
// Its message deliberately carries the URL and the HTTP status and nothing
// else: every poll error is logged, and the token must never be.
type pollError struct {
	Status int
	msg    string
}

func (e *pollError) Error() string { return e.msg }

// pollFailureStatus maps a failed poll onto what the tray should say. Anything
// that is not a recognised HTTP answer is the pre-existing feed-lost path.
func pollFailureStatus(err error) WatchStatus {
	var pe *pollError
	if errors.As(err, &pe) {
		switch pe.Status {
		case http.StatusUnauthorized:
			return WatchRejected
		case http.StatusNotFound:
			return WatchNoLine
		}
	}
	return WatchLost
}

// stateSourceURL builds the URL the watcher polls: the dashboard's purpose-built
// token-gated read API (design spec §4.2), which is reachable over the public
// internet in a way the viewer-tier page routes B0 used are not.
//
// The token is NOT part of this URL and never will be: it rides the
// Authorization header, because bearer URLs land in reverse-proxy access logs
// and headers do not.
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
	u.Path = strings.TrimRight(u.Path, "/") + fmt.Sprintf("/api/v0/lines/%d", line)
	return u.String(), nil
}

// fetchLineState polls the dashboard once and decodes the current level. The
// token is sent as a bearer credential; an empty one is a caller bug, since the
// read API has no anonymous tier and the loop is meant to skip the poll
// entirely rather than send a request that can only ever be refused.
//
// Redirects are NOT followed on purpose: a dashboard that has not shipped the
// read API answers the browser gate with a 302 to the landing page, and
// following it would hand us a 200 full of HTML — a failed poll wearing a
// success code.
func fetchLineState(ctx context.Context, client *http.Client, dashboardURL string, line int, token string) (*LineState, error) {
	target, err := stateSourceURL(dashboardURL, line)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("no dashboard API token is set")
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
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := noFollow.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polling %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// The status is kept, not flattened: 401 and 404 are the two failures
		// the operator can actually do something about, and the tray says which.
		// The body is NOT quoted — §4.2 makes it a constant with no information
		// in it, and quoting response bodies into logs is how secrets escape.
		return nil, &pollError{
			Status: resp.StatusCode,
			msg:    fmt.Sprintf("polling %s: dashboard answered %s", target, resp.Status),
		}
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

// watchStatusLabel is what the tray says about the dashboard feed, for the
// line currently bound. Every dark-beacon reason is called out by name: the
// beacon goes dark for all of them, and "dark because the token was revoked"
// must not read as "dark because nothing is happening".
func watchStatusLabel(ws WatchStatus, line int) string {
	switch ws {
	case WatchOK:
		return "● Dashboard: watching"
	case WatchLost:
		return "▲ Dashboard: FEED LOST"
	case WatchRejected:
		return "▲ Dashboard: token rejected"
	case WatchNoLine:
		return fmt.Sprintf("▲ Dashboard: line %d not found", line)
	case WatchNoToken:
		return "○ Dashboard: no token set"
	case WatchStopped:
		return "○ Dashboard: line not running"
	default:
		return "○ Dashboard: not configured"
	}
}
