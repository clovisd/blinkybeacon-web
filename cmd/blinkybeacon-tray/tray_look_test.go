package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTrayIcon_isPickedByStateConnectionAndBinding(t *testing.T) {
	icons := map[string][]byte{
		"disconnected": iconDisconnected,
		"idle":         iconIdle, "idle, bound": iconIdleBound,
		"spin": iconSpin, "spin, bound": iconSpinBound,
		"flash": iconFlash, "flash, bound": iconFlashBound,
	}
	for _, c := range []struct {
		look trayLook
		want string
	}{
		{trayLook{state: StateIdle}, "disconnected"},
		{trayLook{state: StateSpin}, "disconnected"},
		// Grey stays grey: with no beacon there is no light for a line to drive.
		{trayLook{state: StateIdle, bound: true}, "disconnected"},
		{trayLook{state: StateFlash, bound: true}, "disconnected"},
		{trayLook{state: StateIdle, connected: true}, "idle"},
		{trayLook{state: StateIdle, connected: true, bound: true}, "idle, bound"},
		{trayLook{state: StateSpin, connected: true}, "spin"},
		{trayLook{state: StateSpin, connected: true, bound: true}, "spin, bound"},
		{trayLook{state: StateFlash, connected: true}, "flash"},
		{trayLook{state: StateFlash, connected: true, bound: true}, "flash, bound"},
	} {
		if got := trayIcon(c.look); !bytes.Equal(got, icons[c.want]) {
			got := "something else"
			for name, b := range icons {
				if bytes.Equal(trayIcon(c.look), b) {
					got = name
				}
			}
			t.Errorf("trayIcon(%+v) is the %s icon, want %s", c.look, got, c.want)
		}
	}
}

func TestTrayRedraw_whenAnythingTheIconIsDrawnFromChanges(t *testing.T) {
	// The tray polls twice a second and redraws only on a change. Binding a
	// line while the light sits idle changes nothing but the binding, and it
	// must still redraw — otherwise the mark waits for the light's next move.
	var looks []trayLook
	for _, sv := range []StateValue{StateIdle, StateSpin, StateFlash} {
		for _, connected := range []bool{false, true} {
			for _, bound := range []bool{false, true} {
				looks = append(looks, trayLook{state: sv, connected: connected, bound: bound})
			}
		}
	}
	for _, last := range looks {
		for _, cur := range looks {
			if got, want := trayNeedsRedraw(last, cur), last != cur; got != want {
				t.Errorf("trayNeedsRedraw(%+v, %+v) = %v, want %v", last, cur, got, want)
			}
		}
	}
}

func TestBoundRow_namesALineExactlyWhenTheMenuSaysBound(t *testing.T) {
	// The mark shows exactly when the menu's bound row names a line. Swept over
	// every status the watcher reports, so a new wording for that row cannot
	// silently leave the icon behind.
	statuses := []WatchStatus{WatchOff, WatchNoToken, WatchOK, WatchLost, WatchRejected,
		WatchNoLine, WatchStopped, WatchNoGameYet, WatchQuiet}
	for _, ws := range statuses {
		for _, line := range []int{-1, 0, 1, 3, 99} {
			for _, label := range []string{"", "Line C"} {
				row := boundLineLabel(ws, line, label)
				if row != "Unbound" && !strings.HasPrefix(row, "Bound: ") {
					t.Fatalf("the bound row reads %q: neither Unbound nor a Bound: row", row)
				}
				if got, want := boundRowNamesALine(row), row != "Unbound"; got != want {
					t.Errorf("boundRowNamesALine(%q) = %v, want %v", row, got, want)
				}
			}
		}
	}
}
