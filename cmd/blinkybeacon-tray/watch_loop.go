package main

import (
	"context"
	"log"
	"net/http"
	"time"
)

// pollTimeout bounds a single poll so a hung dashboard cannot wedge the loop.
// Comfortably under pollInterval, so a dead dashboard is noticed promptly.
const pollTimeout = 1500 * time.Millisecond

// runWatchLoop polls the dashboard and drives the beacon until ctx is done.
//
// The config is re-read every tick, so saving new settings retargets the
// watcher without restarting the app.
func runWatchLoop(ctx context.Context, app *AppState, client *http.Client, cfg func() Config, interval time.Duration) {
	w := NewWatcher()
	lastTarget := ""

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		c := cfg()
		target, err := stateSourceURL(c.DashboardURL, c.LineNumber)
		if err != nil {
			// Not configured (or misconfigured). Stay out of the way entirely:
			// the tray's manual Spin/Flash/Stop must still mean something.
			app.SetWatchStatus(WatchOff)
			lastTarget = ""
		} else {
			if target != lastTarget {
				// Retargeted. Whatever the previous line was doing is not ours
				// any more — start from no assumptions.
				w = NewWatcher()
				lastTarget = target
				log.Printf("Watching %s", target)
			}

			pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
			ls, ferr := fetchLineState(pollCtx, client, c.DashboardURL, c.LineNumber)
			cancel()
			if ferr != nil {
				// A failed poll is a level too: Decide(nil) means "blind".
				ls = nil
			}

			state, status := w.Decide(time.Now(), ls)
			if status != app.WatchStatus() {
				if ferr != nil {
					log.Printf("Dashboard feed lost: %v", ferr)
				} else {
					log.Printf("Dashboard feed: %s", status)
				}
			}
			app.SetWatchStatus(status)
			applyState(app, state)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
