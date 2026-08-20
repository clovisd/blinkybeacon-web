package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds runtime settings persisted to blinkybeacon-config.json
// in the same directory as the executable.
type Config struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
	// DashboardURL is the base URL of the LIVE Dashboard, e.g.
	// "http://192.168.1.50:8080". Empty means the watcher stays off and the
	// beacon is yours to drive by hand from the tray.
	DashboardURL string `json:"dashboard_url"`
	// LineNumber is the dashboard line to watch (the N in /line/N/).
	LineNumber int `json:"line_number"`
	// APIToken is the bearer token the dashboard admin mints under
	// Settings -> Integrations. The read API has no anonymous tier, so an
	// empty token means the watcher does not poll at all.
	//
	// Stored under "token" rather than "api_token" because this file is
	// hand-editable and documented: short beats descriptive at the keyboard.
	APIToken string `json:"token"`
}

const defaultAddr = "127.0.0.1"
const defaultPort = 1337
const defaultLineNumber = 1

// configDir is where blinkybeacon-config.json lives: next to the executable.
// A variable so tests can point it somewhere disposable.
var configDir = func() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	return filepath.Dir(exe), nil
}

func configFilePath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "blinkybeacon-config.json"), nil
}

// loadConfig reads the config file and returns its contents, or defaults if the
// file does not exist or cannot be parsed.
func loadConfig() Config {
	cfg := defaultConfig()
	path, err := configFilePath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig()
	}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	if cfg.LineNumber < 1 {
		cfg.LineNumber = defaultLineNumber
	}
	return cfg
}

func defaultConfig() Config {
	return Config{Addr: defaultAddr, Port: defaultPort, LineNumber: defaultLineNumber}
}

// configFileMode keeps the config file to its owner. It holds APIToken, a
// bearer credential for the dashboard — which persists its own copy of the same
// token at 0600 — so world-readable is not good enough any more.
const configFileMode = 0o600

// saveConfig writes cfg to the config file next to the executable.
func saveConfig(cfg Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Write a NEW owner-only file and rename it over the old one, rather than
	// truncating the old one in place.
	//
	// os.WriteFile's mode only applies when it CREATES the file, so writing
	// over the 0644 config a pre-token build left behind would put the
	// credential in a world-readable file and only narrow it afterwards —
	// a window any local process can sit and wait for. A fresh 0600 file plus
	// an atomic rename has no such window, and as a bonus no reader ever sees
	// a half-written config.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".blinkybeacon-config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // a no-op once the rename below has succeeded

	if err := tmp.Chmod(configFileMode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
