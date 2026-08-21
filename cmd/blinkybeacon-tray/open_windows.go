//go:build windows

package main

import "os/exec"

// openBrowser opens url in the default browser without a console window.
func openBrowser(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
