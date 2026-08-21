//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultConfigDir is where the config file lives off Windows: the user's
// config directory (~/Library/Application Support/BlinkyBeacon on macOS).
// Beside the executable is wrong here — inside an .app bundle, or in a
// directory the user cannot write to.
func defaultConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine the user config directory: %w", err)
	}
	dir := filepath.Join(base, "BlinkyBeacon")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
