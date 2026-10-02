//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	startupRegPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	startupRegKey  = "BlinkyBeacon"
)

// IsStartupEnabled reports whether Windows will start THIS program at login:
// the Run value exists and names this executable. A value another copy left
// behind — the program under an older file name, or in a folder it has since
// left — reads as off, because nothing it names will start. Ticking the item
// then rewrites the value to this program.
func IsStartupEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	stored, _, err := k.GetStringValue(startupRegKey)
	if err != nil {
		return false
	}
	exe, err := GetExePath()
	if err != nil {
		return false
	}
	return sameWindowsPath(stored, exe)
}

func SetStartupEnabled(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enabled {
		err = k.DeleteValue(startupRegKey)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	exe, err := GetExePath()
	if err != nil {
		return err
	}
	return k.SetStringValue(startupRegKey, exe)
}

func GetExePath() (string, error) {
	return os.Executable()
}
