package main

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
)

// pollTimeout bounds a single poll so a hung dashboard cannot wedge the loop.
// Comfortably under pollInterval, so a dead dashboard is noticed promptly.
const pollTimeout = 1500 * time.Millisecond

// runWatchLoop polls the dashboard and drives the beacon until ctx is done.
//
// The config is re-read every tick, so saving new settings retargets the
// watcher — new URL, new line, new token — without restarting the app.
func runWatchLoop(ctx context.Context, app *AppState, client *http.Client, cfg func() Config, interval time.Duration) {
	w := NewWatcher(watcherSettings(cfg()))
	lastTarget := ""
	// The beacon settings the live watcher was built with. Saving new ones has
	// to reach a watcher that was built before they existed, and the honest way
	// to do that is to build a new one — the same thing a retarget does.
	lastSettings := w.set

	for {
		c := cfg()
		app.SetWatchLine(c.LineNumber)
		status := WatchOff
		// The words the tray puts to that status. Recomputed every tick from
		// the poll itself, so it can never go stale behind the status it
		// explains — an unbound or failed poll simply has nothing to add.
		detail := WatchDetail{}

		target, err := stateSourceURL(c.DashboardURL, c.LineNumber)
		unbound := WatchStatus("")
		switch {
		case err != nil:
			// Not configured, or misconfigured.
			unbound = WatchOff
		case strings.TrimSpace(c.APIToken) == "":
			// The read API has no anonymous tier, so a poll without a token can
			// only ever come back 401. Don't make the request: an unbound
			// watcher is quiet, and says why rather than reporting a feed it
			// never asked for as lost.
			unbound = WatchNoToken
		}

		if unbound != "" {
			// An unbound watcher stays out of the way: the tray's manual
			// Spin/Flash/Stop must still mean something. But if we were
			// DRIVING when the binding went away, hand the beacon back dark
			// first — a light left spinning on a level nobody is watching any
			// more is the confident wrong light, reached by the back door.
			// Exactly once, so the manual controls stick afterwards.
			if lastTarget != "" {
				applyState(app, StateIdle)
				lastTarget = ""
			}
			status = unbound
		} else {
			set := watcherSettings(c)
			switch {
			case target != lastTarget:
				// Retargeted. Whatever the previous line was doing is not ours
				// any more — start from no assumptions.
				w = NewWatcher(set)
				lastTarget, lastSettings = target, set
				log.Printf("Watching %s", target)
			case set != lastSettings:
				// The operator saved new beacon settings. Rebuilt rather than
				// patched, because half this watcher's state is about how long
				// a flash that is ALREADY RUNNING has left — and that answer
				// belongs to the settings it was armed under.
				w = NewWatcher(set)
				lastSettings = set
				log.Printf("Beacon settings changed: flash %v, lobby flash %v, pauses %s",
					set.FlashDuration, set.LobbyFlash, set.PauseSide)
			}

			pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
			ls, ferr := fetchLineState(pollCtx, client, c.DashboardURL, c.LineNumber, c.APIToken)
			cancel()

			var state StateValue
			if ferr != nil {
				// A failed poll is a level too: Decide(nil) means "blind", and
				// blind is always dark. The status is the ONLY thing the
				// failure's shape changes — never the light.
				state, _ = w.Decide(time.Now(), nil)
				status = pollFailureStatus(ferr)
			} else {
				state, status = w.Decide(time.Now(), ls)
				detail = watchDetail(ls)
				// Recorded, not acted on: the dashboard's own name for this
				// line, so the tray's bound row can say "Line C" instead of
				// making the operator open Settings to find out. Only from a
				// poll that answered, and stamped with the line it describes.
				app.SetWatchLabel(c.LineNumber, ls.Label)
			}

			if status != app.WatchStatus() {
				if ferr != nil {
					log.Printf("Dashboard feed %s: %v", status, ferr)
				} else {
					log.Printf("Dashboard feed: %s", status)
				}
			}
			applyState(app, state)
		}

		app.SetWatchDetail(detail)
		app.SetWatchStatus(status)

		// A refused token is a standing condition, not a blip: back off rather
		// than pointing a permanent stream of 401s at an internet-facing site.
		timer := time.NewTimer(pollDelay(interval, status))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
