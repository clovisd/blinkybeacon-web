//go:build windows

package main

import (
	"time"

	"github.com/getlantern/systray"
)

// TrayCallbacks are called by the tray UI when the user clicks menu items.
// Beacon control callbacks (OnSpin, OnFlash, OnStop) run the actual USB command.
type TrayCallbacks struct {
	OnSpin  func()
	OnStop  func()
	OnFlash func()
	OnQuit  func()
}

// RunTray starts the system tray. Blocks until the user clicks Quit.
func RunTray(state *AppState, listenAddr string, cbs TrayCallbacks) {
	systray.Run(
		func() { onTrayReady(state, listenAddr, cbs) },
		func() {},
	)
}

func onTrayReady(state *AppState, listenAddr string, cbs TrayCallbacks) {
	systray.SetIcon(iconSiren)
	systray.SetTooltip("BlinkyBeacon — Idle")

	mStatus := systray.AddMenuItem("● Beacon: Disconnected", "")
	mStatus.Disable()

	mHTTP := systray.AddMenuItem("HTTP: "+listenAddr, "")
	mHTTP.Disable()

	systray.AddSeparator()

	mSpin  := systray.AddMenuItem("Spin",  "Start spinning the beacon")
	mFlash := systray.AddMenuItem("Flash", "Start flashing the beacon")
	mStop  := systray.AddMenuItem("Stop",  "Stop the beacon")
	mSpin.Disable()
	mFlash.Disable()
	mStop.Disable()

	systray.AddSeparator()

	mStartup := systray.AddMenuItem("Start at Windows startup", "")
	if IsStartupEnabled() {
		mStartup.Check()
	}
	mQuit := systray.AddMenuItem("Quit", "Stop BlinkyBeacon and exit")

	done := make(chan struct{})

	// Poll AppState every 500ms and sync icon + menu items.
	go func() {
		var lastState StateValue
		var lastConnected bool
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sv, connected, _ := state.Get()
				if sv == lastState && connected == lastConnected {
					continue
				}
				lastState, lastConnected = sv, connected

				switch {
				case !connected:
					systray.SetTooltip("BlinkyBeacon — Disconnected")
					mStatus.SetTitle("● Beacon: Disconnected")
					mSpin.Disable()
					mFlash.Disable()
					mStop.Disable()
				case sv == StateSpin:
					systray.SetTooltip("BlinkyBeacon — Spinning")
					mStatus.SetTitle("● Beacon: Spinning")
					mSpin.Disable()
					mFlash.Enable()
					mStop.Enable()
				case sv == StateFlash:
					systray.SetTooltip("BlinkyBeacon — Flashing")
					mStatus.SetTitle("● Beacon: Flashing")
					mSpin.Enable()
					mFlash.Disable()
					mStop.Enable()
				default: // StateIdle, connected
					systray.SetTooltip("BlinkyBeacon — Idle")
					mStatus.SetTitle("● Beacon: Idle")
					mSpin.Enable()
					mFlash.Enable()
					mStop.Disable()
				}
			case <-done:
				return
			}
		}
	}()

	// Handle menu clicks in a separate goroutine.
	go func() {
		for {
			select {
			case <-mSpin.ClickedCh:
				cbs.OnSpin()
			case <-mFlash.ClickedCh:
				cbs.OnFlash()
			case <-mStop.ClickedCh:
				cbs.OnStop()
			case <-mStartup.ClickedCh:
				if mStartup.Checked() {
					mStartup.Uncheck()
					SetStartupEnabled(false)
				} else {
					mStartup.Check()
					SetStartupEnabled(true)
				}
			case <-mQuit.ClickedCh:
				close(done) // signal both goroutines to exit
				systray.Quit()
				cbs.OnQuit()
				return
			case <-done:
				return
			}
		}
	}()
}
