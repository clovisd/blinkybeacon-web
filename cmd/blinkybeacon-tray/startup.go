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

func IsStartupEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupRegPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(startupRegKey)
	return err == nil
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
