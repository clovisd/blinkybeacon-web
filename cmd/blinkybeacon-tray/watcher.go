package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// gameStateHeroSelection is the dashboard's own constant for the draft. It is
// deliberately NOT what arms the flash, and must not be wired back up as one:
// the owner's ruling of 2026-08-20 is that "'Draft ended' means when all picks
// and bans have completed, not when players have picked their assigned hero
// from the drafted bunch". In Captains Mode this state outlasts the pick/ban
// phase by the whole choose-your-hero stretch, so leaving it fires minutes
// late. draft_complete is the signal; this constant is kept for the record and
// because game_state is still decoded.
const gameStateHeroSelection = "DOTA_GAMERULES_STATE_HERO_SELECTION"

// flashDuration is the one timed element in this watcher. Everything else is a
// function of the latest poll. Fifteen seconds, per the owner: long enough that
// a desk looking away at the last pick still catches it.
const flashDuration = 15 * time.Second

// feedLostAfterSeconds is how quiet seconds_since_gsi may get before we stop
// believing the payload. Deliberately well above the ~10s heartbeat Dota sends
// during a pause, so a legitimately paused line is never graded as lost.
const feedLostAfterSeconds = 30.0

// pollInterval is how often the dashboard is asked for the current level.
const pollInterval = 2 * time.Second

// LineState is the dashboard's v0 line projection: GET /api/v0/lines/{n}, the
// eight fields of the S79 design spec §4.1 plus draft_complete, appended last.
//
// Nullability is load-bearing, not decoration. game_state decodes to "" when
// null; seconds_since_gsi and match_id are pointers so "never heard from Dota"
// stays distinguishable from "heard from 0 seconds ago", and "no match" from
// match 0. draft_complete is a *bool for the same reason and it is the sharpest
// case of all: null means "no draft block has been seen for this match", which
// the contract says is NEVER to be read as false. Flattening it to a plain bool
// would turn "we have no idea" into "the draft is unfinished", and the very
// next true would fire a flash for a draft this tray never watched. A payload
// without the key at all — an older dashboard — decodes to nil, the same as an
// explicit null, which is exactly right.
//
// Only Paused, SecondsSinceGSI, Running, MatchID and DraftComplete feed the
// state machine — V, N, Label, GameState and TS are decoded because the
// contract promises them, and a decoder that silently drops promised fields is
// a decoder that will not notice when the contract moves.
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
	DraftComplete   *bool    `json:"draft_complete"`
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

	// The two halves of what used to be one undifferentiated "FEED LOST".
	// A line with no game on it and a dashboard that has fallen over look
	// identical at the beacon — both dark — so the words have to separate them.
	WatchNoGameYet WatchStatus = "no-game-yet" // running, but Dota has never spoken to it
	WatchQuiet     WatchStatus = "quiet"       // running, was flowing, and has gone silent
)

// WatchDetail is the handful of facts the tray's words need beyond the status
// value itself. A status can say "the feed went quiet"; only the detail can say
// how long, or that no draft data is coming at all.
type WatchDetail struct {
	QuietSeconds float64 // seconds_since_gsi at the last poll
	NoDraftData  bool    // draft_complete was null on a live, running line
}

// watchDetail reads the extra words out of one poll. A nil LineState is a poll
// that failed, which has nothing to add.
func watchDetail(ls *LineState) WatchDetail {
	if ls == nil {
		return WatchDetail{}
	}
	d := WatchDetail{NoDraftData: ls.Running && ls.DraftComplete == nil}
	if ls.SecondsSinceGSI != nil {
		d.QuietSeconds = *ls.SecondsSinceGSI
	}
	return d
}

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

// dashboardAPIURL builds a URL under the dashboard's purpose-built token-gated
// read API (design spec §4.2), which is reachable over the public internet in a
// way the viewer-tier page routes B0 used are not.
//
// The token is NOT part of any URL this returns and never will be: it rides the
// Authorization header, because bearer URLs land in reverse-proxy access logs
// and headers do not.
//
// The operator types the dashboard URL by hand, so this is forgiving about what
// they type — a bare host:port, a trailing slash — and strict about what comes
// out: no host, no URL.
func dashboardAPIURL(dashboardURL, path string) (string, error) {
	raw := strings.TrimSpace(dashboardURL)
	if raw == "" {
		return "", errNoDashboardURL
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
	u.Path = strings.TrimRight(u.Path, "/") + path
	return u.String(), nil
}

// stateSourceURL builds the URL the watcher polls: one line's projection.
func stateSourceURL(dashboardURL string, line int) (string, error) {
	if strings.TrimSpace(dashboardURL) == "" {
		return "", errNoDashboardURL
	}
	if line < 1 {
		return "", fmt.Errorf("line number must be 1 or greater, got %d", line)
	}
	return dashboardAPIURL(dashboardURL, fmt.Sprintf("%s/%d", linesListPath, line))
}

// fetchLineState polls the dashboard once and decodes the current level. The
// token is sent as a bearer credential by getJSON, which is where the rules
// about it live; an empty one is a caller bug, since the read API has no
// anonymous tier and the loop is meant to skip the poll entirely rather than
// send a request that can only ever be refused.
func fetchLineState(ctx context.Context, client *http.Client, dashboardURL string, line int, token string) (*LineState, error) {
	target, err := stateSourceURL(dashboardURL, line)
	if err != nil {
		return nil, err
	}
	var ls LineState
	if err := getJSON(ctx, client, target, token, &ls); err != nil {
		return nil, err
	}
	return &ls, nil
}

// Watcher turns a stream of polls into a beacon mode.
//
// LEVEL-DRIVEN: every decision is taken from the CURRENT poll. The only memory
// it keeps is the previous draft_complete (and the match it belonged to, so the
// edge cannot cross a match boundary) plus the deadline of the 15-second flash.
// A missed poll, a duplicate poll or a restart mid-pause all converge on the
// right light.
type Watcher struct {
	prevMatchID       string
	prevDraftComplete *bool
	flashUntil        time.Time
}

func NewWatcher() *Watcher { return &Watcher{} }

// Decide returns the beacon mode and the feed status for one poll.
// A nil LineState means the poll itself failed.
func (w *Watcher) Decide(now time.Time, ls *LineState) (StateValue, WatchStatus) {
	if ls == nil || !ls.Running || feedIsStale(ls) {
		// Blind. Drop everything we thought we knew: on recovery we resync from
		// the level we can actually see, rather than firing a transition whose
		// moment has passed. Never a confident wrong light.
		w.forget()
		// The light is the same dark for all four of these. The status is the
		// only thing that separates "nothing is happening on this line" from
		// "the dashboard has stopped answering", and the operator needs that.
		switch {
		case ls == nil:
			return StateIdle, WatchLost
		case !ls.Running:
			return StateIdle, WatchStopped
		case ls.SecondsSinceGSI == nil:
			return StateIdle, WatchNoGameYet
		default:
			return StateIdle, WatchQuiet
		}
	}

	w.forgetIfNewMatch(ls)
	if w.draftJustCompleted(ls) {
		w.flashUntil = now.Add(flashDuration)
	}
	w.rememberDraft(ls)

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

// draftJustCompleted reports the false→true edge of draft_complete inside one
// match: the moment the last final pick of the pick/ban phase lands and all ten
// pick slots are filled. That edge, and nothing else, arms the flash.
//
// null is not false. A tray that started polling after the draft had already
// finished sees null→true — or nothing→true on its very first poll — and must
// stay dark, because the moment it would be announcing has already passed.
func (w *Watcher) draftJustCompleted(ls *LineState) bool {
	if ls.DraftComplete == nil || !*ls.DraftComplete {
		return false
	}
	return w.prevDraftComplete != nil && !*w.prevDraftComplete
}

// forgetIfNewMatch drops the previous draft value when the line has moved on to
// a different match, so a match abandoned mid-draft cannot lend its `false` to
// the next match's already-finished draft.
//
// Only a match_id we can actually see counts — the contract's "where present".
// The projection publishes null for a match it cannot name yet, and treating
// null as "a different match" would reset the detector in the middle of the
// very draft it is watching.
func (w *Watcher) forgetIfNewMatch(ls *LineState) {
	if ls.MatchID == nil || *ls.MatchID == "" {
		return
	}
	if w.prevMatchID != "" && *ls.MatchID != w.prevMatchID {
		w.prevDraftComplete = nil
	}
	w.prevMatchID = *ls.MatchID
}

// rememberDraft stores this poll's draft_complete by value. By value, not by
// pointer: the watcher's memory must not be rewritable by whoever still holds
// the payload we decoded it from.
func (w *Watcher) rememberDraft(ls *LineState) {
	if ls.DraftComplete == nil {
		w.prevDraftComplete = nil
		return
	}
	v := *ls.DraftComplete
	w.prevDraftComplete = &v
}

// forget drops everything the watcher believed. Called whenever it goes blind.
func (w *Watcher) forget() {
	w.prevMatchID = ""
	w.prevDraftComplete = nil
	w.flashUntil = time.Time{}
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

// boundLineLabel is the tray's read-only row naming which line the beacon
// follows. It answers the first question anyone asks of a beacon on a shelf —
// "which line is that one?" — without making them open Settings to find out.
//
// The dashboard's own name for the line is used whenever we have been told it,
// so the row matches what the operator reads on their screen. Until the first
// poll answers, the configured number is all we honestly have.
func boundLineLabel(ws WatchStatus, line int, label string) string {
	if ws == WatchOff || line < 1 {
		return "Unbound"
	}
	if label == "" {
		return fmt.Sprintf("Bound: line %d (unnamed)", line)
	}
	return "Bound: " + label
}

// watchStatusLabel is what the tray says about the dashboard feed, for the
// line currently bound. Every dark-beacon reason is called out by name: the
// beacon goes dark for all of them, and "dark because the token was revoked"
// must not read as "dark because nothing is happening".
//
// That is why an idle line no longer says FEED LOST. Between games the tray
// used the same alarming words for "no game on this line yet" as for "the
// dashboard fell over", which taught the operator to ignore both. Three
// distinct sentences now: idle, went quiet, unreachable.
func watchStatusLabel(ws WatchStatus, line int, d WatchDetail) string {
	switch ws {
	case WatchOK:
		label := "● Dashboard: watching"
		if d.NoDraftData {
			// Said BEFORE the draft rather than after: this is the operator's
			// only warning that the beacon cannot flash at the last pick, and
			// it names the one thing that fixes it.
			label += " · no draft data (reinstall cfg)"
		}
		return label
	case WatchNoGameYet:
		return "○ Dashboard: idle — no game data yet"
	case WatchQuiet:
		return fmt.Sprintf("▲ Dashboard: feed went quiet (%.0fs)", d.QuietSeconds)
	case WatchLost:
		return "▲ Dashboard: unreachable"
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
