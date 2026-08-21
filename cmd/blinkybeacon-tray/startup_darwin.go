//go:build darwin

package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Start-at-login on macOS is a per-user LaunchAgent: a plist in
// ~/Library/LaunchAgents that launchd reads at login. Writing the file is the
// whole mechanism — no daemon to talk to, nothing to register — so the menu
// item toggles exactly one file and asks for nothing else.

const launchAgentLabel = "com.blinkybeacon.tray"

// launchAgentsDir is a variable so tests can point it somewhere disposable.
var launchAgentsDir = func() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents"), nil
}

func launchAgentPath() (string, error) {
	dir, err := launchAgentsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, launchAgentLabel+".plist"), nil
}

// launchAgentPlist is the file launchd reads. Only the program path varies,
// and it is user-controlled territory (wherever they put the app), so it is
// escaped for the XML it lands in.
func launchAgentPlist(exe string) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + launchAgentLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>`)
	xml.EscapeText(&b, []byte(exe))
	b.WriteString(`</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`)
	return b.Bytes()
}

func IsStartupEnabled() bool {
	path, err := launchAgentPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func SetStartupEnabled(enabled bool) error {
	path, err := launchAgentPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	exe, err := GetExePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating LaunchAgents: %w", err)
	}
	return os.WriteFile(path, launchAgentPlist(exe), 0o644)
}

func GetExePath() (string, error) {
	return os.Executable()
}
