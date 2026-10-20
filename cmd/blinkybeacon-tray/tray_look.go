package main

import "strings"

// trayLook is everything the tray icon is drawn from: what the light is
// doing, whether the beacon is plugged in, and whether a dashboard line is
// driving it.
type trayLook struct {
	state     StateValue
	connected bool
	bound     bool
}

// trayIcon is the icon for a look. A bound line adds the mark to the idle,
// spin and flash sirens; a disconnected beacon is grey either way.
func trayIcon(l trayLook) []byte {
	switch {
	case !l.connected:
		return iconDisconnected
	case l.state == StateSpin:
		if l.bound {
			return iconSpinBound
		}
		return iconSpin
	case l.state == StateFlash:
		if l.bound {
			return iconFlashBound
		}
		return iconFlash
	default: // StateIdle, connected
		if l.bound {
			return iconIdleBound
		}
		return iconIdle
	}
}

// trayNeedsRedraw reports whether the tray must redraw its icon. Every field
// counts: binding a line while the light sits idle changes nothing else, and
// the mark must not wait for the light's next move to appear.
func trayNeedsRedraw(last, cur trayLook) bool {
	return cur.state != last.state || cur.connected != last.connected || cur.bound != last.bound
}

// boundRowNamesALine reports whether the tray menu's bound row (see
// boundLineLabel) names a line. That is exactly when the icon carries the mark,
// so the icon and the menu never disagree.
func boundRowNamesALine(row string) bool {
	return strings.HasPrefix(row, "Bound: ")
}
