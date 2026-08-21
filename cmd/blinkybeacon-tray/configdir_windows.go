//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// defaultConfigDir is where the config file lives on Windows: beside the
// executable, which is where every existing install already has it.
func defaultConfigDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	return filepath.Dir(exe), nil
}
