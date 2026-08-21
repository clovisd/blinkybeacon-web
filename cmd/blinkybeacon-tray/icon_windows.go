//go:build windows

package main

// trayIconBytes wraps a PNG for the platform's tray: Windows wants an ICO
// container (PNG-in-ICO is fine from Vista on).
func trayIconBytes(pngData []byte) []byte { return wrapPNGInICO(pngData) }
