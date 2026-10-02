package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// writeRawConfig plants a config file by hand, so a test can say exactly which
// keys are in it — including the case that matters most, which is the file an
// existing v0.5.0 install already has and none of the new keys are in.
func writeRawConfig(t *testing.T, body string) {
	t.Helper()
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), configFileMode); err != nil {
		t.Fatalf("writing config: %v", err)
	}
}

// ------------------------------------------------------- the three defaults

func TestConfig_beaconLightDefaultsReproduceV050(t *testing.T) {
	// The upgrade promise: an operator who installs v0.6.0 and never opens
	// Settings must see exactly what v0.5.0 did — a 15-second draft flash, no
	// new-lobby flash at all, and a spin for every pause whoever called it.
	withTempConfig(t)
	writeRawConfig(t, `{"addr":"127.0.0.1","port":1337,"line_number":2}`)

	cfg := loadConfig()

	if cfg.FlashSeconds != 15 {
		t.Errorf("FlashSeconds = %d, want v0.5.0's 15", cfg.FlashSeconds)
	}
	if cfg.LobbyFlash {
		t.Error("LobbyFlash = true, want the new-lobby flash off unless asked for")
	}
	if cfg.LobbyFlashSeconds != 10 {
		t.Errorf("LobbyFlashSeconds = %d, want 10", cfg.LobbyFlashSeconds)
	}
	if cfg.PauseSide != pauseSideBoth {
		t.Errorf("PauseSide = %q, want %q", cfg.PauseSide, pauseSideBoth)
	}
	// The keys it did have must survive being defaulted around.
	if cfg.LineNumber != 2 {
		t.Errorf("LineNumber = %d, want the 2 that was in the file", cfg.LineNumber)
	}
}

func TestConfig_beaconLightSettingsRoundTrip(t *testing.T) {
	withTempConfig(t)
	want := defaultConfig()
	want.FlashSeconds = 45
	want.LobbyFlash = true
	want.LobbyFlashSeconds = 600
	want.PauseSide = pauseSideDire
	if err := saveConfig(want); err != nil {
		t.Fatalf("saveConfig: %v", err)
	}

	got := loadConfig()

	if got.FlashSeconds != 45 || got.LobbyFlashSeconds != 600 {
		t.Errorf("durations = %d/%d, want 45/600", got.FlashSeconds, got.LobbyFlashSeconds)
	}
	if !got.LobbyFlash {
		t.Error("LobbyFlash did not survive the round trip")
	}
	if got.PauseSide != pauseSideDire {
		t.Errorf("PauseSide = %q, want %q", got.PauseSide, pauseSideDire)
	}
}

func TestConfig_hasHandEditableNamesForTheThreeSettings(t *testing.T) {
	// The file is documented and hand-edited, so the keys are part of the
	// contract with the operator, not an implementation detail of the struct.
	withTempConfig(t)
	writeRawConfig(t, `{"flash_seconds":30,"lobby_flash":true,"lobby_flash_seconds":5,"pause_side":"radiant"}`)

	cfg := loadConfig()

	if cfg.FlashSeconds != 30 {
		t.Errorf(`"flash_seconds" did not land: FlashSeconds = %d`, cfg.FlashSeconds)
	}
	if !cfg.LobbyFlash {
		t.Error(`"lobby_flash" did not land`)
	}
	if cfg.LobbyFlashSeconds != 5 {
		t.Errorf(`"lobby_flash_seconds" did not land: %d`, cfg.LobbyFlashSeconds)
	}
	if cfg.PauseSide != pauseSideRadiant {
		t.Errorf(`"pause_side" did not land: %q`, cfg.PauseSide)
	}
}

// --------------------------------------------- what a bad file loads as

func TestConfig_outOfRangeDurationsLoadAsTheirDefaults(t *testing.T) {
	// Hand-edited file, so hand-typed mistakes. A beacon that refused to start
	// over a typo would be worse than one that starts with the documented
	// default; the POST path is where a typo is worth arguing about.
	cases := []struct {
		name string
		body string
	}{
		{"zero", `{"flash_seconds":0,"lobby_flash_seconds":0}`},
		{"negative", `{"flash_seconds":-5,"lobby_flash_seconds":-1}`},
		{"past the ten-minute bound", `{"flash_seconds":601,"lobby_flash_seconds":100000}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempConfig(t)
			writeRawConfig(t, tc.body)

			cfg := loadConfig()

			if cfg.FlashSeconds != defaultFlashSeconds {
				t.Errorf("FlashSeconds = %d, want the default %d", cfg.FlashSeconds, defaultFlashSeconds)
			}
			if cfg.LobbyFlashSeconds != defaultLobbyFlashSeconds {
				t.Errorf("LobbyFlashSeconds = %d, want the default %d", cfg.LobbyFlashSeconds, defaultLobbyFlashSeconds)
			}
		})
	}
}

func TestConfig_keepsTheEndsOfTheValidRange(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, `{"flash_seconds":1,"lobby_flash_seconds":600}`)

	cfg := loadConfig()

	if cfg.FlashSeconds != 1 {
		t.Errorf("FlashSeconds = %d, want 1 — the bottom of the range is valid", cfg.FlashSeconds)
	}
	if cfg.LobbyFlashSeconds != 600 {
		t.Errorf("LobbyFlashSeconds = %d, want 600 — the top of the range is valid", cfg.LobbyFlashSeconds)
	}
}

func TestConfig_anUnknownPauseSideLoadsAsBoth(t *testing.T) {
	// Anything that is not one of the three words means the operator did not
	// successfully ask for a filter, and the safe reading of that is the
	// unfiltered light they had before.
	for _, body := range []string{
		`{"pause_side":"Radiant"}`,
		`{"pause_side":"team1"}`,
		`{"pause_side":""}`,
	} {
		withTempConfig(t)
		writeRawConfig(t, body)

		if got := loadConfig().PauseSide; got != pauseSideBoth {
			t.Errorf("%s loaded PauseSide = %q, want %q", body, got, pauseSideBoth)
		}
	}
}

func TestConfig_defaultConfigIsItselfValid(t *testing.T) {
	cfg := defaultConfig()
	if defaultFlashSeconds != 15 || defaultLobbyFlashSeconds != 10 || defaultPauseSide != pauseSideBoth {
		t.Errorf("the documented defaults moved: %d/%d/%q, want 15/10/%q",
			defaultFlashSeconds, defaultLobbyFlashSeconds, defaultPauseSide, pauseSideBoth)
	}
	if !validFlashSeconds(cfg.FlashSeconds) || !validFlashSeconds(cfg.LobbyFlashSeconds) {
		t.Errorf("defaultConfig durations %d/%d are outside the range it enforces",
			cfg.FlashSeconds, cfg.LobbyFlashSeconds)
	}
	if !validPauseSide(cfg.PauseSide) {
		t.Errorf("defaultConfig PauseSide = %q, which it would itself reject", cfg.PauseSide)
	}
}

func TestConfig_validFlashSecondsBoundsTheRangeAtOneAndSixHundred(t *testing.T) {
	for _, n := range []int{0, -1, 601, 100000} {
		if validFlashSeconds(n) {
			t.Errorf("validFlashSeconds(%d) = true, want false", n)
		}
	}
	for _, n := range []int{1, 15, 600} {
		if !validFlashSeconds(n) {
			t.Errorf("validFlashSeconds(%d) = false, want true", n)
		}
	}
}

// ------------------------------------------------------- the poll interval

// v070ConfigFile is a config file exactly as dashboard-v0.7.0 writes it: every
// key that version knows, in its order and indentation, and no
// poll_interval_ms — because that version had none. The light settings are
// off their defaults so a test can see they survive the new key's defaulting.
const v070ConfigFile = `{
  "addr": "127.0.0.1",
  "port": 1337,
  "dashboard_url": "https://dashboard.example.com",
  "line_number": 2,
  "token": "test-bearer-token",
  "flash_seconds": 20,
  "lobby_flash": true,
  "lobby_flash_seconds": 7,
  "pause_side": "dire"
}`

func TestConfig_aV070FilePollsEveryTwoSecondsAsBefore(t *testing.T) {
	// The upgrade promise again: a file written by the previous release has no
	// poll_interval_ms, and must load as the two seconds that release polled at.
	withTempConfig(t)
	writeRawConfig(t, v070ConfigFile)

	cfg := loadConfig()

	if cfg.PollIntervalMs != 2000 {
		t.Errorf("PollIntervalMs = %d, want 2000 — the cadence before the key existed", cfg.PollIntervalMs)
	}
	want := Config{
		Addr: "127.0.0.1", Port: 1337, DashboardURL: "https://dashboard.example.com",
		LineNumber: 2, APIToken: "test-bearer-token",
		FlashSeconds: 20, LobbyFlash: true, LobbyFlashSeconds: 7, PauseSide: pauseSideDire,
		PollIntervalMs: 2000,
	}
	if cfg != want {
		t.Errorf("loaded %+v, want %+v", cfg, want)
	}
}

func TestConfig_aFileWithKeysOmittedPollsEveryTwoSeconds(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, `{"addr":"127.0.0.1","dashboard_url":"https://dashboard.example.com"}`)

	if got := loadConfig().PollIntervalMs; got != 2000 {
		t.Errorf("PollIntervalMs = %d, want the default 2000", got)
	}
}

func TestConfig_readsAHandEditedPollInterval(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, strings.Replace(v070ConfigFile, `"pause_side": "dire"`,
		`"pause_side": "dire",
  "poll_interval_ms": 750`, 1))

	if got := loadConfig().PollIntervalMs; got != 750 {
		t.Errorf("PollIntervalMs = %d, want the 750 typed into the file", got)
	}
}

func TestConfig_keepsTheEndsOfThePollIntervalRange(t *testing.T) {
	for _, ms := range []int{500, 10000} {
		withTempConfig(t)
		writeRawConfig(t, fmt.Sprintf(`{"poll_interval_ms":%d}`, ms))

		if got := loadConfig().PollIntervalMs; got != ms {
			t.Errorf("poll_interval_ms %d loaded as %d — the ends of the range are valid", ms, got)
		}
	}
}

func TestConfig_anOutOfRangePollIntervalLoadsAsTheDefault(t *testing.T) {
	// Same rule as the flash durations: a hand-typed mistake starts the app at
	// the documented default rather than stopping it — or, worse, hammering
	// the dashboard at a cadence nobody meant.
	for _, ms := range []int{0, -1, 1, 499, 10001, 600000} {
		withTempConfig(t)
		writeRawConfig(t, fmt.Sprintf(`{"poll_interval_ms":%d}`, ms))

		if got := loadConfig().PollIntervalMs; got != defaultPollIntervalMs {
			t.Errorf("poll_interval_ms %d loaded as %d, want the default %d", ms, got, defaultPollIntervalMs)
		}
	}
}

func TestConfig_validPollIntervalMsBoundsTheRangeAt500And10000(t *testing.T) {
	for _, n := range []int{-1, 0, 499, 10001} {
		if validPollIntervalMs(n) {
			t.Errorf("validPollIntervalMs(%d) = true, want false", n)
		}
	}
	for _, n := range []int{500, 2000, 10000} {
		if !validPollIntervalMs(n) {
			t.Errorf("validPollIntervalMs(%d) = false, want true", n)
		}
	}
}

func TestConfig_theDefaultPollIntervalIsTwoSeconds(t *testing.T) {
	if defaultPollIntervalMs != 2000 {
		t.Errorf("defaultPollIntervalMs = %d, want 2000", defaultPollIntervalMs)
	}
	if got := defaultConfig().PollIntervalMs; got != defaultPollIntervalMs {
		t.Errorf("defaultConfig().PollIntervalMs = %d, want %d", got, defaultPollIntervalMs)
	}
}
