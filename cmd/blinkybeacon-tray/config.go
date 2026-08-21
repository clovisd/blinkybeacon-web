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
	// FlashSeconds is how long the draft-end flash runs. Was a constant until
	// v0.6.0; the operator's desk, not this source file, is where fifteen
	// seconds turns out to be too long or too short.
	FlashSeconds int `json:"flash_seconds"`
	// LobbyFlash turns on a second, independent flash: one when a new match
	// appears on the line while the game is still before or in the draft.
	// Off by default, so an upgrade changes nothing until it is asked for.
	LobbyFlash bool `json:"lobby_flash"`
	// LobbyFlashSeconds is how long THAT flash runs. Separate from
	// FlashSeconds on purpose: "a lobby appeared" and "the draft just ended"
	// are worth different amounts of attention.
	LobbyFlashSeconds int `json:"lobby_flash_seconds"`
	// PauseSide is which side's pauses spin the light: "both", "radiant" or
	// "dire". Anything else is not a filter the operator asked for, and loads
	// as "both" — see validPauseSide.
	PauseSide string `json:"pause_side"`
}

// The three beacon-light settings, defaulted to v0.5.0's behaviour exactly: a
// fifteen-second draft flash, no new-lobby flash, and a spin for every pause
// whoever it was attributed to. An operator who upgrades and never opens
// Settings must not be able to tell that any of this was added.
const (
	defaultFlashSeconds      = 15
	defaultLobbyFlashSeconds = 10
	defaultPauseSide         = pauseSideBoth
)

// The bounds on both flash durations. One second is the shortest flash that is
// a flash rather than a flicker; ten minutes is far longer than anyone wants a
// strobe running, and is there to stop a typo pinning the light on for a day.
const (
	minFlashSeconds = 1
	maxFlashSeconds = 600
)

// The three words pause_side accepts. Lower case and spelled as the dashboard
// spells the team it attributes a pause to, because that is what they are
// compared against.
const (
	pauseSideBoth    = "both"
	pauseSideRadiant = "radiant"
	pauseSideDire    = "dire"
)

// validFlashSeconds reports whether a flash duration is one we will run.
func validFlashSeconds(n int) bool {
	return n >= minFlashSeconds && n <= maxFlashSeconds
}

// validPauseSide reports whether s is one of the three words. Nothing is
// trimmed or lower-cased here: this is the value that gets compared against
// pause_party off the wire, and a "Radiant " that passed validation would sit
// in the config filtering out every pause forever.
func validPauseSide(s string) bool {
	switch s {
	case pauseSideBoth, pauseSideRadiant, pauseSideDire:
		return true
	}
	return false
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
	// A missing key and a hand-typed nonsense value land in the same place: the
	// documented default. This file is edited in Notepad, and a beacon that
	// refused to start over a stray digit would be worse than one that starts
	// with the fifteen seconds the docs promise. The settings POST is stricter,
	// because there an operator is watching and can be told.
	if !validFlashSeconds(cfg.FlashSeconds) {
		cfg.FlashSeconds = defaultFlashSeconds
	}
	if !validFlashSeconds(cfg.LobbyFlashSeconds) {
		cfg.LobbyFlashSeconds = defaultLobbyFlashSeconds
	}
	if !validPauseSide(cfg.PauseSide) {
		cfg.PauseSide = defaultPauseSide
	}
	return cfg
}

func defaultConfig() Config {
	return Config{
		Addr:              defaultAddr,
		Port:              defaultPort,
		LineNumber:        defaultLineNumber,
		FlashSeconds:      defaultFlashSeconds,
		LobbyFlashSeconds: defaultLobbyFlashSeconds,
		PauseSide:         defaultPauseSide,
	}
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
