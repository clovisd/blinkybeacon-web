//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempLaunchAgents(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "LaunchAgents") // deliberately absent: enabling must create it
	orig := launchAgentsDir
	launchAgentsDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { launchAgentsDir = orig })
	return dir
}

func TestStartup_roundtrip(t *testing.T) {
	dir := withTempLaunchAgents(t)

	if IsStartupEnabled() {
		t.Fatal("expected IsStartupEnabled()=false before anything is written")
	}
	if err := SetStartupEnabled(true); err != nil {
		t.Fatalf("SetStartupEnabled(true): %v", err)
	}
	if !IsStartupEnabled() {
		t.Error("expected IsStartupEnabled()=true after enable")
	}
	data, err := os.ReadFile(filepath.Join(dir, launchAgentLabel+".plist"))
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	exe, _ := GetExePath()
	for _, want := range []string{launchAgentLabel, exe, "<key>RunAtLoad</key>", "<true/>"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("plist lacks %q:\n%s", want, data)
		}
	}

	if err := SetStartupEnabled(false); err != nil {
		t.Fatalf("SetStartupEnabled(false): %v", err)
	}
	if IsStartupEnabled() {
		t.Error("expected IsStartupEnabled()=false after disable")
	}
	// Disabling twice is not an error: the item is a toggle, not a transaction.
	if err := SetStartupEnabled(false); err != nil {
		t.Errorf("second SetStartupEnabled(false): %v", err)
	}
}

func TestLaunchAgentPlist_escapesThePath(t *testing.T) {
	got := string(launchAgentPlist(`/Users/me/Apps & Tools/<beacon>.app/x`))
	if strings.Contains(got, "Apps & Tools") || strings.Contains(got, "<beacon>") {
		t.Errorf("plist did not escape the path:\n%s", got)
	}
	if !strings.Contains(got, "Apps &amp; Tools/&lt;beacon&gt;.app/x") {
		t.Errorf("plist escaping is not what XML wants:\n%s", got)
	}
}

func TestGetExePath_returnsNonEmpty(t *testing.T) {
	p, err := GetExePath()
	if err != nil {
		t.Fatalf("GetExePath() error: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("exe path %q does not exist: %v", p, err)
	}
}
