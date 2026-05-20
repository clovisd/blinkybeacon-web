//go:build windows

package main

import (
	"fmt"
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
func RunTray(state *AppState, cbs TrayCallbacks) {
	systray.Run(
		func() { onTrayReady(state, cbs) },
		func() {},
	)
}

func onTrayReady(state *AppState, cbs TrayCallbacks) {
	systray.SetIcon(iconIdle)
	systray.SetTooltip("BlinkyBeacon — Idle")

	mStatus := systray.AddMenuItem("● Beacon: Disconnected", "")
	mStatus.Disable()

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

	// Poll AppState every 500ms and sync icon + menu items.
	go func() {
		var lastState StateValue
		var lastConnected bool
		ticker := time.NewTicker(500 * time.Millisecond)
		for range ticker.C {
			sv, connected, _ := state.Get()
			if sv == lastState && connected == lastConnected {
				continue
			}
			lastState, lastConnected = sv, connected

			switch {
			case !connected:
				systray.SetIcon(iconIdle)
				systray.SetTooltip("BlinkyBeacon — Disconnected")
				mStatus.SetTitle("● Beacon: Disconnected")
				mSpin.Disable()
				mFlash.Disable()
				mStop.Disable()
			case sv == StateSpin:
				systray.SetIcon(iconSpin)
				systray.SetTooltip("BlinkyBeacon — Spinning")
				mStatus.SetTitle("● Beacon: Spinning")
				mSpin.Disable()
				mFlash.Enable()
				mStop.Enable()
			case sv == StateFlash:
				systray.SetIcon(iconFlash)
				systray.SetTooltip("BlinkyBeacon — Flashing")
				mStatus.SetTitle("● Beacon: Flashing")
				mSpin.Enable()
				mFlash.Disable()
				mStop.Enable()
			default: // StateIdle, connected
				systray.SetIcon(iconIdle)
				systray.SetTooltip("BlinkyBeacon — Idle")
				mStatus.SetTitle("● Beacon: Idle")
				mSpin.Enable()
				mFlash.Enable()
				mStop.Disable()
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
				systray.Quit()
				cbs.OnQuit()
			}
		}
	}()

	// Set initial status label.
	sv, connected, _ := state.Get()
	if !connected {
		mStatus.SetTitle("● Beacon: Disconnected")
	} else {
		mStatus.SetTitle(fmt.Sprintf("● Beacon: %s", string(sv)))
	}
}
