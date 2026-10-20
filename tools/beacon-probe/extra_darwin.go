//go:build darwin

package main

import (
	"fmt"

	hid "github.com/sstallion/go-hid"
)

// platformPortKey is empty on macOS: the enumeration path there is a registry
// entry id that changes on every plug, so nothing in it names the port. The
// location id, read from an open handle below, is what does.
func platformPortKey(info hid.DeviceInfo) string {
	return ""
}

// platformExtra adds the IOKit location id, which encodes the USB controller
// and port the device is on and is the same every time something is plugged
// into that port.
func platformExtra(dev *hid.Device) []string {
	id, err := dev.GetLocationID()
	if err != nil {
		return []string{fmt.Sprintf("location id       : error: %v", err)}
	}
	return []string{fmt.Sprintf("location id       : 0x%08x", id)}
}
