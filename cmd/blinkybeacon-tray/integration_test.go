package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// These tests talk to a REAL dashboard. They are skipped unless pointed at one,
// so the ordinary `go test ./...` run stays hermetic and offline.
//
// Bring one up with the recipe in Stage A's retro §5, then:
//
//	BLINKY_INT_URL=http://127.0.0.1:4793 \
//	BLINKY_INT_TOKEN=$(cat /tmp/s79-int/token) \
//	BLINKY_INT_COOKIE=$(cat /tmp/s79-int/cookie) \
//	BLINKY_INT_CSRF=$(cat /tmp/s79-int/csrf) \
//	go test -count=1 -run Integration -v ./cmd/blinkybeacon-tray/
//
// The point of these is the one thing a stub can never prove: that the bytes
// this client decodes are the bytes that server actually emits.
//
// ORDER MATTERS, and it is source order: the revoke test is LAST because it
// destroys the token the others need. Re-mint before a second run.
func intEnv(t *testing.T) (base, token string) {
	t.Helper()
	base, token = os.Getenv("BLINKY_INT_URL"), os.Getenv("BLINKY_INT_TOKEN")
	if base == "" || token == "" {
		t.Skip("set BLINKY_INT_URL and BLINKY_INT_TOKEN to run against a real dashboard")
	}
	return base, token
}

// revokeTheKey uses the admin session the recipe minted, exactly as the browser
// would, so the revocation under test is the product's own.
func revokeTheKey(t *testing.T) {
	t.Helper()
	base := os.Getenv("BLINKY_INT_URL")
	cookie, csrf := os.Getenv("BLINKY_INT_COOKIE"), os.Getenv("BLINKY_INT_CSRF")
	if cookie == "" || csrf == "" {
		t.Skip("set BLINKY_INT_COOKIE and BLINKY_INT_CSRF to exercise revocation")
	}
	req, err := http.NewRequest(http.MethodPost, base+"/admin/api/key",
		strings.NewReader(url.Values{"action": {"revoke"}}.Encode()))
	if err != nil {
		t.Fatalf("building the revoke request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Cookie", cookie)
	req.Header.Set("X-CSRF-Token", csrf)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("revoking: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("revoke answered %s", resp.Status)
	}
	t.Logf("revoked the key via POST /admin/api/key (action=revoke) -> %s", resp.Status)
}

func TestIntegration_decodesTheRealDashboardProjection(t *testing.T) {
	base, token := intEnv(t)

	ls, err := fetchLineState(context.Background(), http.DefaultClient, base, 1, token)
	if err != nil {
		t.Fatalf("polling the real dashboard: %v", err)
	}
	pretty, _ := json.Marshal(ls)
	t.Logf("decoded LineState from %s/api/v0/lines/1: %s", base, pretty)

	if ls.V != 0 {
		t.Errorf("v = %d, want 0", ls.V)
	}
	if ls.N != 1 {
		t.Errorf("n = %d, want 1", ls.N)
	}
	if ls.Label == "" {
		t.Error("label is empty — the server always sends a string here")
	}
	if ls.TS == 0 {
		t.Error("ts is 0 — the server sends its own wall clock")
	}

	state, status := NewWatcher().Decide(time.Now(), ls)
	t.Logf("Decide -> state=%q status=%q (running=%v paused=%v game_state=%q seconds_since_gsi=%v)",
		state, status, ls.Running, ls.Paused, ls.GameState, ls.SecondsSinceGSI)
	if state != StateIdle {
		t.Errorf("state = %q — a line with no GSI behind it must be dark", state)
	}
}

func TestIntegration_namesAnUnknownLine(t *testing.T) {
	base, token := intEnv(t)

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})
	cfg := Config{DashboardURL: base, LineNumber: 99999, APIToken: token}
	startWatchLoop(t, app, http.DefaultClient, func() Config { return cfg }, 250*time.Millisecond)

	waitFor(t, "the tray to report the line missing", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchNoLine
	})
	t.Logf("line 99999: status=%q label=%q", app.WatchStatus(),
		watchStatusLabel(app.WatchStatus(), app.WatchLine(), app.WatchDetail()))
}

func TestIntegration_goesDarkAndSaysTokenRejectedAfterARevoke(t *testing.T) {
	base, token := intEnv(t)

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})
	cfg := Config{DashboardURL: base, LineNumber: 1, APIToken: token}
	startWatchLoop(t, app, http.DefaultClient, func() Config { return cfg }, 250*time.Millisecond)

	waitFor(t, "the watcher to reach the real line", func() bool {
		s := app.WatchStatus()
		return s == WatchOK || s == WatchStopped
	})
	t.Logf("bound and polling: status=%q", app.WatchStatus())

	revokeTheKey(t)

	waitFor(t, "the tray to report the token rejected", func() bool {
		state, _, _ := app.Get()
		return state == StateIdle && app.WatchStatus() == WatchRejected
	})
	t.Logf("after revoke: status=%q beacon=%q", app.WatchStatus(), func() StateValue {
		s, _, _ := app.Get()
		return s
	}())
}

// TestIntegration_recoversWhenAFreshTokenIsPasted is the operator's actual
// recovery path, run against the real server at the real 30-second backoff:
// the key was revoked, the tray is dark and saying so, and someone pastes a
// newly minted token into settings. Nothing restarts.
//
// It is deliberately NOT accelerated — the point is to prove the shipped
// backoff does not strand the operator, so it takes up to rejectedBackoff.
func TestIntegration_recoversWhenAFreshTokenIsPasted(t *testing.T) {
	base, stale := intEnv(t)
	fresh := os.Getenv("BLINKY_INT_TOKEN2")
	if fresh == "" {
		t.Skip("set BLINKY_INT_TOKEN2 to a freshly minted token to exercise recovery")
	}

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	var mu sync.Mutex
	cfg := Config{DashboardURL: base, LineNumber: 1, APIToken: stale}
	get := func() Config {
		mu.Lock()
		defer mu.Unlock()
		return cfg
	}
	startWatchLoop(t, app, http.DefaultClient, get, 250*time.Millisecond)

	waitFor(t, "the stale token to be rejected", func() bool {
		return app.WatchStatus() == WatchRejected
	})
	t.Logf("stale token: status=%q — now backing off %s before the next poll",
		app.WatchStatus(), rejectedBackoff)

	mu.Lock()
	cfg.APIToken = fresh // the operator pastes the new token and saves
	mu.Unlock()

	started := time.Now()
	deadline := time.Now().Add(rejectedBackoff + 15*time.Second)
	for time.Now().Before(deadline) {
		if s := app.WatchStatus(); s == WatchOK || s == WatchStopped {
			t.Logf("recovered to status=%q after %s, with no restart",
				s, time.Since(started).Round(time.Second))
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("did not recover within %s; status is still %q", time.Since(started), app.WatchStatus())
}
