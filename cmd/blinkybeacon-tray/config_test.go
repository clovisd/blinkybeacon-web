package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestConfig_defaultsWhenNoFile(t *testing.T) {
	withTempConfig(t)
	cfg := loadConfig()
	if cfg.Addr != defaultAddr || cfg.Port != defaultPort {
		t.Errorf("loadConfig() = %+v, want the defaults", cfg)
	}
}

func TestConfig_roundTrip(t *testing.T) {
	withTempConfig(t)
	if err := saveConfig(Config{Addr: "0.0.0.0", Port: 8080}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	got := loadConfig()
	if got.Addr != "0.0.0.0" || got.Port != 8080 {
		t.Errorf("loadConfig() = %+v, want addr 0.0.0.0 port 8080", got)
	}
}

func TestSaveConfig_keepsTheFileToItsOwner(t *testing.T) {
	// The config decides which interface the control API is bound to; it is
	// the operator's to read and write, nobody else's on the machine.
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes are not the access-control mechanism on Windows")
	}
	withTempConfig(t)
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %#o, want 0600", mode)
	}
}

func TestSaveConfig_tightensAnAlreadyWorldReadableConfig(t *testing.T) {
	// Older builds wrote the file 0644. os.WriteFile does not change an existing
	// file's mode, so saving over it has to do that deliberately.
	if runtime.GOOS == "windows" {
		t.Skip("Unix file modes are not the access-control mechanism on Windows")
	}
	withTempConfig(t)
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"addr":"127.0.0.1","port":1337}`), 0o644); err != nil {
		t.Fatalf("seeding an old config: %v", err)
	}
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := fi.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %#o, want 0600 after re-saving over an old 0644 file", mode)
	}
}

func TestSaveConfig_leavesNoTempFileBehind(t *testing.T) {
	// The write is a fresh file renamed over the old one, so a reader never
	// sees a half-written config. The scratch file must not outlive the save.
	withTempConfig(t)
	if err := saveConfig(Config{Addr: defaultAddr, Port: defaultPort}); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}
	dir, _ := configDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file %s left behind in %s", e.Name(), dir)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "blinkybeacon-config.json")); err != nil {
		t.Errorf("config file missing after save: %v", err)
	}
}
