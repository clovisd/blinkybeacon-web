//go:build windows

package main

import (
	"fmt"
	"strings"

	hid "github.com/sstallion/go-hid"
	"golang.org/x/sys/windows"
)

// platformPortKey is the piece of a Windows HID path that names the device
// instance: the segment between the second and third '#'. For a device with
// no serial number Windows builds it from the hub and port the device is on,
// so it is what tells one serial-less beacon from another.
func platformPortKey(info hid.DeviceInfo) string {
	parts := strings.Split(info.Path, "#")
	if len(parts) < 3 {
		return ""
	}
	return strings.ToLower(parts[2])
}

// platformExtra adds the container id, which groups every interface of one
// physical USB device under one guid.
func platformExtra(dev *hid.Device) []string {
	var guid windows.GUID
	if err := dev.GetContainerID(&guid); err != nil {
		return []string{fmt.Sprintf("container id      : error: %v", err)}
	}
	return []string{"container id      : " + guid.String()}
}
