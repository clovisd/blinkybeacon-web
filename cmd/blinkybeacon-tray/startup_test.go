//go:build windows

package main

import (
	"os"
	"testing"
)

func TestStartup_roundtrip(t *testing.T) {
	// Enable startup, verify it reads back as enabled, then clean up.
	if err := SetStartupEnabled(true); err != nil {
		t.Fatalf("SetStartupEnabled(true) error: %v", err)
	}
	t.Cleanup(func() { SetStartupEnabled(false) }) // ensure cleanup on any failure path
	if !IsStartupEnabled() {
		t.Error("expected IsStartupEnabled()=true after enable")
	}
	// Disable and verify.
	if err := SetStartupEnabled(false); err != nil {
		t.Fatalf("SetStartupEnabled(false) error: %v", err)
	}
	if IsStartupEnabled() {
		t.Error("expected IsStartupEnabled()=false after disable")
	}
}

func TestGetExePath_returnsNonEmpty(t *testing.T) {
	p, err := GetExePath()
	if err != nil {
		t.Fatalf("GetExePath() error: %v", err)
	}
	if p == "" {
		t.Error("expected non-empty exe path")
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("exe path %q does not exist: %v", p, err)
	}
}
