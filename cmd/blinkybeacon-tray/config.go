package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds runtime settings persisted to blinkybeacon-config.json —
// beside the executable on Windows, in the user's config directory elsewhere
// (see defaultConfigDir).
type Config struct {
	Addr string `json:"addr"`
	Port int    `json:"port"`
}

const defaultAddr = "127.0.0.1"
const defaultPort = 1337

// configDir is where the config file lives, per platform.
// A variable so tests can point it somewhere disposable.
var configDir = defaultConfigDir

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
	cfg := Config{Addr: defaultAddr, Port: defaultPort}
	path, err := configFilePath()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{Addr: defaultAddr, Port: defaultPort}
	}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	if cfg.Port == 0 {
		cfg.Port = defaultPort
	}
	return cfg
}

// configFileMode keeps the config file to its owner. It decides which interface
// the control API is bound to, and forks of this app keep credentials in it.
const configFileMode = 0o600

// saveConfig writes cfg to the config file.
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
	// over a 0644 config an older build left behind would keep it world-
	// readable. A fresh 0600 file plus an atomic rename has no such window,
	// and as a bonus no reader ever sees a half-written config.
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
