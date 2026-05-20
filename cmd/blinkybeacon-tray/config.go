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
}

const defaultAddr = "127.0.0.1"
const defaultPort = 1337

func configFilePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine executable path: %w", err)
	}
	return filepath.Join(filepath.Dir(exe), "blinkybeacon-config.json"), nil
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
	return os.WriteFile(path, data, 0644)
}
