package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// inProcessTransport hands each request straight to a handler, in the calling
// goroutine. No socket is involved, which is what lets the loop run inside a
// synctest bubble: there every wait is on the bubble's fake clock, so a test
// can watch ten seconds of polling without spending them.
//
// A request whose context has ended by the time the handler returns fails the
// way a real transport does when it gives up, rather than handing back
// whatever the handler had written so far.
type inProcessTransport struct{ h http.Handler }

func (tr inProcessTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	tr.h.ServeHTTP(rec, r)
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	return rec.Result(), nil
}

// runLoopFor runs the real watch loop inside the current bubble for d of fake
// time, with the settings every build uses, and stops it again.
func runLoopFor(t *testing.T, h http.Handler, cfg Config, d time.Duration) {
	t.Helper()
	app := NewAppState()
	app.SetBeacon(&countingBeacon{})
	client := &http.Client{Transport: inProcessTransport{h}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runWatchLoop(ctx, app, client, func() Config { return cfg }, watcherSettings)
	}()
	time.Sleep(d)
	synctest.Wait()
	cancel()
	<-done
}

// boundConfig is a fresh install bound to a line: defaults everywhere else.
func boundConfig() Config {
	cfg := defaultConfig()
	cfg.DashboardURL, cfg.LineNumber, cfg.APIToken = "http://dashboard.test", 1, testToken
	return cfg
}

func TestWatchLoop_pollsAtTheConfiguredInterval(t *testing.T) {
	cases := []struct {
		name  string
		ms    int
		polls int // in 5.25s of fake time, counting the poll at zero
	}{
		{"the default, two seconds", defaultPollIntervalMs, 3}, // 0, 2, 4
		{"the floor, half a second", minPollIntervalMs, 11},    // 0, 0.5 … 5.0
		{"hand-edited to 750ms", 750, 8},                       // 0, 0.75 … 5.25
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				stub := newStubDashboard(livePayload)
				cfg := boundConfig()
				cfg.PollIntervalMs = tc.ms

				runLoopFor(t, stub, cfg, 5250*time.Millisecond)

				if got := stub.pollCount(); got != tc.polls {
					t.Errorf("poll_interval_ms %d: %d polls in 5.25s, want %d", tc.ms, got, tc.polls)
				}
			})
		})
	}
}

func TestWatchLoop_pollsNeverOverlapAtTheFastestInterval(t *testing.T) {
	// At the 500ms floor the per-poll timeout (1.5s) is longer than the
	// interval. A dashboard that hangs must still see one request at a time:
	// the next poll is half a second after the last one gave up, never half a
	// second after it started.
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		inFlight, most := 0, 0
		var starts []time.Time
		hang := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			inFlight++
			most = max(most, inFlight)
			starts = append(starts, time.Now())
			mu.Unlock()

			<-r.Context().Done() // never answers; the poll's own timeout ends it

			mu.Lock()
			inFlight--
			mu.Unlock()
		})
		cfg := boundConfig()
		cfg.PollIntervalMs = minPollIntervalMs

		runLoopFor(t, hang, cfg, 9*time.Second)

		mu.Lock()
		defer mu.Unlock()
		if most != 1 {
			t.Errorf("%d polls were in flight at once, want 1", most)
		}
		if len(starts) != 5 { // 0, 2, 4, 6, 8
			t.Errorf("%d polls in 9s, want 5 — one every timeout plus interval", len(starts))
		}
		for i := 1; i < len(starts); i++ {
			if gap := starts[i].Sub(starts[i-1]); gap != pollTimeout+500*time.Millisecond {
				t.Errorf("poll %d started %v after the one before, want %v", i, gap, pollTimeout+500*time.Millisecond)
			}
		}
	})
}

func TestWatchLoop_aNewPollIntervalAloneDoesNotRebuildTheWatcher(t *testing.T) {
	// A hand-edited poll_interval_ms reaches the loop with the next settings
	// save, possibly mid-draft. Nothing the watcher remembers depends on the
	// cadence, so it must not be rebuilt for it: a rebuild would cut off the
	// flash that is running and, having forgotten the match's spent flash,
	// fire again when hero selection ends.
	synctest.Test(t, func(t *testing.T) {
		const (
			heroSelection = "DOTA_GAMERULES_STATE_HERO_SELECTION"
			strategyTime  = "DOTA_GAMERULES_STATE_STRATEGY_TIME"
			inProgress    = "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS"
		)
		dash := &sequenceDashboard{seq: []string{
			v0Line("8000000001", heroSelection, false), // t=0
			v0Line("8000000001", heroSelection, true),  // t=2: the last pick, a 15s flash
			v0Line("8000000001", heroSelection, true),  // t=4: first poll at the new interval's settings
			v0Line("8000000001", strategyTime, true),   // t=5: hero selection ends
			v0Line("8000000001", inProgress, true),
		}}
		app := NewAppState()
		b := &countingBeacon{}
		app.SetBeacon(b)

		var mu sync.Mutex
		cfg := boundConfig() // the defaults: 2s polls, a 15s draft flash
		getCfg := func() Config {
			mu.Lock()
			defer mu.Unlock()
			return cfg
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			runWatchLoop(ctx, app, &http.Client{Transport: inProcessTransport{dash}}, getCfg, watcherSettings)
		}()

		time.Sleep(3 * time.Second) // the flash is running
		mu.Lock()
		cfg.PollIntervalMs = 1000
		mu.Unlock()
		time.Sleep(7500 * time.Millisecond) // to t=10.5, still inside the flash
		synctest.Wait()
		state, _, _ := app.Get()
		polls := dash.pollCount()
		cancel()
		<-done

		if state != StateFlash {
			t.Errorf("state = %q at t=10.5s, want flash — the 15s flash from t=2 was cut off", state)
		}
		if b.flashes != 1 || b.stops != 0 {
			t.Errorf("Flash called %d times and Stop %d, want 1 and 0 — one flash, running throughout",
				b.flashes, b.stops)
		}
		if polls != 9 { // 0, 2, 4, then every second to 10
			t.Errorf("%d polls by t=10.5s, want 9 — the new interval still takes effect", polls)
		}
	})
}
