package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// errNoDashboardURL and errNoToken are the two "nothing is wrong, you just
// have not finished setting up" failures. Sentinels because the settings page
// tells them apart to say which, and matching on message text is how that kind
// of check rots.
var (
	errNoDashboardURL = errors.New("dashboard URL is not set")
	errNoToken        = errors.New("no dashboard API token is set")
)

// linesListPath is the dashboard's index of official lines: the list half of
// the same v0 read API the watcher polls one line of (design spec §4.2).
const linesListPath = "/api/v0/lines"

// lineSummary is one entry of GET /api/v0/lines. The dashboard publishes the
// full per-line projection here, but the settings picker needs exactly three
// facts, and decoding only those keeps the picker independent of fields the
// watcher's LineState may gain.
//
// N is a POINTER for the one reason that matters: `n` is nullable. A dashboard
// older than v3.98.0 has not allocated line numbers and publishes null — which
// into a plain int would decode as 0, an entirely plausible-looking line number
// that binds the beacon to nothing. Null has to stay tellable from a number.
type lineSummary struct {
	V       int    `json:"v"`
	N       *int   `json:"n"`
	Label   string `json:"label"`
	Running bool   `json:"running"`
}

// linesEnvelope is the list response: a version and the lines.
type linesEnvelope struct {
	V     int           `json:"v"`
	Lines []lineSummary `json:"lines"`
}

// fetchLines asks the dashboard which lines it has.
//
// Server-side on purpose (spec §5.1): the browser showing /settings is on
// 127.0.0.1 and the dashboard is across the internet, so a fetch() from the
// page would need CORS the dashboard does not grant. Asking from here is
// CORS-free by construction — and keeps the token off the page entirely.
//
// Same credential discipline as the watcher's poll: bearer in a header, never
// in the URL, and redirects deliberately not followed.
func fetchLines(ctx context.Context, client *http.Client, dashboardURL, token string) ([]lineSummary, error) {
	target, err := dashboardAPIURL(dashboardURL, linesListPath)
	if err != nil {
		return nil, err
	}
	var env linesEnvelope
	if err := getJSON(ctx, client, target, token, &env); err != nil {
		return nil, err
	}
	return env.Lines, nil
}

// getJSON performs one authenticated GET against the dashboard's read API and
// decodes the body. It is the single place the bearer credential is attached,
// so the rules about it live in one place too.
//
// Redirects are NOT followed on purpose: a dashboard that has not shipped the
// read API answers the browser gate with a 302 to the landing page, and
// following it would hand us a 200 full of HTML — a failed request wearing a
// success code. It would also walk the Authorization header to wherever that
// redirect points, which is somebody else's host.
func getJSON(ctx context.Context, client *http.Client, target, token string, out any) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errNoToken
	}

	noFollow := *client
	noFollow.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := noFollow.Do(req)
	if err != nil {
		return fmt.Errorf("polling %s: %w", target, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// The status is kept, not flattened: 401 and 404 are the two failures
		// the operator can actually do something about, and the tray says which.
		// The body is NOT quoted — §4.2 makes it a constant with no information
		// in it, and quoting response bodies into logs is how secrets escape.
		return &pollError{
			Status: resp.StatusCode,
			msg:    fmt.Sprintf("polling %s: dashboard answered %s", target, resp.Status),
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("reading %s: %w", target, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding %s: %w", target, err)
	}
	return nil
}

// lineListTTL is how long the settings page keeps one answer to "which lines
// does this dashboard have?" — a list or a failure alike.
//
// GET /settings has no login, so a web page the operator happens to visit can
// load it over and over. Without this, every load sent the dashboard a request
// carrying the token, and waited up to linesFetchTimeout for it.
const lineListTTL = 5 * time.Second

// lineListCache asks a dashboard for its line list at most once per
// lineListTTL for each dashboard URL and token. Page loads that arrive while a
// request is out wait for it rather than sending their own. The zero value is
// ready to use.
type lineListCache struct {
	// now is the clock answers are timed by; nil means time.Now.
	now func() time.Time

	mu      sync.Mutex
	entries map[lineListKey]*lineListEntry
}

// lineListKey is what an answer belongs to. A different URL or token is a
// different dashboard, or a different view of one, and gets its own request.
type lineListKey struct {
	dashboardURL string
	token        string
}

// lineListEntry is one request: in flight until done is closed, then an
// answer stamped with when it arrived. Every field but done is written once,
// before done is closed, and read only after.
type lineListEntry struct {
	done  chan struct{}
	at    time.Time
	lines []lineSummary
	err   error
}

func (e *lineListEntry) answered() bool {
	select {
	case <-e.done:
		return true
	default:
		return false
	}
}

// get returns the line list for key: the answer from the last lineListTTL if
// there is one, the request already out if there is one, and otherwise the
// answer to a new request made with fetch.
func (c *lineListCache) get(key lineListKey, fetch func() ([]lineSummary, error)) ([]lineSummary, error) {
	c.mu.Lock()
	now := c.clock()
	for k, e := range c.entries {
		if e.answered() && now.Sub(e.at) >= lineListTTL {
			delete(c.entries, k)
		}
	}
	if e, ok := c.entries[key]; ok {
		c.mu.Unlock()
		<-e.done
		return e.lines, e.err
	}
	e := &lineListEntry{done: make(chan struct{})}
	if c.entries == nil {
		c.entries = map[lineListKey]*lineListEntry{}
	}
	c.entries[key] = e
	c.mu.Unlock()

	// Deferred, so that page loads waiting on this request are released even
	// if it never returns normally.
	defer func() {
		e.at = c.clock()
		close(e.done)
	}()
	e.lines, e.err = fetch()
	return e.lines, e.err
}

func (c *lineListCache) clock() time.Time {
	if c.now == nil {
		return time.Now()
	}
	return c.now()
}
