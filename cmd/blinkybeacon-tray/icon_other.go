//go:build !windows

package main

// trayIconBytes wraps a PNG for the platform's tray: macOS (and Linux) take
// the PNG as it is.
func trayIconBytes(pngData []byte) []byte { return pngData }
