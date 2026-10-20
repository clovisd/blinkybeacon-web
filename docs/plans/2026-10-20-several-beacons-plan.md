# Several beacons, each following one or more lines — the S16 build plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The tray drives every Farming Simulator beacon plugged into one PC, each beacon following
zero or more dashboard lines with its own light settings, named and arranged on the settings page,
listed and addressable over HTTP, with a `dashboard-v0.8.0` config file loading untouched as one
beacon on one line.

**Architecture:** Today's one-line, one-light state machine (`Watcher.Decide`) is kept whole and run
once per (beacon, line) pair; a beacon shows the rank maximum of its pairs' answers. A registry
replaces `AppState`'s single beacon: one entry per record (configured or seen-in-memory), each with
its public id, match key, open handle, light state and identify deadline. A device manager goroutine
enumerates the USB bus every 10 s through a two-method seam, so the suite runs on Linux against a
fake bus. The config file gains a `beacons` list behind one writer (`configStore.Update`); the five
old top-level keys are mirrored from the first beacon.

**Tech Stack:** Go 1.25 (`go.mod`; the toolchain here is 1.27), `github.com/sstallion/go-hid`
v0.14.1, `github.com/getlantern/systray` v1.2.2, `net/http`, `testing/synctest`. No new dependency:
`go.mod` does not change.

**Spec:** `docs/specs/2026-10-20-several-beacons.md` at `1e4784c` (revision 3, READY FOR PLAN). The
plan argues from the design; a task's implementer reads both. Where the plan names a design section
(§n) it carries that section out; where it departs from the design's own build order (§15), it says
why.

## How to read this plan

- **Tasks** are in build order, T0 to T10, with two split out of the design's T0 and T5 so that the
  probe's result blocks the least work: T0b (fix the identity mode) and T5b (the real USB source and
  the final `main.go`) are the only tasks that wait on the probe file. The design's T3 is one task
  here but its mechanical test move is its own tiny task, T3m, so the two large wave-2 tasks never
  share a test file.
- **Every code step shows the code.** Types, signatures, tests and their disarms are written out;
  an implementer transcribes and fills bodies the plan describes in words only where the body is
  a straight loop over data the plan has already typed.
- **Red first.** Each test names the disarm that must turn it red: the implementer runs the test
  against the disarm before writing the implementation, sees the failure, then implements.
- **Measured, not guessed.** Sizes come from counting the code: the files named carry the line
  counts measured at `1e4784c` (`wc -l`), and every "N calls" is a `grep -c` from that commit.

## Global constraints

Copied from the design and the project rules; every task's requirements include these.

- The suite is `go test -count=1 -race ./cmd/blinkybeacon-tray/`, offline, on Linux, with the fake
  bus. `go test ./...` stays red on Linux (`pkg/fsbeacon` needs `libudev.h`, which this box lacks:
  the `.so` is present, the header is not). The library's own test runs on the macOS CI leg.
- `go vet ./cmd/blinkybeacon-tray/` clean on Linux and under
  `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc`; `gofmt -l` empty. Vet the
  tray package only: upstream's `cmd/fsbeacon/commands/root.go` is a documented quirk.
- Files that import `pkg/fsbeacon` or `systray` carry `//go:build windows || darwin` (as
  `main.go:1`, `tray.go:1`, `beacon_check.go:1` do). Everything the suite exercises must compile on
  Linux: the seam, the registry, the manager, the loop, the HTTP server, the settings page.
- **`/spin`, `/flash`, `/stop` and `/status` keep their paths and their no-parameter replies, status
  and body, exactly** (`server.go:76-131`). The one addition on the three control routes is the
  optional `beacon` query parameter (owner, 2026-10-20). `/status` ignores every query parameter.
- Public ids are `b` and a decimal number with no leading zero (`b1`, `b2`, …), assigned from
  `beacon_seq`, never reused after a Forget; the match key (`sn:…` or `port:…`) never travels in a
  URL or a form.
- Names: trimmed, 1–40 characters counted in runes, unique among configured beacons by exact match,
  never of the shape `b` + digits in any letter case.
- Light settings per beacon, with today's bounds: flash seconds 1–600 (`config.go:63-66`),
  `pause_side` one of `both`/`radiant`/`dire`; line numbers 1–99 on the form (`settings_handler.go:256`),
  ≥ 1 in the file.
- Timings that are the design's: enumerate every 10 s (`main.go:91`); the library's keepalive every
  5 s (`usbhid.go:70`); `pollTimeout` 1.5 s; Identify 2 s; 429 backoff `Retry-After` capped at
  30 s, 10 s when absent or unreadable; the refused-token backoff 30 s unchanged.
- The config file stays owner-only and atomically written (`config.go:186-227`); nothing but
  `configStore.Update` calls `saveConfig` after T2 lands.
- Product text and code comments carry no process vocabulary: no task numbers, ruling codes,
  review-thread references or exception text. Commit messages path-scoped in the branch's style
  (`feat(tray): …`, `test(tray): …`, `docs: …`), no attribution lines.
- Branch `wt/s16-plan` carries the design; the build runs on **`wt/s16-build`** cut from the merged
  plan on `dashboard`; `docs/ROADMAP-STAGES.md` and `docs/PUNCHLIST.md` are not touched (they live
  on `main`; the master moves rows at the ship).

## The task table

| # | Task | Builds | Seat | Size | Wave |
|---|---|---|---|---|---|
| T0 | Identity functions, both modes; the public-id rules | `identity.go`, `identity_test.go` | `cwd-implementer-opus5` | S | 0 |
| T1 | Library: enumerate, open by path, `Info`, `LastAttemptOK`, `LocationID` | `pkg/fsbeacon/usbhid.go`, `pkg/fsbeacon/usbhid_darwin.go`, `pkg/fsbeacon/usbhid_test.go`, `.github/workflows/build.yml` | `cwd-implementer-opus5` | S | 0 |
| T2 | Config: `BeaconRecord`, the `beacons` list, `beacon_seq`, migration, the mirror, loader forgiveness, `configStore` | `config.go`, `config_store.go`, `config_test.go`, `config_store_test.go` | `cwd-implementer-opus5` | M | 0 |
| T3m | Mechanical: the settings tests move out of `watch_loop_test.go` | `watch_loop_test.go`, `settings_handler_test.go` | general-purpose, `model: "sonnet"` | S (moves only) | 0 |
| T3 | The registry, the aggregate, the seam types, per-line watch state; `main.go` and `server.go` compat; the test-setup sweep | `beacons.go` (from `state.go`), `beacons_test.go` (from `state_test.go`), `devices.go`, `watcher.go`, `main.go`, `server.go`, `settings_handler.go` (tail of `handlePost` only), `watch_loop.go` and `tray.go` (compat lines), six test files' setup lines | `cwd-implementer-opus5` | L | 1 |
| T4 | Per-pair watchers, the combine, one fetch per distinct line, the per-line 429 backoff, Identify's wake | `watch_loop.go`, `watcher.go`, `lines.go`, `combine.go`, `combine_test.go`, `watch_loop_test.go`, `watch_loop_cadence_test.go`, `watcher_test.go`, `main.go` (one line) | `cwd-implementer-opus5` | L | 2 |
| T5a | The device manager with the fake bus | `manager.go`, `manager_test.go`, `fakebus_test.go` | `cwd-implementer-opus5` | M | 2 |
| T6 | HTTP: `?beacon=`, `GET /beacons`, the aggregate `/status` | `server.go`, `server_test.go` | `cwd-implementer-opus5` | M | 2 |
| T7 | The tray: eight slots, the ordering rule, the aggregate icon, the S14 hook | `tray.go`, `tray_slots.go`, `tray_slots_test.go`, `tray_labels.go`, `tray_labels_test.go` | `cwd-implementer-opus5` | M | 2 |
| T8 | The settings page: per-beacon rows, the merge, Identify, Forget, Take over, the API block | `settings_handler.go`, `settings_beacons.go`, `settings_beacons_test.go`, `settings_handler_test.go`, `lines_cache_test.go` | `cwd-implementer-opus5` | L | 2 |
| T0b | Fix the identity mode and the preferred usage pair from the probe's file; the stop | `identity_mode.go`, `identity_test.go`, `pkg/fsbeacon/usbhid.go` (one comment) | `cwd-implementer-opus5` | XS | P (the probe file is in) |
| T5b | The real USB source and handle; the final `main.go` | `devices_farm.go`, `devices_farm_windows.go`, `devices_farm_darwin.go`, `beacon_check.go`, `main.go` | `cwd-implementer-opus5` | M | P+1 |
| T9 | Docs | `BEACON-DASHBOARD.md`, `README.md` | `cwd-implementer` | S | 3 |
| T10 | The owner's hands-on test; the Windows hidden-slot check; CI | — | the master and the owner | — | 4 |

Sizes: XS under 50 changed lines, S 50–200, M 200–500, L 500–1,000. **Seats:** every task but
T9 touches real data (the operator's config file), a third-party API (`go-hid`, `systray`),
numerics (the combine, the aggregate, the backoff, the id counter) or the HTTP contract, or, for
T5a, holds concurrency reasoning the plan cannot hold (goroutine interleavings, the deadlock
guard), which the owner's rule sends to `cwd-implementer-opus5` when in doubt. T9 is
transcription of text this plan and the design already hold: `cwd-implementer`. T3m is a file
move: a general-purpose dispatch on `sonnet`. Both Claude seats resolve to the same model and
effort today (owner, 2026-09-23); the split is kept as the owner's rule names it. Dispatch the
Claude seats with no `model:` field.

## Waves

A wave's tasks touch disjoint files and can run in parallel. Each task's review package is its
diff plus its section of this plan. Between waves the branch must be green: suite, vet (both
platforms), `gofmt`.

| Wave | Tasks | Files touched (no two tasks share one) | Starts when |
|---|---|---|---|
| 0 | T0 ∥ T1 ∥ T2 ∥ T3m | T0: `identity*.go` · T1: `pkg/fsbeacon/*`, `.github/workflows/build.yml` · T2: `config.go`, `config_store*.go`, `config_test.go` · T3m: `watch_loop_test.go`, `settings_handler_test.go` | now |
| 1 | T3 | `state.go`→`beacons.go`, `state_test.go`→`beacons_test.go`, `devices.go`, `watcher.go`, `main.go`, `server.go`, `settings_handler.go`, `watch_loop.go`, `tray.go`, and setup lines in `server_test.go`, `watcher_test.go`, `watch_loop_test.go`, `watch_loop_cadence_test.go`, `settings_handler_test.go`, `integration_test.go` | wave 0 merged |
| 2 | T4 ∥ T5a ∥ T6 ∥ T7 ∥ T8 | T4: `watch_loop.go`, `watcher.go`, `lines.go`, `combine*.go`, `watch_loop_test.go`, `watch_loop_cadence_test.go`, `watcher_test.go`, `main.go` (one line) · T5a: `manager*.go`, `fakebus_test.go` · T6: `server.go`, `server_test.go` · T7: `tray.go`, `tray_slots*.go`, `tray_labels*.go` · T8: `settings_handler.go`, `settings_beacons*.go`, `settings_handler_test.go`, `lines_cache_test.go` | wave 1 merged |
| P | T0b | `identity_mode.go`, `identity_test.go`, `pkg/fsbeacon/usbhid.go:3` | the probe file is in and read; may run beside any wave (its files are its own after T0) |
| P+1 | T5b | `devices_farm*.go`, `beacon_check.go`, `main.go` | T0b, T1, T3 and T5a merged |
| 3 | T9 | `BEACON-DASHBOARD.md`, `README.md` | wave 2 and T5b merged |
| 4 | T10 | — | T9 merged; the Windows exe built and hash-matched |

**Between T3 and T5b the tray compiles and opens no light.** T3 removes the single-beacon USB
goroutine from `main.go` (it uses `AppState.SetBeacon`, which goes); the registry starts empty, the
tray shows no beacon, the HTTP routes answer `503 beacon not connected`. That is the normal state of
a feature branch mid-build and it is stated here so no reviewer reads it as a defect. The branch is
not released until T10.

**Dispatch lines.** The controller pastes each task's section as the brief, with
`~/projects/agent-fieldtest/templates/dispatch-lines.md`. Every task ends with the branch green and
the task's commits pushed; the reviewer seat (`cwd-reviewer`) holds the diff against the task's
section. A Codex `task-review` seat runs beside it on the identical package, blind (owner rules
2026-09-20); the master rules.

## The probe, and what waits on it

The owner runs `beacon-probe.exe` (design §14.1) and sends back `beacon-probe.txt` with the note of
which light flashed for each `beacon N`. The master reads its `summary` block:

| The file says | The build does |
|---|---|
| `serial numbers: DISTINCT` | T0b sets `identityMode = identitySerial` |
| `serial numbers: NONE` | T0b sets `identityMode = identityPort` |
| `serial numbers: MIXED` or `DUPLICATED` | **stop**: T0b and T5b are not built; the file goes back to the design (§5.3) |
| two `beacon N` entries with the same `port key` and the **same** `usage page / usage` | **stop**, as above |
| two entries with the same `port key` and **different** usage lines | one light, two collections: T0b sets `preferredUsage` to the pair whose `beacon N` the owner saw flash (both flashed: the first in enumeration order) |
| otherwise | `preferredUsage = nil`: every path is opened |

Only T0b and T5b depend on the mode. Every other task is mode-agnostic: the fake bus is set to
`sn` or `port` per test, the registry and the HTTP routes handle a key as an opaque string, and the
settings page shows whichever key the record holds. If the stop fires, waves 0–2 are still correct
code; they wait for a revised design before T5b.

## Review Focus

Five input classes the design implies and no listed test exercised, most likely to bite first;
each line names the task that now pins it.

1. **Two records in the file with the same `key`** (a hand edit, or a copy-paste). The first in
   file order binds the light; the second stays a record that is never plugged in, listed
   `unplugged`, Forget-able. Nothing crashes, nothing is re-keyed silently. Pinned in T3
   (`TestRegistry_aDuplicateKeyInTheFileBindsTheFirstRecordOnly`).
2. **A name of forty runes that is more than forty bytes** (`Désk lëft …`). Counted in runes, so a
   forty-character accented name saves; a forty-one-rune one is refused. Pinned in T8
   (`TestSettingsPost_countsNameLengthInRunes`).
3. **A `Retry-After` that is a date already past, or a number with a sign or a fraction** (`-5`,
   `+5`, `1.5`). A past date or a negative number backs off nothing (0 s); `+5` and `1.5` are
   unreadable and land on 10 s. Pinned in T4 (`TestRetryAfterDelay_sweep` and its table).
4. **Nine or more lights.** The ninth is driven, listed by HTTP and the settings page, and the tray's
   top row says `9 beacons (8 shown here)`; no slot is written twice. Pinned in T7
   (`TestSlots_aNinthBeaconIsCountedNotShown`).
5. **A POST to a control route with a form body and no query**, carrying `beacon=b1` in the body.
   Go's `ParseForm` fills `r.Form` from the body too, so it drives b1; the documented way stays the
   query string, and a broken body never breaks the no-parameter path. Pinned in T6
   (`TestHandleSpin_beaconInAFormBodyIsHonoured`, `TestHandleSpin_aMalformedBodyStillDrivesEveryBeacon`).

---

## T0 — Identity: the key functions, both modes, and the public-id rules

**Carries out:** §5.3 (the match key, the two prefixes, the public id's shape), §5.4 (a name that
reads as a public id), §13 (the port key is the only platform-specific line), §10.1 (`b01` is
malformed). The mode's *value* is T0b's; T0 writes both branches so that every later task is
mode-agnostic.

**Files:**
- Create: `cmd/blinkybeacon-tray/identity.go`
- Create: `cmd/blinkybeacon-tray/identity_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (used by T2, T3, T5a, T5b, T6, T8):

```go
// identityMode says what a beacon's identity is. The build fixes one from the
// probe's file (identity_mode.go); it is never mixed at run time.
type identityMode int

const (
	identitySerial identityMode = iota + 1 // "sn:<serial>": the light carries its identity
	identityPort                           // "port:<key>": whatever light sits in the port is the beacon
)

const (
	keyPrefixSerial = "sn:"
	keyPrefixPort   = "port:"
)

// matchKey builds the key a record is recognised by, from what the source
// knows about a light: its serial string ("" when it reports none) and its
// port key (windowsPortKey or darwinPortKey). In serial mode a light with no
// serial gets "sn:", which every such light shares, so the second one is
// refused as having the same identity as the first: loud, never wrong.
func matchKey(mode identityMode, serial, portKey string) string

// windowsPortKey is the device-instance segment of a Windows HID path — the
// text between the second and third '#' — lower-cased. "" when the path has
// no such segment. For a light with no serial number Windows builds it from
// the hub and port, so it names the seat.
func windowsPortKey(path string) string

// darwinPortKey formats an IOKit location id as the macOS port key: "0x" and
// eight lower-case hex digits.
func darwinPortKey(locationID uint32) string

// publicID formats a counter value as the id the tray hands out: "b" + n.
func publicID(n int) string

// isPublicID reports whether s is exactly a public id as the tray writes one:
// a lower-case "b" and a decimal number ≥ 1 with no leading zero. "b01",
// "b0", "B1", "b1 " and "b" are not.
func isPublicID(s string) bool

// publicIDNumber is the number in a public id, 0 when s is not one.
func publicIDNumber(s string) int

// readsAsPublicID reports whether a name, after trimming, would be taken for
// a public id in any letter case ("B2", "b002" included): such a name is
// refused at save so a ?beacon= value is never ambiguous.
func readsAsPublicID(name string) bool
```

- [ ] **Step 1: Write the failing tests** (`identity_test.go`)

```go
package main

import (
	"fmt"
	"testing"
)

const (
	// A Windows HID path for a light with no serial number: the instance
	// segment is bus-generated (the '&' marks it), from the hub and port.
	winPathNoSerial = `\\?\hid#vid_340d&pid_1710#6&2a1b3c4d&0&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}`
	// The same light in another port: a new instance segment.
	winPathOtherPort = `\\?\hid#vid_340d&pid_1710#6&2a1b3c4d&0&0001#{4d1e55b2-f16f-11cf-88cb-001111000030}`
	// A light that reports a serial: Windows uses it as the instance id.
	winPathWithSerial = `\\?\hid#vid_340d&pid_1710#ABC123#{4d1e55b2-f16f-11cf-88cb-001111000030}`
)

func TestWindowsPortKey_isTheInstanceSegmentLowerCased(t *testing.T) {
	cases := map[string]string{
		winPathNoSerial:   "6&2a1b3c4d&0&0000",
		winPathOtherPort:  "6&2a1b3c4d&0&0001",
		winPathWithSerial: "abc123",
		`\\?\hid#vid_340d&pid_1710`: "", // no instance segment
		"":                           "",
		"DevSrvsID:4295000123":       "", // a macOS path has no '#'
	}
	for path, want := range cases {
		if got := windowsPortKey(path); got != want {
			t.Errorf("windowsPortKey(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestDarwinPortKey_isEightHexDigits(t *testing.T) {
	cases := map[uint32]string{0x14200000: "0x14200000", 0: "0x00000000", 0xffffffff: "0xffffffff"}
	for id, want := range cases {
		if got := darwinPortKey(id); got != want {
			t.Errorf("darwinPortKey(%#x) = %q, want %q", id, got, want)
		}
	}
}

func TestMatchKey_bothModes(t *testing.T) {
	cases := []struct {
		mode           identityMode
		serial, port   string
		want           string
	}{
		{identitySerial, "ABC123", "6&2a1b3c4d&0&0000", "sn:ABC123"},
		{identitySerial, " ABC123 ", "6&2a1b3c4d&0&0000", "sn:ABC123"}, // trimmed
		{identitySerial, "", "6&2a1b3c4d&0&0000", "sn:"},               // no serial in serial mode: shared, refused later
		{identityPort, "ABC123", "6&2a1b3c4d&0&0000", "port:6&2a1b3c4d&0&0000"},
		{identityPort, "", "0x14200000", "port:0x14200000"},
	}
	for _, c := range cases {
		if got := matchKey(c.mode, c.serial, c.port); got != c.want {
			t.Errorf("matchKey(%v, %q, %q) = %q, want %q", c.mode, c.serial, c.port, got, c.want)
		}
	}
}

func TestMatchKey_aPortMoveChangesThePortKeyNotTheSerialKey(t *testing.T) {
	before := matchKey(identityPort, "", windowsPortKey(winPathNoSerial))
	after := matchKey(identityPort, "", windowsPortKey(winPathOtherPort))
	if before == after {
		t.Errorf("port mode: a move must change the key; both are %q", before)
	}
	if matchKey(identitySerial, "ABC123", windowsPortKey(winPathNoSerial)) !=
		matchKey(identitySerial, "ABC123", windowsPortKey(winPathOtherPort)) {
		t.Error("serial mode: a move must not change the key")
	}
}

func TestPublicID_shape(t *testing.T) {
	yes := []string{"b1", "b2", "b10", "b1000000"}
	no := []string{"", "b", "b0", "b01", "b001", "B1", "b1 ", " b1", "b-1", "b+1", "b1.0", "bx", "1", "beacon1", "b99999999999999999999"}
	for _, s := range yes {
		if !isPublicID(s) {
			t.Errorf("isPublicID(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isPublicID(s) {
			t.Errorf("isPublicID(%q) = true, want false", s)
		}
	}
}

func TestPublicID_numberRoundTripsAcrossTheRange(t *testing.T) {
	// The id counter is numeric: sweep well past any desk's beacon count, and
	// past the pinned values 1, 2, 3 the design's examples use.
	for n := 1; n <= 10_000_000; n = n*3 + 1 {
		for _, m := range []int{n - 1, n, n + 1} {
			if m < 1 {
				continue
			}
			id := publicID(m)
			if !isPublicID(id) {
				t.Fatalf("publicID(%d) = %q is not a public id", m, id)
			}
			if got := publicIDNumber(id); got != m {
				t.Fatalf("publicIDNumber(%q) = %d, want %d", id, got, m)
			}
			if !readsAsPublicID(id) || !readsAsPublicID("B"+id[1:]) || !readsAsPublicID("  "+id+"  ") {
				t.Fatalf("readsAsPublicID must accept %q in any case and with spaces around it", id)
			}
			if isPublicID(fmt.Sprintf("b0%d", m)) {
				t.Fatalf("a leading zero must not be a public id: b0%d", m)
			}
		}
	}
	if publicIDNumber("b01") != 0 || publicIDNumber("desk") != 0 {
		t.Error("publicIDNumber must be 0 for a non-id")
	}
}

func TestReadsAsPublicID_namesThatAreNotIds(t *testing.T) {
	for _, s := range []string{"Desk left", "b1x", "beacon", "", "   ", "b", "B", "1b", "b 1"} {
		if readsAsPublicID(s) {
			t.Errorf("readsAsPublicID(%q) = true, want false", s)
		}
	}
	for _, s := range []string{"b1", "B1", "b01", "B007", " b2 "} {
		if !readsAsPublicID(s) {
			t.Errorf("readsAsPublicID(%q) = false, want true", s)
		}
	}
}
```

- [ ] **Step 2: Run them red.** `go test -count=1 -run 'PortKey|MatchKey|PublicID' ./cmd/blinkybeacon-tray/`
  fails to compile (`undefined: windowsPortKey` and the rest). **Disarms** to prove each test
  bites once the code exists: `windowsPortKey` returning `parts[2]` without `strings.ToLower`
  turns `TestWindowsPortKey…` red on `winPathWithSerial`; `matchKey` without `TrimSpace` turns
  the trimmed case red; `isPublicID` implemented as `strings.HasPrefix(s,"b") && digits` turns
  `b0`/`b01` red; `readsAsPublicID` without `TrimSpace` turns `" b2 "` red.

- [ ] **Step 3: Implement** `identity.go` with the signatures above. `isPublicID`: `len(s) ≥ 2`,
  `s[0] == 'b'`, every remaining byte in `'0'..'9'`, `s[1] != '0'`, and `strconv.Atoi(s[1:])`
  succeeds (so a 20-digit tail is not an id). `readsAsPublicID`: trim, lower-case, then
  `len ≥ 2 && s[0]=='b' && all digits` (leading zeros allowed here on purpose). `windowsPortKey`:
  `strings.Split(path, "#")`, need `len ≥ 3`, return `strings.ToLower(parts[2])`.

- [ ] **Step 4: Run green**, then `gofmt -l`, `go vet ./cmd/blinkybeacon-tray/` (Linux and
  `GOOS=windows`), the whole suite.

- [ ] **Step 5: Commit** `feat(tray): the beacon identity functions — match keys in serial and port mode, and the public-id rules`

**Acceptance:**
- Both key modes and both platforms' port keys are table-tested on Linux; no platform build tag in
  these two files.
- The public-id sweep runs to ten million and pins `b01`, `b0`, `B1` as non-ids and `b01`/`B1` as
  names that are refused.
- No constant names the mode; `identity.go` compiles without `identity_mode.go` (T0b).

**Size:** S (about 90 lines of code, 130 of test). **Seat:** `cwd-implementer-opus5`: numerics (the
id counter's parser is swept, plan-gate rule 1) and a parser of a third-party path shape.

**Plan gate:** rule 1: the sweep above; pinned values `b1`…`b3` from the design's examples are
ordinary members of the sweep, no coincidence. Rule 2: nothing sent. Rule 3: the parser's
real-shape fixture is the Windows path shape from `hid_windows.c:713` as the design cites it; the
probe's **real** path lines are added as fixtures in T0b, which is when they exist. Rules 4–5: n/a.

---

## T1 — Library: enumerate, open by path, `Info`, `LastAttemptOK`, `LocationID`

**Carries out:** §12 (the library change paragraph), §5.1 (`OpenPath` is the one the tray moves
to), §14.2 (`Live()` reads an atomic the worker sets), red-team note 3 (`LastAttemptOK` starts
`true` at open). `OpenFarmBeacon()` and the three 10-byte reports are untouched; `cmd/fsbeacon`
keeps building.

**Files:**
- Modify: `pkg/fsbeacon/usbhid.go` (165 lines today)
- Create: `pkg/fsbeacon/usbhid_darwin.go`
- Modify: `pkg/fsbeacon/usbhid_test.go` (35 lines: one new test, one skip made robust)
- Modify: `.github/workflows/build.yml:66-69` (the macOS arm64 test step also runs `./pkg/fsbeacon/`)

**Interfaces:**
- Consumes: `github.com/sstallion/go-hid` v0.14.1: `hid.Enumerate(vid, pid, fn)` (`hid.go:143`),
  `hid.OpenPath(path)` (`hid.go:188`), `hid.DeviceInfo` (`hid.go:104-116`),
  `(*hid.Device).GetLocationID()` (`hid_darwin.go:33`).
- Produces (used by T5b):

```go
// DeviceInfo is what the HID layer reports about one attached beacon.
type DeviceInfo = hid.DeviceInfo

// Enumerate lists every beacon the HID layer can see, in the layer's order.
// It initialises the HID library if nothing has yet.
func Enumerate() ([]DeviceInfo, error)

// OpenPath opens exactly the device info names and returns its handle. The
// handle remembers the info it was opened from (Info) and starts with its
// last keepalive attempt counted as a success (LastAttemptOK), because the
// keepalive's first write happens only when a worker is started and its
// first checked write five seconds after that.
func OpenPath(info DeviceInfo) (*FarmBeacon, error)

// Info returns the DeviceInfo the beacon was opened from; the zero value for
// a handle from OpenFarmBeacon.
func (beacon *FarmBeacon) Info() DeviceInfo

// LastAttemptOK reports whether the keepalive's most recent write succeeded.
// True from open until a write fails. It is an atomic, read without the
// beacon's mutex, so a caller can ask while the worker holds the lock.
func (beacon *FarmBeacon) LastAttemptOK() bool

// usbhid_darwin.go — LocationID is the IOKit location id of the open device:
// the USB controller and port, the stable identity of a seat on a Mac.
func (beacon *FarmBeacon) LocationID() (uint32, error)
```

- [ ] **Step 1: Write the failing test** (append to `usbhid_test.go`; it compiles and runs only
  where the package links, which is the macOS CI leg and a developer's Mac or Windows PC)

```go
func TestFarmBeacon_lastAttemptOKStartsTrue(t *testing.T) {
	// A handle that has just been opened has had no write fail. If this
	// started false, a freshly opened light would read "not live" for the
	// five seconds before its first checked keepalive write, and a second
	// path with the same key would be taken for a replug and steal the record.
	b := newFarmBeacon(nil, DeviceInfo{Path: "test"})
	if !b.LastAttemptOK() {
		t.Fatal("LastAttemptOK() = false on a freshly opened handle, want true")
	}
	if b.Info().Path != "test" {
		t.Errorf("Info().Path = %q, want the path it was opened from", b.Info().Path)
	}
}
```

  And make the existing hardware test skip on **any** open failure, not one message
  (`usbhid_test.go:11`): replace the `if err.Error() == "No HID devices …"` check with
  `t.Skipf("no beacon to test with: %v", err)`. A light-less CI runner is the negative environment
  of plan-gate rule 4; the recorded outcome must be a skip, never a false failure on a message that
  differs by platform.

- [ ] **Step 2: Run red** where it can run: on this box the package does not compile (`libudev.h`
  missing), so the red run is `GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc go vet ./pkg/fsbeacon/`
  failing with `undefined: newFarmBeacon`. **Disarm:** once implemented, delete the
  `lastAttemptOK.Store(true)` line in `newFarmBeacon`; the test must fail on the macOS CI leg.

- [ ] **Step 3: Implement.** In `usbhid.go`:

```go
import (
	"sync"
	"sync/atomic"
	"time"

	hid "github.com/sstallion/go-hid"
)

type DeviceInfo = hid.DeviceInfo

type FarmBeacon struct {
	sync.Mutex
	stop      chan bool
	hidDevice *hid.Device
	info      DeviceInfo
	// lastAttemptOK is the result of the keepalive's most recent write. An
	// atomic so it can be read while the worker holds the mutex above.
	lastAttemptOK atomic.Bool
	OnError       func()
}

func newFarmBeacon(dev *hid.Device, info DeviceInfo) *FarmBeacon {
	b := &FarmBeacon{hidDevice: dev, info: info}
	b.lastAttemptOK.Store(true)
	return b
}

func OpenFarmBeacon() (*FarmBeacon, error) {
	if err := hid.Init(); err != nil {
		return nil, err
	}
	dev, err := hid.OpenFirst(fs22vid, fs22pid)
	if err != nil {
		return nil, err
	}
	return newFarmBeacon(dev, DeviceInfo{}), nil
}

func Enumerate() ([]DeviceInfo, error) {
	if err := hid.Init(); err != nil {
		return nil, err
	}
	var found []DeviceInfo
	err := hid.Enumerate(fs22vid, fs22pid, func(info *hid.DeviceInfo) error {
		found = append(found, *info)
		return nil
	})
	return found, err
}

func OpenPath(info DeviceInfo) (*FarmBeacon, error) {
	if err := hid.Init(); err != nil {
		return nil, err
	}
	dev, err := hid.OpenPath(info.Path)
	if err != nil {
		return nil, err
	}
	return newFarmBeacon(dev, info), nil
}

func (beacon *FarmBeacon) Info() DeviceInfo   { return beacon.info }
func (beacon *FarmBeacon) LastAttemptOK() bool { return beacon.lastAttemptOK.Load() }
```

  In `worker`, record every write's result and keep the flow as it is (the initial write is still
  not acted on; the ticker's failure still calls `OnError` and returns):

```go
	// Set the initial device state
	_, err := beacon.hidDevice.Write(magic)
	beacon.lastAttemptOK.Store(err == nil)
	for {
		select {
		case <-ticker.C:
			if _, err := beacon.hidDevice.Write(magic); err != nil {
				beacon.lastAttemptOK.Store(false)
				if beacon.OnError != nil {
					beacon.OnError()
				}
				return
			}
			beacon.lastAttemptOK.Store(true)
```

  `usbhid_darwin.go` (`//go:build darwin`):

```go
package fsbeacon

// LocationID is the IOKit location id of the open device: the USB controller
// and port path, which is the same every time a light is plugged into that
// port and is the identity of a seat on a Mac.
func (beacon *FarmBeacon) LocationID() (uint32, error) {
	return beacon.hidDevice.GetLocationID()
}
```

  `build.yml`, the macOS `test` step: `run: go test -count=1 -race ./cmd/blinkybeacon-tray/ ./pkg/fsbeacon/`.
  The Windows job cross-compiles from Linux and cannot run the library's test; unchanged.

- [ ] **Step 4: Verify.** `GOOS=windows … go vet ./pkg/fsbeacon/ ./cmd/blinkybeacon-tray/` clean;
  `GOOS=darwin CGO_ENABLED=0 go vet ./pkg/fsbeacon/` is **not** meaningful (cgo off drops the
  package), so the darwin file is checked by the `build` workflow's macOS legs on the pushed
  branch: both green, and the arm64 leg's log shows `ok  …/pkg/fsbeacon` with the hardware test
  `SKIP`. The tray suite on Linux unchanged (`ok`). `go build ./cmd/fsbeacon/` under
  `GOOS=windows` still builds (the CLI uses `OpenFarmBeacon`).

- [ ] **Step 5: Commit** `feat(fsbeacon): enumerate beacons and open one by path; a handle keeps its DeviceInfo and reports its last keepalive result`

**Acceptance:**
- `OpenFarmBeacon`, `Flash`, `Spin`, `Stop`, `Close`, `StartIdleWorker` and the three report byte
  strings are byte-for-byte unchanged (`git diff` shows no hunk in them beyond the constructor
  call).
- `LastAttemptOK()` is true at open and false after a failed ticker write; the worker's exit path
  is unchanged.
- The hardware test skips on any open failure with the error in the skip message.
- CI: macOS arm64 runs the library's tests; all three build legs green.

**Size:** S (about 60 lines of code, 15 of test, 1 of workflow). **Seat:** `cwd-implementer-opus5`:
a third-party API surface.

**Plan gate:** rule 1: n/a. Rule 2 (payload): the three HID reports are the unchanged 10-byte
constants `fs22MagicFlash/Spin/Stop`; this task adds no write. Rule 3: n/a (no parser). Rule 4:
this is the one task whose test imports a cgo package; the negative environment is Linux without
`libudev.h` (this box) where the package does not compile at all, a pre-existing documented quirk
(`docs/PUNCHLIST.md`, "Documented quirks"); the remedy is to run it on the macOS leg, which this
task wires in, and the light-less runner is the second negative environment, where the hardware
test now records `SKIP`. Rule 5: none.

---

## T2 — Config: `BeaconRecord`, the `beacons` list, `beacon_seq`, migration, the mirror, forgiveness, and the one writer

**Carries out:** §6 (the model), §10.1 (the new shape, the id forgiveness), §10.2 steps 1 and 3
(the loader's synthesis; the bind itself is T3's), §10.5 (`configStore`, one writer, no restart on a
bind), D5 (the mirror). Red-team note 4's "per-start reservation" is answered here: every write
through the store carries the registry's live counter (T3's `updateConfig` sets `BeaconSeq`), so an
in-memory id issued before any later save is never handed out again after a restart; and before
any save the stale tab is refused by the form token (see T8).

**Files:**
- Modify: `cmd/blinkybeacon-tray/config.go` (227 lines)
- Create: `cmd/blinkybeacon-tray/config_store.go`
- Modify: `cmd/blinkybeacon-tray/config_test.go` (295 lines; one test's struct comparison changes,
  fixtures and tests added)
- Create: `cmd/blinkybeacon-tray/config_store_test.go`

**Interfaces:**
- Consumes: T0's `isPublicID`, `publicID`, `publicIDNumber`.
- Produces:

```go
// BeaconRecord is one beacon as the config file holds it (§10.1). Lines is
// the dashboard line numbers it follows (zero or more); the four light
// settings are its own.
type BeaconRecord struct {
	ID                string `json:"id"`
	Key               string `json:"key"`
	Name              string `json:"name"`
	Lines             []int  `json:"lines"`
	FlashSeconds      int    `json:"flash_seconds"`
	LobbyFlash        bool   `json:"lobby_flash"`
	LobbyFlashSeconds int    `json:"lobby_flash_seconds"`
	PauseSide         string `json:"pause_side"`
}

type Config struct {
	Addr           string         `json:"addr"`
	Port           int            `json:"port"`
	DashboardURL   string         `json:"dashboard_url"`
	APIToken       string         `json:"token"`
	PollIntervalMs int            `json:"poll_interval_ms"`
	BeaconSeq      int            `json:"beacon_seq"`
	Beacons        []BeaconRecord `json:"beacons"`
	// The mirror of the first beacon, for a dashboard-v0.8.0 build pointed at
	// this file: written from Beacons[0] on every save, read only when
	// "beacons" is absent or empty.
	LineNumber        int    `json:"line_number"`
	FlashSeconds      int    `json:"flash_seconds"`
	LobbyFlash        bool   `json:"lobby_flash"`
	LobbyFlashSeconds int    `json:"lobby_flash_seconds"`
	PauseSide         string `json:"pause_side"`
}

const migratedBeaconName = "Beacon 1"

// defaultBeacon is the record a fresh install, or a file with no beacons
// list, starts with: b1, unmatched, "Beacon 1", following the mirror's line
// with the mirror's light settings (each defaulted where invalid).
func defaultBeacon(c Config) BeaconRecord

// effectiveBeacons is the records a Config stands for: Beacons when it has
// any, else the one record synthesised from the mirror. Every reader of the
// list goes through it, so a Config built by hand in a test or by main.go's
// flags means the same as one loaded from a file.
func (c Config) effectiveBeacons() []BeaconRecord

// normaliseBeacons applies the loader's forgiveness to the list in place and
// returns the lines it logged: per record, lines < 1 dropped and duplicates
// collapsed (first occurrence order kept); out-of-range durations and an
// unknown pause_side land on the defaults; then BeaconSeq is raised to the
// highest valid id number, and every record whose id is missing, malformed
// or a repeat of an earlier record's is re-issued the next id.
func normaliseBeacons(c *Config) []string

// mirrorFirstBeacon copies Beacons[0]'s line (or 1 if it follows none) and
// four light settings onto the five mirror fields.
func (c *Config) mirrorFirstBeacon()

// clone is a deep copy: Beacons and every Lines slice copied, so a caller
// that edits the copy cannot touch the live config.
func (c Config) clone() Config

// configStore is the one writer of the config file (§10.5). Get is what
// getCfg was in main.go; Update applies one change to a copy of the live
// config, writes it with saveConfig's atomic owner-only write, and only then
// publishes it. onChange is told every published change with the old and new
// config, under the store's lock, so it must not call back into the store.
type configStore struct {
	mu       sync.Mutex
	cfg      Config
	onChange func(old, new Config)
}

func newConfigStore(cfg Config, onChange func(old, new Config)) *configStore
func (s *configStore) Get() Config
func (s *configStore) Update(fn func(*Config) error) error

// listenAddrChanged says whether an update moved the HTTP server: the one
// change that restarts it.
func listenAddrChanged(old, new Config) bool
```

  `loadConfig` keeps its signature. After unmarshalling and today's defaults it does:
  `if len(cfg.Beacons) == 0 { cfg.Beacons = []BeaconRecord{defaultBeacon(cfg)} }`, then
  `for _, l := range normaliseBeacons(&cfg) { log.Print(l) }`, then `cfg.mirrorFirstBeacon()`.
  `defaultConfig()` returns `Beacons: []BeaconRecord{defaultBeacon(...)}` with `BeaconSeq: 1`.
  `saveConfig` calls `cfg.mirrorFirstBeacon()` before marshalling. Loading writes nothing.

- [ ] **Step 1: Fixtures** (in `config_test.go`, beside `v070ConfigFile`):

```go
// v050ConfigFile is the shape the earliest dashboard builds wrote: no light
// settings, no token, no poll interval.
const v050ConfigFile = `{"addr":"127.0.0.1","port":1337,"dashboard_url":"https://dashboard.example.com","line_number":2}`

// v080ConfigFile is a config file exactly as dashboard-v0.8.0 writes it:
// json.MarshalIndent of that build's Config, every key, its order and
// indentation (generated from that struct on 2026-10-20).
const v080ConfigFile = `{
  "addr": "127.0.0.1",
  "port": 1337,
  "dashboard_url": "https://dashboard.example.com",
  "line_number": 2,
  "token": "test-bearer-token",
  "flash_seconds": 20,
  "lobby_flash": true,
  "lobby_flash_seconds": 7,
  "pause_side": "dire",
  "poll_interval_ms": 750
}`

// v090ConfigFile is the new shape (design §10.1): two beacons, the mirror.
const v090ConfigFile = `{
  "addr": "127.0.0.1",
  "port": 1337,
  "dashboard_url": "https://dashboard.example.com",
  "token": "test-bearer-token",
  "poll_interval_ms": 2000,
  "beacon_seq": 3,
  "beacons": [
    {"id": "b1", "key": "port:6&2a1b3c4d&0&0001", "name": "Desk left", "lines": [1, 3], "flash_seconds": 15, "lobby_flash": false, "lobby_flash_seconds": 10, "pause_side": "both"},
    {"id": "b3", "key": "port:6&2a1b3c4d&0&0002", "name": "Desk right", "lines": [2], "flash_seconds": 20, "lobby_flash": true, "lobby_flash_seconds": 7, "pause_side": "radiant"}
  ],
  "line_number": 1,
  "flash_seconds": 15,
  "lobby_flash": false,
  "lobby_flash_seconds": 10,
  "pause_side": "both"
}`
```

- [ ] **Step 2: Write the failing tests** (`config_test.go`)

```go
func TestConfig_aFileWithoutBeaconsLoadsAsOneUnmatchedBeaconOnItsLine(t *testing.T) {
	cases := []struct {
		name, body            string
		wantLine, wantFlash   int
		wantLobby             bool
		wantLobbySecs         int
		wantSide              string
	}{
		{"v0.5.0", v050ConfigFile, 2, 15, false, 10, pauseSideBoth},
		{"v0.7.0", v070ConfigFile, 2, 20, true, 7, pauseSideDire},
		{"v0.8.0", v080ConfigFile, 2, 20, true, 7, pauseSideDire},
		{"empty beacons list", `{"line_number":4,"beacons":[]}`, 4, 15, false, 10, pauseSideBoth},
		{"no file at all", "", 1, 15, false, 10, pauseSideBoth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempConfig(t)
			if tc.body != "" {
				writeRawConfig(t, tc.body)
			}
			cfg := loadConfig()
			if len(cfg.Beacons) != 1 {
				t.Fatalf("%d beacons, want exactly 1", len(cfg.Beacons))
			}
			b := cfg.Beacons[0]
			if b.ID != "b1" || b.Key != "" || b.Name != migratedBeaconName {
				t.Errorf("record = %+v, want b1, no key, %q", b, migratedBeaconName)
			}
			if len(b.Lines) != 1 || b.Lines[0] != tc.wantLine {
				t.Errorf("Lines = %v, want [%d]", b.Lines, tc.wantLine)
			}
			if b.FlashSeconds != tc.wantFlash || b.LobbyFlash != tc.wantLobby ||
				b.LobbyFlashSeconds != tc.wantLobbySecs || b.PauseSide != tc.wantSide {
				t.Errorf("light = %+v, want %d/%v/%d/%s", b, tc.wantFlash, tc.wantLobby, tc.wantLobbySecs, tc.wantSide)
			}
			if cfg.BeaconSeq != 1 {
				t.Errorf("BeaconSeq = %d, want 1", cfg.BeaconSeq)
			}
		})
	}
}

func TestConfig_loadingWritesNothing(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, v080ConfigFile)
	path, _ := configFilePath()
	before, _ := os.ReadFile(path)
	loadConfig()
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Error("loading a v0.8.0 file changed it; loading must be a pure read")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("loading left %d files in the config dir, want the one it read", len(entries))
	}
}

func TestConfig_theNewShapeRoundTrips(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, v090ConfigFile)
	cfg := loadConfig()
	if len(cfg.Beacons) != 2 || cfg.Beacons[0].ID != "b1" || cfg.Beacons[1].ID != "b3" {
		t.Fatalf("beacons = %+v", cfg.Beacons)
	}
	if cfg.Beacons[1].Name != "Desk right" || cfg.Beacons[1].Key != "port:6&2a1b3c4d&0&0002" ||
		!reflect.DeepEqual(cfg.Beacons[1].Lines, []int{2}) || cfg.Beacons[1].PauseSide != pauseSideRadiant {
		t.Errorf("second record = %+v", cfg.Beacons[1])
	}
	if cfg.BeaconSeq != 3 {
		t.Errorf("BeaconSeq = %d, want 3", cfg.BeaconSeq)
	}
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	again := loadConfig()
	if !reflect.DeepEqual(again, cfg) {
		t.Errorf("round trip changed the config:\n%+v\n%+v", cfg, again)
	}
}

func TestConfig_theMirrorIsWrittenFromTheFirstBeacon(t *testing.T) {
	withTempConfig(t)
	cfg := loadConfig() // the defaults: b1 on line 1
	cfg.Beacons = []BeaconRecord{
		{ID: "b2", Key: "port:x", Name: "Desk right", Lines: []int{3, 7}, FlashSeconds: 45, LobbyFlash: true, LobbyFlashSeconds: 9, PauseSide: pauseSideDire},
		{ID: "b1", Key: "port:y", Name: "Desk left", Lines: nil, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
	}
	cfg.BeaconSeq = 2
	if err := saveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	path, _ := configFilePath()
	raw, _ := os.ReadFile(path)
	var top struct {
		LineNumber        int    `json:"line_number"`
		FlashSeconds      int    `json:"flash_seconds"`
		LobbyFlash        bool   `json:"lobby_flash"`
		LobbyFlashSeconds int    `json:"lobby_flash_seconds"`
		PauseSide         string `json:"pause_side"`
	}
	json.Unmarshal(raw, &top)
	if top.LineNumber != 3 || top.FlashSeconds != 45 || !top.LobbyFlash || top.LobbyFlashSeconds != 9 || top.PauseSide != pauseSideDire {
		t.Errorf("mirror = %+v, want the first record's values with its first line", top)
	}
	// A first beacon following no line mirrors line 1, the old default.
	cfg.Beacons[0], cfg.Beacons[1] = cfg.Beacons[1], cfg.Beacons[0]
	saveConfig(cfg)
	raw, _ = os.ReadFile(path)
	json.Unmarshal(raw, &top)
	if top.LineNumber != 1 {
		t.Errorf("mirror line_number = %d for a first beacon with no lines, want 1", top.LineNumber)
	}
	// And an old build reading this file sees exactly the keys it knows how to read.
	for _, key := range []string{`"line_number"`, `"flash_seconds"`, `"lobby_flash"`, `"lobby_flash_seconds"`, `"pause_side"`, `"token"`, `"poll_interval_ms"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("saved file lacks %s", key)
		}
	}
}

func TestConfig_theMirrorIsIgnoredWhenBeaconsArePresent(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, `{"line_number":9,"flash_seconds":600,"beacons":[{"id":"b1","key":"","name":"Beacon 1","lines":[2],"flash_seconds":15,"lobby_flash":false,"lobby_flash_seconds":10,"pause_side":"both"}]}`)
	cfg := loadConfig()
	if cfg.Beacons[0].Lines[0] != 2 || cfg.Beacons[0].FlashSeconds != 15 {
		t.Errorf("the mirror overrode the record: %+v", cfg.Beacons[0])
	}
	if cfg.LineNumber != 2 || cfg.FlashSeconds != 15 {
		t.Errorf("after load the mirror fields must be the first record's, got line %d flash %d", cfg.LineNumber, cfg.FlashSeconds)
	}
}

func TestConfig_beaconSeqIsRaisedToTheHighestIdOnLoad(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, `{"beacon_seq":1,"beacons":[{"id":"b7","key":"port:a","name":"A","lines":[1]},{"id":"b2","key":"port:b","name":"B","lines":[]}]}`)
	cfg := loadConfig()
	if cfg.BeaconSeq != 7 {
		t.Errorf("BeaconSeq = %d, want 7 (edited down, raised back)", cfg.BeaconSeq)
	}
	withTempConfig(t)
	writeRawConfig(t, `{"beacons":[{"id":"b4","key":"port:a","name":"A","lines":[1]}]}`)
	if got := loadConfig().BeaconSeq; got != 4 {
		t.Errorf("BeaconSeq = %d with no counter in the file, want 4", got)
	}
}

func TestConfig_aMissingMalformedOrDuplicateIdIsReissuedAndLogged(t *testing.T) {
	withTempConfig(t)
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	writeRawConfig(t, `{"beacon_seq":3,"beacons":[
	  {"id":"b1","key":"port:a","name":"A","lines":[1]},
	  {"id":"b01","key":"port:b","name":"B","lines":[]},
	  {"key":"port:c","name":"C","lines":[]},
	  {"id":"b1","key":"port:d","name":"D","lines":[]},
	  {"id":"B2","key":"port:e","name":"E","lines":[]}]}`)
	cfg := loadConfig()
	want := []string{"b1", "b4", "b5", "b6", "b7"}
	for i, b := range cfg.Beacons {
		if b.ID != want[i] {
			t.Errorf("record %d (%s) id = %q, want %q", i, b.Name, b.ID, want[i])
		}
	}
	if cfg.BeaconSeq != 7 {
		t.Errorf("BeaconSeq = %d, want 7", cfg.BeaconSeq)
	}
	for _, frag := range []string{`"b01"`, `"B2"`, `"b4"`, `"b7"`} {
		if !strings.Contains(logged.String(), frag) {
			t.Errorf("log does not name %s:\n%s", frag, logged.String())
		}
	}
}

func TestConfig_idReissueSweep(t *testing.T) {
	// The counter is numeric (plan-gate rule 1): over random lists of ids —
	// valid, malformed, duplicated — every loaded id is valid and distinct,
	// the counter is at least the highest valid id, and every re-issued id
	// is above every id the file held.
	rng := rand.New(rand.NewSource(20261020))
	for round := 0; round < 500; round++ {
		n := 1 + rng.Intn(20)
		var recs []string
		highest := 0
		for i := 0; i < n; i++ {
			switch rng.Intn(5) {
			case 0:
				recs = append(recs, `"id":"b01"`)
			case 1:
				recs = append(recs, `"id":""`)
			default:
				m := 1 + rng.Intn(1_000_000)
				if m > highest {
					highest = m
				}
				recs = append(recs, fmt.Sprintf(`"id":"b%d"`, m))
			}
		}
		seq := rng.Intn(2_000_000)
		var sb strings.Builder
		fmt.Fprintf(&sb, `{"beacon_seq":%d,"beacons":[`, seq)
		for i, r := range recs {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `{%s,"key":"port:%d","name":"N%d","lines":[1]}`, r, i, i)
		}
		sb.WriteString("]}")
		// Which records keep their id: the ones whose id was valid and the
		// first of its value in file order.
		keep := map[int]string{}
		firstOf := map[string]bool{}
		for i, r := range recs {
			id := strings.TrimSuffix(strings.TrimPrefix(r, `"id":"`), `"`)
			if isPublicID(id) && !firstOf[id] {
				firstOf[id] = true
				keep[i] = id
			}
		}
		withTempConfig(t)
		writeRawConfig(t, sb.String())
		cfg := loadConfig()
		seen := map[string]bool{}
		floor := max(seq, highest)
		for i, b := range cfg.Beacons {
			if !isPublicID(b.ID) || seen[b.ID] {
				t.Fatalf("round %d: id %q invalid or repeated in %s", round, b.ID, sb.String())
			}
			seen[b.ID] = true
			if want, kept := keep[i]; kept {
				if b.ID != want {
					t.Fatalf("round %d: record %d kept a valid unique id %q in the file but loaded as %q", round, i, want, b.ID)
				}
			} else if publicIDNumber(b.ID) <= floor {
				t.Fatalf("round %d: record %d was re-issued %q, not above the file's counter %d and highest id %d", round, i, b.ID, seq, highest)
			}
		}
		if cfg.BeaconSeq < floor {
			t.Fatalf("round %d: BeaconSeq %d below highest id %d or file counter %d", round, cfg.BeaconSeq, highest, seq)
		}
	}
}

func TestConfig_perBeaconValuesLoadWithTodaysForgiveness(t *testing.T) {
	withTempConfig(t)
	writeRawConfig(t, `{"beacons":[{"id":"b1","key":"","name":"Beacon 1","lines":[0,3,-1,3,2],"flash_seconds":0,"lobby_flash_seconds":100000,"pause_side":"Radiant"}]}`)
	b := loadConfig().Beacons[0]
	if !reflect.DeepEqual(b.Lines, []int{3, 2}) {
		t.Errorf("Lines = %v, want [3 2]: under-1 dropped, duplicates collapsed, order kept", b.Lines)
	}
	if b.FlashSeconds != defaultFlashSeconds || b.LobbyFlashSeconds != defaultLobbyFlashSeconds || b.PauseSide != pauseSideBoth {
		t.Errorf("light = %+v, want the documented defaults", b)
	}
}

func TestConfig_effectiveBeaconsSynthesisesFromTheMirrorForAHandBuiltConfig(t *testing.T) {
	c := Config{DashboardURL: "http://d", LineNumber: 5, APIToken: "t"}
	got := c.effectiveBeacons()
	if len(got) != 1 || got[0].ID != "b1" || !reflect.DeepEqual(got[0].Lines, []int{5}) || got[0].FlashSeconds != defaultFlashSeconds {
		t.Errorf("effectiveBeacons() = %+v", got)
	}
	c.Beacons = []BeaconRecord{{ID: "b2", Lines: []int{1}}}
	if got := c.effectiveBeacons(); len(got) != 1 || got[0].ID != "b2" {
		t.Errorf("with a list present effectiveBeacons must return it, got %+v", got)
	}
}
```

  And `config_store_test.go`:

```go
func TestConfigStore_updateWritesThenPublishes(t *testing.T) {
	withTempConfig(t)
	var changes []string
	s := newConfigStore(loadConfig(), func(old, new Config) {
		changes = append(changes, fmt.Sprintf("%d->%d", old.Port, new.Port))
	})
	if err := s.Update(func(c *Config) error { c.Port = 1400; return nil }); err != nil {
		t.Fatal(err)
	}
	if s.Get().Port != 1400 || loadConfig().Port != 1400 {
		t.Error("the update did not reach the live config and the file")
	}
	if !reflect.DeepEqual(changes, []string{"1337->1400"}) {
		t.Errorf("onChange saw %v", changes)
	}
}

func TestConfigStore_aFailedUpdateChangesNothing(t *testing.T) {
	withTempConfig(t)
	called := 0
	s := newConfigStore(loadConfig(), func(old, new Config) { called++ })
	err := s.Update(func(c *Config) error { c.Port = 1; return errors.New("refused") })
	if err == nil || s.Get().Port != defaultPort || called != 0 {
		t.Errorf("err=%v port=%d onChange=%d; a refused update must leave everything as it was", err, s.Get().Port, called)
	}
	// A write that fails (the config dir is now a file) leaves the live config as it was too.
	dir, _ := configDir()
	os.RemoveAll(dir)
	os.WriteFile(dir, []byte("not a dir"), 0o600)
	err = s.Update(func(c *Config) error { c.Port = 2; return nil })
	if err == nil || s.Get().Port != defaultPort || called != 0 {
		t.Errorf("err=%v port=%d onChange=%d after a failed write", err, s.Get().Port, called)
	}
}

func TestConfigStore_getReturnsACopy(t *testing.T) {
	withTempConfig(t)
	s := newConfigStore(loadConfig(), nil)
	c := s.Get()
	c.Beacons[0].Lines[0] = 99
	c.Beacons[0].Name = "changed"
	if got := s.Get().Beacons[0]; got.Lines[0] == 99 || got.Name == "changed" {
		t.Error("editing a Get() result changed the live config")
	}
}

func TestConfigStore_onlyAnAddressChangeRestartsTheServer(t *testing.T) {
	a := defaultConfig()
	b := a.clone()
	b.Beacons[0].Key = "port:x"
	if listenAddrChanged(a, b) {
		t.Error("a bind must not restart the HTTP server")
	}
	b.Port = 1338
	if !listenAddrChanged(a, b) {
		t.Error("a port change must restart it")
	}
	b = a.clone()
	b.Addr = "0.0.0.0"
	if !listenAddrChanged(a, b) {
		t.Error("an address change must restart it")
	}
}
```

- [ ] **Step 3: Run red.** Compile errors on `BeaconRecord`, `configStore`. **Disarms** once
  implemented: drop `mirrorFirstBeacon()` from `saveConfig` → `theMirrorIsWrittenFromTheFirstBeacon`
  red; skip the `max(seq, highest)` raise → `beaconSeqIsRaised…` red; accept `b01` in
  `normaliseBeacons` → `aMissingMalformed…` red; make `clone` a shallow copy →
  `getReturnsACopy` red; publish before `saveConfig` in `Update` → `aFailedUpdateChangesNothing`
  red on the second half.

- [ ] **Step 4: Fix the one existing test that no longer compiles.**
  `TestConfig_aV070FilePollsEveryTwoSecondsAsBefore` (`config_test.go:219-227`) compares
  `cfg != want`; `Config` now holds slices and is not comparable. Replace with field assertions on
  `Addr, Port, DashboardURL, APIToken, PollIntervalMs` and on `Beacons[0]` carrying line 2, flash 20,
  lobby on, 7 s, dire. Every other existing config test passes as it is (they read the mirror
  fields, which the loader fills from `Beacons[0]`).

- [ ] **Step 5: Implement** `config.go` and `config_store.go` per the Interfaces block. The id
  re-issue log line: `fmt.Sprintf("beacon %q: id %q is not usable, re-issued as %q", rec.Name, old, new)`.

- [ ] **Step 6: Verify** suite, vet (both), gofmt. Run `go test -count=1 -run Config ./cmd/blinkybeacon-tray/ -v | grep -c PASS`
  and quote it in the task report.

- [ ] **Step 7: Commit** `feat(tray): the config file holds a beacons list with public ids behind one writer; an old file loads as one beacon on its line`

**Acceptance:**
- The five fixtures load as stated; loading writes nothing; the new shape round-trips byte-for-byte
  after one save.
- `beacon_seq` only ever rises: raised to the highest id on load, and every re-issue advances it.
- `saveConfig` is called from `configStore.Update` and from the two `main.go` call sites that exist
  today (`main.go:31`, flags; the settings handler's call at `settings_handler.go:322` is T3's to
  remove); `grep -n saveConfig` in the task report lists every caller.
- `onChange` fires once per successful `Update`, after the write, with old and new.

**Size:** M (about 180 lines of code, 320 of test). **Seat:** `cwd-implementer-opus5`: the loader
of the operator's real config file, and the id counter's numerics.

**Plan gate:** rule 1: `TestConfig_idReissueSweep` (500 random lists, numbers to 10⁶, counters to
2×10⁶) beyond the pinned `b01`/`b1`-twice/`B2` and `beacon_seq: 1`; the pinned `3` in the design's
sample is the number of records plus nothing — a clean coincidence with nothing. Rule 2: the file
written is JSON with every list bounded by the record count; no network. Rule 3: the loader ends
with read-only smokes on `v050ConfigFile`, `v070ConfigFile`, `v080ConfigFile` (the exact bytes
dashboard-v0.8.0 writes, generated from that build's struct) and a file with keys omitted (`{"line_number":4,"beacons":[]}`),
plus `loadingWritesNothing`; the owner's own file is the real input, smoked in T10 step 1. Rule 4:
no git, no cgo. Rule 5: none.

---

## T3m — Mechanical: the settings tests move out of `watch_loop_test.go`

**Why:** `watch_loop_test.go` (1,574 lines) holds twenty settings tests
(`TestSettingsForm_*`, `TestSettingsPost_*`, `TestSettingsSavedPage_*`: lines 601–705, 1004–1211
and 1309–1422) beside the loop tests. In wave 2, T4 owns the loop tests and T8 owns the settings
tests; they must not share a file. This task moves text and changes nothing else. The
`TestConfig_token*`, `TestConfig_noTokenByDefault` and `TestSaveConfig_*` tests in the same file
are config tests T4 does not touch; they stay.

**Files:**
- Modify: `cmd/blinkybeacon-tray/watch_loop_test.go` (remove the moved functions)
- Modify: `cmd/blinkybeacon-tray/settings_handler_test.go` (append them, with the imports they need)

- [ ] **Step 1:** List every function to move: `grep -n '^func Test\(SettingsForm\|SettingsPost\|SettingsSavedPage\)' cmd/blinkybeacon-tray/watch_loop_test.go`.
  Expected: 20 functions. Also move the helpers only they use: `submitSettings`, `postSettings`
  (`watch_loop_test.go:118-140`). `withTempConfig` and `writeRawConfig` stay where they are (loop
  and config tests use them too).
- [ ] **Step 2:** Cut each function with its leading comment block and paste it at the end of
  `settings_handler_test.go` under a `// ---- moved from watch_loop_test.go` banner. Add the imports
  `settings_handler_test.go` now needs (`regexp`, `url`, `os`, `path/filepath`, `runtime` as `go vet`
  demands) and drop the ones `watch_loop_test.go` no longer needs.
- [ ] **Step 3: Verify** `go test -count=1 -race ./cmd/blinkybeacon-tray/` → `ok` with the same test
  count as before (`-v | grep -c '^=== RUN'` before and after: equal); `git diff --color-moved=zebra --stat`;
  and `git diff -U0 | grep '^[-+]' | grep -v '^[-+][-+]' | sort | uniq -c | awk '$1 != 2'` prints
  only import lines and the banner: every other removed line is added once.
- [ ] **Step 4: Commit** `test(tray): the settings tests live in settings_handler_test.go`

**Acceptance:** same suite count; no assertion changed (the uniq check); gofmt clean.
**Size:** S (moves only; about 450 lines moved). **Seat:** general-purpose, `model: "sonnet"`: a
file move from a complete brief.
**Plan gate:** rules 1–5 n/a; the reviewer checks the uniq line.

---

## T3 — The registry, the aggregate, the seam types, per-line watch state; `main.go` and `server.go` compat; the test-setup sweep

**Carries out:** §6 (the model's live half), §7.3 (`wanted`'s inputs: identify deadline, hand
state), §9.2 (the aggregate: connected = any, state = rank maximum over connected beacons), §10.2
step 2 (an unmatched record binds to the first light no record claims), §10.3 (a record survives
its light), §10.4 (an in-memory record for a first-seen light; the counter advances in memory),
§10.5 (tombstones; `restartCh` carries the restart, not the config), §11.2's server-side actions
(Identify, Forget, Take over as registry operations), §12 items 2–3 (the identity-checked mark:
red-team note 1), §14.2 (the seam types; the honest test budget). It replaces `AppState`'s single
beacon with a registry and keeps the type's name so the fifty-odd call sites change one line each.

**Files:**
- Rename: `cmd/blinkybeacon-tray/state.go` (147 lines) → `beacons.go` (`git mv`, then rewrite)
- Rename: `cmd/blinkybeacon-tray/state_test.go` (52 lines, 4 tests) → `beacons_test.go` (rewritten;
  the four tests become registry tests, the one expectation change §14.2 allows)
- Create: `cmd/blinkybeacon-tray/devices.go` (the seam: three types, no build tag)
- Modify: `cmd/blinkybeacon-tray/watcher.go:548-573` (`applyState` moves to `beacons.go`, per beacon;
  and two declarations T4 gives behaviour to, so wave 2's tasks can all name them:
  `const WatchThrottled WatchStatus = "throttled"` beside the others at `:95-109`, and
  `RetryInSeconds int` on `WatchDetail` at `:114-117`)
- Modify: `cmd/blinkybeacon-tray/main.go` (the store, the registry, the callbacks; the single-beacon
  goroutine removed; `fsbeacon` import dropped — `beacon_check.go` keeps the package imported)
- Modify: `cmd/blinkybeacon-tray/server.go:22-47` (`NewHTTPServer(app, addr, port)`; the handlers
  compile against the registry through one helper and keep today's replies — the `?beacon=` work is
  T6's)
- Modify: `cmd/blinkybeacon-tray/settings_handler.go:123-136, 294-341` (the handler holds `app`;
  `handlePost` writes through `app.updateConfig`; `onSave` goes)
- Modify: `cmd/blinkybeacon-tray/watch_loop.go:35-128` and `tray.go:89,97` (compat lines only, named in Step 6; T4 and T7 replace them)
- Setup lines in: `server_test.go` (3 `SetBeacon`, 3 `Get`), `watcher_test.go` (3 `SetBeacon`, 3 `Get`,
  8 `applyState`), `watch_loop_test.go` (23 `SetBeacon`, 22 `Get`, 3 `applyState`),
  `watch_loop_cadence_test.go` (2, 1), `settings_handler_test.go` (1 `SetBeacon`, 3 `onSave`),
  `integration_test.go` (3, 3). Measured by `grep -c` at `1e4784c`: **78 lines**, not the design's
  "about 35".

**Interfaces:**
- Consumes: T2's `configStore`, `BeaconRecord`, `Config.effectiveBeacons()`, `clone()`; T0's
  `publicID`, `publicIDNumber`.
- Produces (`devices.go`):

```go
// deviceSource is what the device manager talks to: the USB library on
// Windows and macOS, a fake in the suite.
type deviceSource interface {
	Enumerate() ([]deviceInfo, error)           // every beacon the layer can see, in its order
	Open(info deviceInfo) (deviceHandle, error) // open exactly that one
}

// deviceInfo is one enumerated path. Path is what Open takes; it is held in
// memory for an open handle and never written to the config.
type deviceInfo struct {
	Path      string
	Serial    string // "" when the light reports none
	UsagePage uint16 // a light with several collections shows one path per collection
	Usage     uint16
}

// deviceHandle is one open light.
type deviceHandle interface {
	Beacon                 // Flash, Spin, Stop, Close
	Key() string           // the match key, computed by the source (on macOS from the open handle)
	OnUnplug(func())       // registers the callback the keepalive fires when a write fails
	StartIdleWorker()      // begins the keepalive, so an unplug is noticed while idle
	Live() bool            // the last keepalive attempt succeeded (true from open)
}
```

  (`beacons.go`):

```go
// identifyFor is how long Identify flashes a light.
const identifyFor = 2 * time.Second

// beaconEntry is one beacon as the tray knows it right now: its record,
// whether that record is in the file yet, and the live half no file holds.
type beaconEntry struct {
	rec           BeaconRecord
	inMemory      bool          // seen, never saved (§10.4)
	handle        deviceHandle  // nil while unplugged
	state         StateValue    // what the light shows now
	hand          StateValue    // the last Spin/Flash/Stop by hand; what an unbound light returns to
	identifyUntil time.Time
	sameIdentity  bool          // a second light with this key was refused at the last enumerate
}

// beaconView is a read-only snapshot of one entry for the tray, the HTTP
// routes and the settings page.
type beaconView struct {
	ID, Name, Key string
	Lines         []int
	Light         BeaconRecord // a copy of the whole record; callers read the four light settings from it
	InMemory      bool
	Connected     bool
	State         StateValue // what the light shows
	Hand          StateValue // the last hand command; what an unbound light returns to
	Identifying   bool
	SameIdentity  bool
}

// lineWatch is what the loop last said about one line.
type lineWatch struct {
	status WatchStatus
	detail WatchDetail
	label  string
}

// AppState is the registry: every beacon the tray knows, what each shows,
// and what the dashboard loop last said about every line any of them follows.
type AppState struct {
	mu         sync.RWMutex
	entries    []*beaconEntry        // configured in config order, then in-memory in order of appearance
	seq        int                   // the live public-id counter; ≥ the file's beacon_seq
	tombstones map[string]bool       // public ids forgotten since start
	lines      map[int]lineWatch
	store      *configStore          // nil in tests that never save
	listenAddr atomic.Value          // string
	// onDrop is told every handle the registry has let go of, after the lock
	// is released; the device manager closes it. Nil until the manager runs.
	onDrop func(deviceHandle)
	// wake is nudged when something wants the loop to re-apply now (Identify's
	// end); buffered one deep, never blocks.
	wake chan struct{}
}

func NewAppState(store *configStore) *AppState // store may be nil

// --- config in and out
// syncConfig makes the configured entries match cfg.effectiveBeacons(): new
// records are added, known ones updated (a key the entry already holds is
// never blanked), configured records no longer present are forgotten
// (tombstoned; a handle they still held moves to a fresh in-memory entry),
// and every unmatched configured record (key "") absorbs the first in-memory
// entry in order of appearance: its handle, key, state, hand and identify
// deadline. seq is raised to cfg.BeaconSeq. The absorbing binds are saved
// through updateConfig in their own goroutine.
func (a *AppState) syncConfig(cfg Config)
// updateConfig is the one way anything in the tray writes the file: it runs
// fn through the store, then stamps BeaconSeq with the live counter so an id
// handed out in memory is reserved on disk by the next save of anything.
// A nil store makes it a no-op that returns nil.
func (a *AppState) updateConfig(fn func(*Config) error) error

// --- lights coming and going (the manager's calls)
// heldBy returns the handle bound to the entry with this key, nil when none
// or when that entry is unplugged.
func (a *AppState) heldBy(key string) deviceHandle
// bind gives a newly opened light to the registry: the entry with its key if
// one is unplugged, else the first unmatched configured entry (which takes
// the key and is saved), else a fresh in-memory entry with the next public
// id. A fresh handle shows idle. Returns the public id. It is an error to
// bind a key whose entry already holds a handle: callers decide live/replug
// first and call rebind.
func (a *AppState) bind(h deviceHandle) (id string, err error)
// rebind replaces the handle of the entry with h.Key(): the old handle is
// returned for the caller to close; state is reset to idle so the next tick
// re-applies the wanted state (a replug mid-pause resumes spinning).
func (a *AppState) rebind(h deviceHandle) (old deviceHandle, id string, err error)
// dropHandle marks the entry that holds exactly h unplugged (handle nil,
// state and hand idle) and hands h to onDrop. A handle no entry holds is
// ignored and false is returned: the identity check of §12 item 2.
func (a *AppState) dropHandle(h deviceHandle) bool
// markSameIdentity records that a second light with key was refused at this
// enumerate (true) or that none was (false).
func (a *AppState) markSameIdentity(key string, refused bool)

// --- driving (the HTTP routes, the tray, the loop)
// Command issues one hand command to one beacon: errBeaconGone when the id
// is unknown, errBeaconUnplugged when it has no handle, the device's error
// when the write fails (the handle is then dropped). On success state and
// hand are both s.
func (a *AppState) Command(id string, s StateValue) error
// CommandAll drives every connected beacon and reports how many were tried
// and how many failed; a failed one is dropped.
func (a *AppState) CommandAll(s StateValue) (tried, failed int)
// applyState drives one beacon to want only if it is connected and not
// already showing it; a failed write drops the handle. hand is untouched.
func applyState(app *AppState, id string, want StateValue)
// Identify flashes one connected beacon now and sets its deadline; the loop
// hands the light back at the deadline (it is nudged through Wake).
func (a *AppState) Identify(id string, now time.Time) error

// --- the settings page's actions (§11.2)
var (
	errBeaconGone      = errors.New("that beacon is no longer listed")
	errBeaconUnplugged = errors.New("that beacon is not plugged in")
	errBeaconInUse     = errors.New("that beacon is plugged in")
)
// Forget removes a configured, unplugged record and tombstones its id, then
// saves. errBeaconGone, errBeaconInUse, or an error for an in-memory id (it
// is not in the file; unplugging it is how it goes away).
func (a *AppState) Forget(id string) error
// TakeOver gives the configured, unplugged record keepID the key, handle,
// state, hand and identify deadline of the in-memory entry dropID, drops
// that entry (its id is never reused), and saves.
func (a *AppState) TakeOver(keepID, dropID string) error
// Promote writes an in-memory entry into the file with the record given
// (name, lines, light settings); the key and id are the entry's own.
func (a *AppState) Promote(id string, rec BeaconRecord) error
func (a *AppState) IsTombstone(id string) bool

// --- reading
func (a *AppState) Views() []beaconView
func (a *AppState) View(id string) (beaconView, bool)
// Resolve finds a beacon by public id, then by exact name. "" and whitespace
// never match.
func (a *AppState) Resolve(idOrName string) (beaconView, bool)
// Aggregate is §9.2: connected when any beacon is; state the rank maximum
// over connected beacons' states (idle when none).
func (a *AppState) Aggregate() (StateValue, bool)
// AggregateOf is the same over the named ids.
func (a *AppState) AggregateOf(ids []string) (StateValue, bool)
func (a *AppState) AnyConnectedFollowsALine() bool
// Lines is every distinct line any record follows, ascending.
func (a *AppState) Lines() []int
// keys is every entry's match key, for the manager's bookkeeping.
func (a *AppState) keys() []string
func (a *AppState) Seq() int
// releaseToHand hands a beacon back to hand control dark: hand becomes idle
// and the light is stopped if it was lit. The loop calls it once per beacon
// when it loses its binding, so the manual controls then stick.
func (a *AppState) releaseToHand(id string)

// --- per-line watch state (what SetWatchStatus/Detail/Line/Label were)
func (a *AppState) SetLineWatch(line int, s WatchStatus, d WatchDetail)
func (a *AppState) SetLineLabel(line int, label string)
func (a *AppState) LineWatch(line int) (WatchStatus, WatchDetail) // WatchOff, zero detail until the loop runs
func (a *AppState) LineLabel(line int) string
// ClearLineWatch forgets lines no record follows any more.
func (a *AppState) ClearLineWatch(keep []int)

func (a *AppState) SetListenAddr(addr string); func (a *AppState) ListenAddr() string
func (a *AppState) Wake() <-chan struct{}; func (a *AppState) nudge()

// rankOf and maxState are the one total order every aggregate uses:
// idle < flash < spin.
func rankOf(s StateValue) int
func maxState(a, b StateValue) StateValue
```

  `main.go` after T3 (the shape T5b completes; the `fsbeacon` import and the USB goroutine are
  gone):

```go
	cfg := loadConfig()
	// flags as today, saved with saveConfig as today (main.go:30-34)
	restartCh := make(chan struct{}, 1)
	var appState *AppState
	store := newConfigStore(cfg, func(old, new Config) {
		appState.syncConfig(new)
		if listenAddrChanged(old, new) {
			select {
			case restartCh <- struct{}{}:
			default:
			}
		}
	})
	appState = NewAppState(store)
	appState.syncConfig(cfg)

	startHTTP := func(c Config) { srv := NewHTTPServer(appState, c.Addr, c.Port); … as today … }
	startHTTP(cfg)
	go func() {
		for range restartCh {
			c := store.Get()
			time.Sleep(300 * time.Millisecond)
			… shutdown, startHTTP(c) as today …
		}
	}()
	// (the device manager goroutine is added here by T5b)
	go runWatchLoop(watchCtx, appState, &http.Client{Timeout: pollTimeout}, store.Get, watcherSettings)
	RunTray(appState, …, TrayCallbacks{
		OnSpin:  func() { appState.CommandAll(StateSpin) },
		OnFlash: func() { appState.CommandAll(StateFlash) },
		OnStop:  func() { appState.CommandAll(StateIdle) },
		OnSettings: as today,
		OnQuit: func() { stopWatch(); appState.CommandAll(StateIdle); … shutdown, close(quit) … },
	})
```

  (T5b adds the manager and makes `OnQuit` close every handle through it. Until then no light is
  opened: see the wave table.)

  `server.go` after T3: `NewHTTPServer(app *AppState, addr string, port int) *HTTPServer`; the
  settings handler is `&settingsHandler{app: app}`; `handleStatus` uses `app.Aggregate()`; the
  three control handlers call one helper `s.driveAll(w, want)`:

```go
func (s *HTTPServer) driveAll(w http.ResponseWriter, want StateValue) {
	tried, failed := s.state.CommandAll(want)
	switch {
	case tried == 0:
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon not connected"})
	case failed == tried:
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon disconnected"})
	default:
		sv, connected := s.state.Aggregate()
		s.writeJSON(w, http.StatusOK, statusResponse{State: sv, Connected: connected})
	}
}
```

  Today's six `server_test.go` assertions hold with this (one beacon: tried 1, failed 0 → 200 with
  the aggregate, which is that beacon's state).

  `settings_handler.go` after T3: `type settingsHandler struct { app *AppState; client; lineList; csrf… }`
  (no `onSave`); the tail of `handlePost` becomes

```go
	if err := h.app.updateConfig(func(c *Config) error {
		c.Addr, c.Port, c.DashboardURL, c.APIToken = addr, port, dashboardURL, token
		// until T8: the one-beacon fields still land on the first record
		b := &c.Beacons[0]
		b.Lines = []int{line}
		b.FlashSeconds, b.LobbyFlash, b.LobbyFlashSeconds, b.PauseSide = flashSeconds, lobbyFlash, lobbyFlashSeconds, pauseSide
		return nil
	}); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}
```

  with `saved := h.app.store.Get()` replacing `loadConfig()` at line 294 (the live config is the
  store's). The HTTP restart now comes from the store's `onChange`, not from the handler. The three
  tests that read `h.onSave` (`TestSettingsPost_savesTheLineWhetherOrNotThePickerRendered:243-250`,
  `TestSettingsPost_keepsAHandEditedPollInterval:779-787`,
  `TestSettingsPost_aFormFromAnEarlierProcess…:821-826`) instead assert on `store.Get()` after the
  post (and, for the refused form, that it is unchanged). The handler tests construct
  `&settingsHandler{app: newTestApp(t)}` or, when they plant a config file first,
  `&settingsHandler{app: newTestAppNoTemp(t)}` after the write (both helpers are in Step 1).

- [ ] **Step 1: The test helper every planted beacon goes through** (`beacons_test.go`):

```go
// fakeHandle wraps a Beacon fake (mockBeacon, countingBeacon, flashCounter)
// as a deviceHandle with a fixed key: what a test plants into the registry.
type fakeHandle struct {
	Beacon
	key      string
	onUnplug func()
	started  bool
	live     bool
}

func (h *fakeHandle) Key() string         { return h.key }
func (h *fakeHandle) OnUnplug(f func())   { h.onUnplug = f }
func (h *fakeHandle) StartIdleWorker()    { h.started = true }
func (h *fakeHandle) Live() bool          { return h.live }

// plant binds b as a light with key "port:p<n>" (n counting up per test) and
// returns its public id. With no configured record claiming it, it becomes an
// in-memory beacon; after a syncConfig of a config whose first record is
// unmatched, that record takes it — which is how every loop test's single
// beacon follows the line the test's Config names.
func plant(app *AppState, b Beacon) string {
	plantSeq++
	id, err := app.bind(&fakeHandle{Beacon: b, key: fmt.Sprintf("port:p%d", plantSeq), live: true})
	if err != nil {
		panic(err)
	}
	return id
}

var plantSeq int

// plantKeyed binds b as the light with exactly this key, so a configured
// record holding that key takes it: how every multi-beacon test wires its
// lights to Desk left and Desk right.
func plantKeyed(app *AppState, key string, b Beacon) string {
	id, err := app.bind(&fakeHandle{Beacon: b, key: key, live: true})
	if err != nil {
		panic(err)
	}
	return id
}

// newTestApp is a registry over a configStore on a fresh temp config dir.
// newTestAppNoTemp is the same over whatever withTempConfig and
// writeRawConfig/saveConfig have already put in place: a test that plants a
// file first calls withTempConfig, writes, then newTestAppNoTemp.
func newTestApp(t *testing.T) *AppState {
	t.Helper()
	withTempConfig(t)
	return newTestAppNoTemp(t)
}

func newTestAppNoTemp(t *testing.T) *AppState {
	t.Helper()
	store := newConfigStore(loadConfig(), nil)
	app := NewAppState(store)
	store.onChange = func(_, new Config) { app.syncConfig(new) }
	app.syncConfig(store.Get())
	return app
}
```

- [ ] **Step 2: Write the failing registry tests** (`beacons_test.go`; the four `state_test.go`
  tests become the first four)

```go
func TestRegistry_startsEmptyAndDisconnected(t *testing.T) {
	app := NewAppState(nil)
	if sv, connected := app.Aggregate(); sv != StateIdle || connected {
		t.Errorf("Aggregate() = %v,%v on a new registry, want idle,false", sv, connected)
	}
	if len(app.Views()) != 0 {
		t.Error("a new registry lists beacons")
	}
}

func TestRegistry_droppingTheOnlyHandleClearsToIdle(t *testing.T) {
	app := NewAppState(nil)
	b := &countingBeacon{}
	id := plant(app, b)
	if err := app.Command(id, StateSpin); err != nil {
		t.Fatal(err)
	}
	if !app.dropHandle(viewHandle(app, id)) {
		t.Fatal("dropHandle returned false for the handle the entry holds")
	}
	v, _ := app.View(id)
	if v.Connected || v.State != StateIdle {
		t.Errorf("after the drop: %+v, want unplugged and idle", v)
	}
	if sv, connected := app.Aggregate(); sv != StateIdle || connected {
		t.Errorf("Aggregate() = %v,%v, want idle,false", sv, connected)
	}
}

// viewHandle reaches the entry's handle for a test: the registry does not
// expose it, and the manager gets it back through dropHandle's argument.
func viewHandle(app *AppState, id string) deviceHandle {
	app.mu.RLock()
	defer app.mu.RUnlock()
	for _, e := range app.entries {
		if e.rec.ID == id {
			return e.handle
		}
	}
	return nil
}

func TestRegistry_commandSetsTheStateOfThatBeaconOnly(t *testing.T) {
	app := NewAppState(nil)
	a, b := plant(app, &countingBeacon{}), plant(app, &countingBeacon{})
	app.Command(a, StateSpin)
	va, _ := app.View(a)
	vb, _ := app.View(b)
	if va.State != StateSpin || vb.State != StateIdle {
		t.Errorf("states %v/%v, want spin/idle", va.State, vb.State)
	}
}

func TestRegistry_concurrentReadsAndWrites(t *testing.T) {
	app := NewAppState(nil)
	id := plant(app, &countingBeacon{})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() { defer wg.Done(); app.Command(id, StateSpin) }()
		go func() { defer wg.Done(); app.Aggregate() }()
		go func() { defer wg.Done(); app.Views() }()
	}
	wg.Wait()
}

func TestRegistry_aggregateIsTheRankMaximumOverConnectedBeacons(t *testing.T) {
	// The §9.2 table, pinned, then the sweep: every multiset of up to six
	// beacons, each connected or not and in one of three states.
	app := NewAppState(nil)
	if sv, c := app.Aggregate(); sv != StateIdle || c {
		t.Errorf("none plugged in: %v,%v", sv, c)
	}
	ids := []string{plant(app, &countingBeacon{}), plant(app, &countingBeacon{}), plant(app, &countingBeacon{})}
	app.Command(ids[0], StateFlash)
	app.Command(ids[1], StateSpin)
	if sv, c := app.Aggregate(); sv != StateSpin || !c {
		t.Errorf("flash+spin+idle: %v,%v, want spin,true", sv, c)
	}
	app.Command(ids[1], StateIdle)
	if sv, c := app.Aggregate(); sv != StateFlash || !c {
		t.Errorf("flash+idle+idle: %v,%v, want flash,true", sv, c)
	}
	// An unplugged beacon's last state is not a light anyone can see.
	app.dropHandle(viewHandle(app, ids[0]))
	if sv, c := app.Aggregate(); sv != StateIdle || !c {
		t.Errorf("unplugged flash + two idle: %v,%v, want idle,true", sv, c)
	}

	states := []StateValue{StateIdle, StateFlash, StateSpin}
	for n := 1; n <= 6; n++ {
		combos := 1
		for i := 0; i < n; i++ {
			combos *= 6 // 3 states × plugged or not
		}
		for c := 0; c < combos; c++ {
			app := NewAppState(nil)
			wantConnected, wantRank := false, 0
			x := c
			for i := 0; i < n; i++ {
				s, plugged := states[x%3], (x/3)%2 == 1
				x /= 6
				id := plant(app, &countingBeacon{})
				app.Command(id, s)
				if !plugged {
					app.dropHandle(viewHandle(app, id))
					continue
				}
				wantConnected = true
				if rankOf(s) > wantRank {
					wantRank = rankOf(s)
				}
			}
			sv, connected := app.Aggregate()
			if connected != wantConnected || (connected && rankOf(sv) != wantRank) || (!connected && sv != StateIdle) {
				t.Fatalf("n=%d combo=%d: Aggregate() = %v,%v, want rank %d connected %v", n, c, sv, connected, wantRank, wantConnected)
			}
		}
	}
}

func TestMaxState_isATotalOrder(t *testing.T) {
	for _, a := range []StateValue{StateIdle, StateFlash, StateSpin} {
		for _, b := range []StateValue{StateIdle, StateFlash, StateSpin} {
			if maxState(a, b) != maxState(b, a) {
				t.Errorf("maxState(%v,%v) is not commutative", a, b)
			}
			if maxState(a, a) != a {
				t.Errorf("maxState(%v,%v) != %v", a, a, a)
			}
		}
	}
	if maxState(StateFlash, StateIdle) != StateFlash || maxState(StateSpin, StateFlash) != StateSpin {
		t.Error("spin beats flash beats idle")
	}
}

func TestRegistry_aFirstSeenLightIsAnInMemoryBeaconFollowingNothing(t *testing.T) {
	app := newTestApp(t) // one configured, unmatched b1 following line 1
	id := plant(app, &countingBeacon{})
	if id != "b1" {
		t.Fatalf("the first light took id %q, want the unmatched b1", id)
	}
	v, _ := app.View("b1")
	if v.Key != "port:p"+fmt.Sprint(plantSeq) || v.InMemory || !reflect.DeepEqual(v.Lines, []int{1}) {
		t.Errorf("b1 after bind = %+v, want keyed, configured, following line 1", v)
	}
	waitFor(t, "the bind to be saved", func() bool { return app.store.Get().Beacons[0].Key == v.Key })
	second := plant(app, &countingBeacon{})
	w, _ := app.View(second)
	if second != "b2" || !w.InMemory || len(w.Lines) != 0 || w.Name != "" || w.Light.FlashSeconds != defaultFlashSeconds {
		t.Errorf("a second light = %+v (id %s), want in-memory b2, no lines, no name, default light", w, second)
	}
	if len(app.store.Get().Beacons) != 1 {
		t.Error("an in-memory beacon was written to the config")
	}
	if app.Seq() != 2 {
		t.Errorf("Seq() = %d, want 2: the counter advanced in memory", app.Seq())
	}
}

func TestRegistry_theLiveCounterIsReservedByTheNextSaveOfAnything(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{}) // b1
	plant(app, &countingBeacon{}) // in-memory b2
	app.updateConfig(func(c *Config) error { c.Port = 1400; return nil })
	if got := app.store.Get().BeaconSeq; got != 2 {
		t.Errorf("beacon_seq after an unrelated save = %d, want 2", got)
	}
}

func TestRegistry_syncKeepsAKeyTheConfigLacks(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{})
	v, _ := app.View("b1")
	stale := app.store.Get()
	stale.Beacons[0].Key = "" // a config published before the bind's save landed
	app.syncConfig(stale)
	w, _ := app.View("b1")
	if w.Key != v.Key || !w.Connected {
		t.Errorf("sync blanked the key or unplugged the light: %+v", w)
	}
}

func TestRegistry_syncUpdatesNamesLinesAndLightOfConfiguredRecords(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{})
	app.updateConfig(func(c *Config) error {
		c.Beacons[0].Name, c.Beacons[0].Lines, c.Beacons[0].FlashSeconds = "Desk left", []int{2, 5}, 45
		return nil
	})
	v, _ := app.View("b1")
	if v.Name != "Desk left" || !reflect.DeepEqual(v.Lines, []int{2, 5}) || v.Light.FlashSeconds != 45 || !v.Connected {
		t.Errorf("after the save: %+v", v)
	}
}

func TestRegistry_aRecordDroppedFromTheConfigIsTombstoned(t *testing.T) {
	app := newTestApp(t)
	app.updateConfig(func(c *Config) error {
		c.Beacons = append(c.Beacons, BeaconRecord{ID: "b2", Key: "port:z", Name: "Spare", FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth})
		c.BeaconSeq = 2
		return nil
	})
	if err := app.Forget("b2"); err != nil {
		t.Fatal(err)
	}
	if _, ok := app.View("b2"); ok || !app.IsTombstone("b2") || len(app.store.Get().Beacons) != 1 {
		t.Error("Forget must drop the record, tombstone the id and save")
	}
	if err := app.Forget("b2"); !errors.Is(err, errBeaconGone) {
		t.Errorf("forgetting a gone id: %v, want errBeaconGone", err)
	}
	// A forgotten id is never handed out again.
	if id := plant(app, &countingBeacon{}); id == "b2" {
		t.Error("b2 was reused after a Forget")
	}
}

func TestRegistry_forgetRefusesAPluggedInOrInMemoryBeacon(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{}) // b1, plugged
	if err := app.Forget("b1"); !errors.Is(err, errBeaconInUse) {
		t.Errorf("Forget on a plugged light: %v, want errBeaconInUse", err)
	}
	id := plant(app, &countingBeacon{}) // in-memory b2
	app.dropHandle(viewHandle(app, id))
	if _, ok := app.View(id); ok {
		t.Error("an unplugged in-memory beacon must vanish from the registry")
	}
}

func TestRegistry_takeOverMovesTheLightToTheOldRecord(t *testing.T) {
	app := newTestApp(t)
	app.updateConfig(func(c *Config) error {
		c.Beacons[0].Name, c.Beacons[0].Key, c.Beacons[0].Lines = "Desk right", "port:old", []int{2}
		return nil
	})
	// Desk right is unplugged (key port:old, no handle); a light appears in a new port.
	moved := &countingBeacon{}
	newID := plant(app, moved) // in-memory b2 with key port:p<n>
	app.Command(newID, StateSpin)
	app.Identify(newID, time.Now())
	if err := app.TakeOver("b1", newID); err != nil {
		t.Fatal(err)
	}
	v, ok := app.View("b1")
	if !ok || v.Name != "Desk right" || !v.Connected || v.Key != "port:p"+fmt.Sprint(plantSeq) || !v.Identifying || !reflect.DeepEqual(v.Lines, []int{2}) {
		t.Errorf("after Take over b1 = %+v", v)
	}
	if _, ok := app.View(newID); ok {
		t.Error("the new light's in-memory record survived the Take over")
	}
	if app.store.Get().Beacons[0].Key != v.Key {
		t.Error("the Take over was not saved")
	}
	app.Command("b1", StateFlash)
	if moved.flashes != 2 { // one from Identify, one now
		t.Errorf("the moved handle took %d flashes, want 2: it is driven as b1 at once", moved.flashes)
	}
	// The handle is not held twice: dropping it unplugs b1 only.
	app.dropHandle(viewHandle(app, "b1"))
	if v, _ := app.View("b1"); v.Connected {
		t.Error("b1 still connected after its handle was dropped")
	}
}

func TestRegistry_takeOverRefusesTheWrongPair(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{})     // b1, plugged, configured
	x := plant(app, &countingBeacon{}) // b2, in-memory
	if err := app.TakeOver("b1", x); !errors.Is(err, errBeaconInUse) {
		t.Errorf("Take over onto a plugged record: %v, want errBeaconInUse", err)
	}
	if err := app.TakeOver("b9", x); !errors.Is(err, errBeaconGone) {
		t.Errorf("Take over onto an unknown record: %v, want errBeaconGone", err)
	}
}

func TestRegistry_promoteWritesTheInMemoryRecord(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{})
	id := plant(app, &countingBeacon{})
	if err := app.Promote(id, BeaconRecord{Name: "Desk right", Lines: []int{2}, FlashSeconds: 20, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth}); err != nil {
		t.Fatal(err)
	}
	v, _ := app.View(id)
	cfg := app.store.Get()
	if v.InMemory || len(cfg.Beacons) != 2 || cfg.Beacons[1].ID != id || cfg.Beacons[1].Name != "Desk right" || cfg.Beacons[1].Key != v.Key || cfg.BeaconSeq != 2 {
		t.Errorf("after Promote: view %+v, config %+v", v, cfg.Beacons)
	}
}

func TestRegistry_dropHandleIgnoresAHandleNoEntryHolds(t *testing.T) {
	app := NewAppState(nil)
	var dropped []deviceHandle
	app.onDrop = func(h deviceHandle) { dropped = append(dropped, h) }
	id := plant(app, &countingBeacon{})
	h := viewHandle(app, id)
	if !app.dropHandle(h) || app.dropHandle(h) {
		t.Error("the first drop must be taken and the second ignored")
	}
	if len(dropped) != 1 || dropped[0] != h {
		t.Errorf("onDrop saw %v, want exactly the one handle once", dropped)
	}
	other := &fakeHandle{Beacon: &countingBeacon{}, key: "port:never"}
	if app.dropHandle(other) {
		t.Error("a handle that was never bound must be ignored")
	}
}

func TestRegistry_aDuplicateKeyInTheFileBindsTheFirstRecordOnly(t *testing.T) {
	app := newTestApp(t)
	app.updateConfig(func(c *Config) error {
		c.Beacons[0].Key = "port:dup"
		c.Beacons = append(c.Beacons, BeaconRecord{ID: "b2", Key: "port:dup", Name: "Copy", FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth})
		c.BeaconSeq = 2
		return nil
	})
	id, err := app.bind(&fakeHandle{Beacon: &countingBeacon{}, key: "port:dup", live: true})
	if err != nil || id != "b1" {
		t.Fatalf("bind = %q, %v; want b1", id, err)
	}
	v2, _ := app.View("b2")
	if v2.Connected {
		t.Error("the second record with the same key must stay unplugged")
	}
	if held := app.heldBy("port:dup"); held == nil {
		t.Error("heldBy must find the first record's handle")
	}
}

func TestRegistry_rebindResetsToIdleAndReturnsTheOldHandle(t *testing.T) {
	app := NewAppState(nil)
	id := plant(app, &countingBeacon{})
	app.Command(id, StateSpin)
	old := viewHandle(app, id)
	fresh := &fakeHandle{Beacon: &countingBeacon{}, key: old.Key(), live: true}
	got, gotID, err := app.rebind(fresh)
	if err != nil || got != old || gotID != id {
		t.Fatalf("rebind = %v,%q,%v", got, gotID, err)
	}
	v, _ := app.View(id)
	if v.State != StateIdle || !v.Connected || viewHandle(app, id) != fresh {
		t.Errorf("after rebind: %+v", v)
	}
	if _, err := app.bind(fresh); err == nil {
		t.Error("bind must refuse a key whose entry holds a handle")
	}
}

func TestRegistry_identifyFlashesNowAndNudgesTheLoop(t *testing.T) {
	app := NewAppState(nil)
	b := &countingBeacon{}
	id := plant(app, b)
	now := time.Now()
	if err := app.Identify(id, now); err != nil {
		t.Fatal(err)
	}
	v, _ := app.View(id)
	if b.flashes != 1 || v.State != StateFlash || !v.Identifying {
		t.Errorf("Identify: flashes=%d view=%+v", b.flashes, v)
	}
	select {
	case <-app.Wake():
		t.Error("Identify must not nudge before its deadline")
	default:
	}
	app.dropHandle(viewHandle(app, id))
	if err := app.Identify(id, now); !errors.Is(err, errBeaconUnplugged) {
		t.Errorf("Identify on an unplugged beacon: %v", err)
	}
	if err := app.Identify("b77", now); !errors.Is(err, errBeaconGone) {
		t.Errorf("Identify on an unknown id: %v", err)
	}
}

func TestRegistry_resolveByIdThenByExactName(t *testing.T) {
	app := newTestApp(t)
	plant(app, &countingBeacon{})
	app.updateConfig(func(c *Config) error { c.Beacons[0].Name = "Desk left"; return nil })
	for _, q := range []string{"b1", "Desk left"} {
		if v, ok := app.Resolve(q); !ok || v.ID != "b1" {
			t.Errorf("Resolve(%q) = %+v,%v", q, v, ok)
		}
	}
	for _, q := range []string{"", "  ", "B1", "desk left", "Desk left ", "b2"} {
		if _, ok := app.Resolve(q); ok {
			t.Errorf("Resolve(%q) matched, want no match", q)
		}
	}
}

func TestRegistry_perLineWatchState(t *testing.T) {
	app := NewAppState(nil)
	if s, _ := app.LineWatch(3); s != WatchOff {
		t.Errorf("LineWatch before the loop runs = %v, want off", s)
	}
	app.SetLineWatch(3, WatchOK, WatchDetail{NoDraftData: true})
	app.SetLineLabel(3, "Line C")
	if s, d := app.LineWatch(3); s != WatchOK || !d.NoDraftData {
		t.Error("line 3's status did not land")
	}
	if app.LineLabel(3) != "Line C" || app.LineLabel(4) != "" {
		t.Error("labels are per line")
	}
	app.ClearLineWatch([]int{4})
	if s, _ := app.LineWatch(3); s != WatchOff || app.LineLabel(3) != "" {
		t.Error("a line nobody follows is forgotten")
	}
}

func TestRegistry_linesAndTheS14Hook(t *testing.T) {
	app := newTestApp(t)
	app.updateConfig(func(c *Config) error {
		c.Beacons[0].Lines = []int{3, 1}
		c.Beacons = append(c.Beacons, BeaconRecord{ID: "b2", Key: "port:z", Name: "Spare", Lines: []int{1, 2}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth})
		c.BeaconSeq = 2
		return nil
	})
	if !reflect.DeepEqual(app.Lines(), []int{1, 2, 3}) {
		t.Errorf("Lines() = %v", app.Lines())
	}
	if app.AnyConnectedFollowsALine() {
		t.Error("nothing is plugged in")
	}
	app.bind(&fakeHandle{Beacon: &countingBeacon{}, key: "port:z", live: true})
	if !app.AnyConnectedFollowsALine() {
		t.Error("Spare is plugged in and follows two lines")
	}
}
```

- [ ] **Step 3: Run red.** Compile errors. **Disarms** once implemented: `dropHandle` marking the
  first entry rather than the one holding `h` → `dropHandleIgnoresAHandleNoEntryHolds` red;
  `Aggregate` counting unplugged states → the sweep red; `syncConfig` copying `rec.Key` unconditionally
  → `syncKeepsAKeyTheConfigLacks` red; `TakeOver` leaving the handle on the dropped entry →
  `takeOverMovesTheLight…` red (`moved.flashes` is 1); `updateConfig` not stamping `BeaconSeq` →
  `theLiveCounterIsReserved…` red; `Resolve` matching names case-insensitively → red.

- [ ] **Step 4: Implement `beacons.go` and `devices.go`.** Rules the code keeps:
  - Every hook (`onDrop`, the bind-save) is called after `a.mu` is released; the bind-save runs as
    `go a.updateConfig(...)` and logs its error (`log.Printf("could not save beacon %s's key: %v", …)`).
  - `syncConfig` runs under `a.mu` and never calls the store; `updateConfig` never holds `a.mu`
    while calling the store (it reads `a.seq` under the lock, releases, then calls `store.Update`
    with a wrapper that runs `fn` and then sets `c.BeaconSeq = max(c.BeaconSeq, seq)`).
  - `Identify` issues `Flash` outside the lock (a USB write under the registry lock would stall
    every reader for the write's duration), re-locks to set state and the deadline, and starts
    `time.AfterFunc(identifyFor, a.nudge)`.
  - `Views()` returns deep copies (`Lines` copied).
  - `entries` order: configured records in config order, then in-memory in order of appearance;
    `syncConfig` re-sorts the configured prefix to the config's order.

- [ ] **Step 5: `main.go`, `server.go`, `settings_handler.go` compat** as the Interfaces block shows.
  `main.go` loses the `fsbeacon` import and the USB goroutine (lines 85–110) and the per-callback
  beacon handling (lines 120–178 become the four `CommandAll` lines). It must compile under
  `GOOS=windows`.

- [ ] **Step 6: The sweep.** Apply, then read every changed line:

```bash
cd cmd/blinkybeacon-tray
# plants
sed -i -E 's/(\w+)\.SetBeacon\((&?\w+\{\}|\w+)\)/plant(\1, \2)/' server_test.go watcher_test.go watch_loop_test.go watch_loop_cadence_test.go settings_handler_test.go integration_test.go
# aggregate reads: three returns become two
sed -i -E 's/(\w+), _, _ := (\w+)\.Get\(\)/\1, _ := \2.Get()/; s/_, (connected|_), _ := (\w+)\.Get\(\)/_, \1 := \2.Get()/; s/\.Get\(\)/.Aggregate()/' server_test.go watcher_test.go watch_loop_test.go watch_loop_cadence_test.go integration_test.go
# applyState per beacon: every test that calls it planted exactly one beacon, b1
sed -i -E 's/applyState\((\w+), State/applyState(\1, "b1", State/' watcher_test.go watch_loop_test.go
# the registries: NewAppState takes a store
sed -i 's/NewAppState()/NewAppState(nil)/' server_test.go watcher_test.go watch_loop_test.go watch_loop_cadence_test.go integration_test.go settings_handler_test.go
```

  Then by hand: `server_test.go:25,46,61,83,105,125` `NewHTTPServer(state, "127.0.0.1", 1337, nil)`
  → `NewHTTPServer(state, "127.0.0.1", 1337)`. Every `&settingsHandler{…}` literal in a test
  (`settings_handler_test.go`, `lines_cache_test.go` and the tests T3m moved there: 15 + 9 + 20
  sites) gains an `app:` field by one rule: a test that calls `withTempConfig(t)` and then plants
  a file (`saveConfig(...)` or `writeRawConfig(...)`) builds the handler **after** the write with
  `app: newTestAppNoTemp(t)`; every other test uses `app: newTestApp(t)` and drops its own
  `withTempConfig(t)` line (the helper calls it). The three `onSave` tests as described above.
  `state_test.go` is gone.
  `TestWatchLoop_switchingLineForgetsTheOldLinesDraft` and every loop test keep working because
  the loop calls `app.syncConfig(c)` each tick — **that line is T4's**; until T4, T3 adds it to
  today's loop at `watch_loop.go:35` (`c := cfg(); app.syncConfig(c)`) and changes the loop's
  five registry calls: `app.SetWatchLine(c.LineNumber)` → removed; `applyState(app, state)` →
  `applyState(app, "b1", state)` (today's loop drives the first record; T4 generalises);
  `app.SetWatchLabel(c.LineNumber, ls.Label)` → `app.SetLineLabel(c.LineNumber, ls.Label)`;
  `app.WatchStatus()` → `s, _ := app.LineWatch(c.LineNumber)`; `app.SetWatchDetail(detail);
  app.SetWatchStatus(status)` → `app.SetLineWatch(c.LineNumber, status, detail)`. The tray
  (`tray.go:89,97`) reads the first configured beacon's first line the same way until T7:
  `line := firstLine(state)` helper in `beacons.go` returning `Lines()[0]` or 0.
  `TestAppState_watchLineDefaultsToZero`/`watchLineRoundTrips` (`watch_loop_test.go:1213-1225`) and
  `TestAppState_watchLabelDoesNotOutliveItsLine` (`settings_handler_test.go:359-374`) are about
  the single `WatchLine`; they are rewritten as `TestRegistry_perLineWatchState` above (the one
  expectation change beyond `state_test.go`; recorded here).

- [ ] **Step 7: Verify.** Full suite green; `go vet` Linux and `GOOS=windows`; `gofmt -l` empty.
  Report: `git diff --stat`, the count of `t.Error`/`t.Fatal` lines before and after in the six
  swept files (`grep -c 't\.\(Error\|Fatal\)' …`): equal, except `beacons_test.go` replacing
  `state_test.go` and the three `WatchLine`/`WatchLabel` tests.

- [ ] **Step 8: Commit** in two commits so the reviewer can hold them: `feat(tray): a registry of
  beacons replaces the single beacon — records, handles, per-beacon state, the aggregate, per-line
  watch state` (beacons.go, devices.go, beacons_test.go, main.go, server.go, settings_handler.go,
  watch_loop.go, tray.go, watcher.go) and `test(tray): every test plants its beacon into the
  registry` (the sweep).

**Acceptance:**
- Every assertion of the swept tests is unchanged (the `t.Error`/`t.Fatal` count), except the
  rewritten `state_test.go` and the three `WatchLine`/`WatchLabel` tests, named here.
- The aggregate sweep covers 6¹+…+6⁶ = 55,986 combinations; the §9.2 table rows are pinned
  separately.
- `dropHandle` is identity-checked; `onDrop` sees a handle at most once.
- No registry method calls the store under `a.mu`; `go test -race` is clean.
- `main.go` compiles under `GOOS=windows` with no `fsbeacon` import; the tray runs with zero beacons.

**Size:** L (about 450 lines of code, 420 of test, 80 swept lines). **Seat:** `cwd-implementer-opus5`:
the aggregate's numerics, the operator's config written through `updateConfig`, and locking the
plan can state but not prove.

**Plan gate:** rule 1: the aggregate sweep over every multiset of up to six (connected, state)
pairs, and `TestMaxState_isATotalOrder`; pinned values are the §9.2 table (none plugged → idle,false;
any plugged → spin > flash > idle over connected). Known clean coincidence: with one beacon the
aggregate equals that beacon's state, which is why the six `server_test.go` tests pass unchanged —
it is the design's intent, not an accident, but a reviewer should know the single-beacon case
cannot tell the aggregate from a pass-through; the sweep does. Rule 2: the only payload this task
sends is today's `/status` and control replies, unchanged in shape. Rule 3: n/a. Rule 4: none.
Rule 5: none.

---

## T4 — Per-pair watchers, the combine, one fetch per distinct line, the per-line 429 backoff, and Identify's wake

**Carries out:** §7.1 (one `Watcher` per (beacon, line) pair, `Decide` unchanged), §7.2 (the
combine and its properties), §7.3 (`wanted`), §8.1 (the tick: distinct lines fetched once,
concurrently; per-line status; the 401 backoff for the whole loop, the 404 per line; the 429
backoff per line with `Retry-After`), §8.2 (requests scale with lines), §10.3 (an unplugged
beacon's watchers keep running), §5.4 (Identify hands the light back at its deadline), the §7.2
closing paragraph (F2: a pause that lifts inside the flash window hands the light back to the
flash — pinned), and the `throttled` word everywhere the design names it (§8.1, §9.3, §11.1).

**Files:**
- Modify: `cmd/blinkybeacon-tray/watch_loop.go` (140 lines; rewritten around the tick)
- Modify: `cmd/blinkybeacon-tray/watcher.go` (`WatchThrottled`, `WatchDetail.RetryInSeconds`,
  `pollError.RetryAfter`, `pollFailureStatus`, `retryAfterDelay`, `beaconWatcherSettings`,
  `watchStatusLabel`'s new case)
- Modify: `cmd/blinkybeacon-tray/lines.go:104-113` (`getJSON` keeps the `Retry-After` header on the error)
- Create: `cmd/blinkybeacon-tray/combine.go`, `cmd/blinkybeacon-tray/combine_test.go`
- Modify: `cmd/blinkybeacon-tray/watch_loop_test.go`, `watch_loop_cadence_test.go`, `watcher_test.go`
  (new tests; the `settings` hook's type in six helper lines)
- Modify: `cmd/blinkybeacon-tray/main.go` (one line: the `settings` argument becomes
  `beaconWatcherSettings`)

**Interfaces:**
- Consumes: T3's `AppState` (`syncConfig`, `Views`, `Lines`, `applyState`, `SetLineWatch`,
  `SetLineLabel`, `ClearLineWatch`, `Wake`, `releaseToHand`, `WatchThrottled`,
  `WatchDetail.RetryInSeconds`), T2's `effectiveBeacons`; the test helpers `plantKeyed`,
  `viewHandle`, `newTestApp` from T3's `beacons_test.go`.
- Produces:

```go
// combine.go — §7.2: a beacon shows the highest-ranked answer among its pairs.
// No pairs is idle. The order is the one maxState keeps: idle < flash < spin.
func combine(answers []StateValue) StateValue

// watcher.go — WatchThrottled ("throttled": reached, answered 429, this line
// waits) and WatchDetail.RetryInSeconds (whole seconds until the line is
// asked again) are declared by T3; this task gives them their behaviour.

type pollError struct {
	Status     int
	RetryAfter string // the Retry-After header, verbatim ("" when absent)
	msg        string
}

// The two numbers of the 429 backoff: when Retry-After is absent or
// unreadable, and the longest any header is honoured for.
const (
	throttleDefault = 10 * time.Second
	throttleCap     = 30 * time.Second
)

// retryAfterDelay reads a Retry-After header: a whole number of seconds, or
// an HTTP date (RFC 7231 §7.1.1.1: IMF-fixdate, RFC 850, asctime — what
// http.ParseTime accepts). A past date or a negative number is 0; anything
// unreadable, "+5" and "1.5" included, is throttleDefault; the result is
// capped at throttleCap.
func retryAfterDelay(header string, now time.Time) time.Duration

// beaconWatcherSettings is one beacon's WatcherSettings: its four light
// settings out of its record, the loop's cadence out of the config, each
// defaulted where invalid as watcherSettings always has.
func beaconWatcherSettings(cfg Config, b BeaconRecord) WatcherSettings
// watcherSettings is the first beacon's: what every build passed before
// beacons were several, kept for the tests and callers that mean exactly that.
func watcherSettings(cfg Config) WatcherSettings

// watch_loop.go
func runWatchLoop(ctx context.Context, app *AppState, client *http.Client,
	cfg func() Config, settings func(Config, BeaconRecord) WatcherSettings)
```

  (`AppState.releaseToHand`, `Views()`'s `Hand` field and `plantKeyed` are T3's; this task is
  their first caller.)

  The tick, as the loop runs it (the shape the implementer transcribes):

```go
type pairKey struct {
	beacon string
	line   int
}

type linePair struct {
	w   *Watcher
	set WatcherSettings
}

type lineResult struct {
	ls  *LineState
	err error
}

func runWatchLoop(ctx context.Context, app *AppState, client *http.Client, cfg func() Config, settings func(Config, BeaconRecord) WatcherSettings) {
	pairs := map[pairKey]*linePair{}
	driven := map[string]bool{}      // beacons the loop has driven since it bound
	throttled := map[int]time.Time{} // per line: not asked again before this moment
	lastTarget := ""
	for {
		c := cfg()
		app.syncConfig(c)
		views := app.Views()
		lines := app.Lines()
		app.ClearLineWatch(lines)
		interval := settings(c, c.effectiveBeacons()[0]).PollInterval

		unbound := WatchStatus("")
		if _, err := dashboardAPIURL(c.DashboardURL, linesListPath); err != nil {
			unbound = WatchOff
		} else if strings.TrimSpace(c.APIToken) == "" {
			unbound = WatchNoToken
		}

		anyRejected := false
		if unbound != "" {
			// Hand every beacon the loop was driving back dark, once; then
			// leave the hand controls alone (today's rule, per beacon).
			for id := range driven {
				app.releaseToHand(id)
				delete(driven, id)
			}
			pairs = map[pairKey]*linePair{}
			lastTarget = ""
			for _, l := range lines {
				app.SetLineWatch(l, unbound, WatchDetail{})
			}
		} else {
			if target := c.DashboardURL + "\x00" + c.APIToken; target != lastTarget {
				pairs = map[pairKey]*linePair{} // retargeted: no assumptions carry over
				lastTarget = target
				log.Printf("Watching %s, lines %v", c.DashboardURL, lines)
			}
			now := time.Now()
			results := fetchLines(ctx, client, c, lines, throttled, now)
			now = time.Now() // one clock reading for every Decide this tick

			// Per-line status, from the result alone; every pair on a line agrees.
			for _, l := range lines {
				r := results[l]
				status, detail := WatchOK, WatchDetail{}
				if r.err != nil {
					status = pollFailureStatus(r.err)
					var pe *pollError
					if status == WatchThrottled && errors.As(r.err, &pe) {
						throttled[l] = now.Add(retryAfterDelay(pe.RetryAfter, now))
					}
					if status == WatchThrottled {
						detail.RetryInSeconds = int(math.Ceil(throttled[l].Sub(now).Seconds()))
					}
					if status == WatchRejected {
						anyRejected = true
					}
				} else {
					detail = watchDetail(r.ls)
					app.SetLineLabel(l, r.ls.Label)
				}
				if prev, _ := app.LineWatch(l); prev != status {
					if r.err != nil {
						log.Printf("Dashboard feed, line %d, %s: %v", l, status, r.err)
					} else {
						log.Printf("Dashboard feed, line %d: %s", l, status)
					}
				}
				app.SetLineWatch(l, status, detail)
			}

			// Pairs: built, rebuilt on a light-settings change, kept on a
			// cadence-only change, dropped when no longer followed.
			live := map[pairKey]bool{}
			for _, v := range views {
				set := settings(c, v.Light)
				for _, l := range v.Lines {
					k := pairKey{v.ID, l}
					live[k] = true
					if p := pairs[k]; p == nil || p.set.withoutPollInterval() != set.withoutPollInterval() {
						pairs[k] = &linePair{w: NewWatcher(set), set: set}
					} else {
						p.set = set
					}
				}
			}
			for k := range pairs {
				if !live[k] {
					delete(pairs, k)
				}
			}

			// Decide per pair, combine per beacon, apply per beacon.
			for _, v := range views {
				if len(v.Lines) == 0 {
					continue
				}
				var answers []StateValue
				for _, l := range v.Lines {
					r := results[l]
					var st StateValue
					if r.err != nil {
						st, _ = pairs[pairKey{v.ID, l}].w.Decide(now, nil)
					} else {
						st, _ = pairs[pairKey{v.ID, l}].w.Decide(now, r.ls)
					}
					answers = append(answers, st)
				}
				want := combine(answers)
				if v.Identifying {
					want = StateFlash
				}
				applyState(app, v.ID, want)
				driven[v.ID] = true
			}
		}

		// Every beacon the loop does not drive — unbound by config, or every
		// beacon while the loop is unbound — returns to its hand state when
		// an Identify ends; otherwise applyState is a no-op.
		for _, v := range views {
			if unbound == "" && len(v.Lines) > 0 {
				continue
			}
			want := v.Hand
			if v.Identifying {
				want = StateFlash
			}
			applyState(app, v.ID, want)
		}

		status := WatchOK
		if anyRejected {
			status = WatchRejected
		}
		timer := time.NewTimer(pollDelay(interval, status))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		case <-app.Wake():
			timer.Stop()
		}
	}
}

// fetchLines asks the dashboard for every line once, concurrently, each
// bounded by pollTimeout; a throttled line is not asked and reports its wait.
func fetchLines(ctx context.Context, client *http.Client, c Config, lines []int, throttled map[int]time.Time, now time.Time) map[int]lineResult {
	results := make(map[int]lineResult, len(lines))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, l := range lines {
		if until, ok := throttled[l]; ok && now.Before(until) {
			results[l] = lineResult{err: &pollError{Status: http.StatusTooManyRequests, msg: fmt.Sprintf("line %d: waiting until %s", l, until.Format(time.TimeOnly))}}
			continue
		}
		delete(throttled, l)
		wg.Add(1)
		go func(l int) {
			defer wg.Done()
			pollCtx, cancel := context.WithTimeout(ctx, pollTimeout)
			ls, err := fetchLineState(pollCtx, client, c.DashboardURL, l, c.APIToken)
			cancel()
			mu.Lock()
			results[l] = lineResult{ls: ls, err: err}
			mu.Unlock()
		}(l)
	}
	wg.Wait()
	return results
}
```

  (`v.Hand` is T3's. A still-throttled line's synthetic
  `pollError` has an empty `RetryAfter` and is **not** used to re-arm `throttled[l]`: only a fresh
  429 from the wire does, which is why the loop checks `errors.As` on a result that came from a
  fetch; the implementer keeps a `fresh bool` on `lineResult` to tell them apart.)

- [ ] **Step 1: Write the failing tests.** `combine_test.go`:

```go
func TestCombine_theTenRowsOfTheDesign(t *testing.T) {
	cases := []struct {
		name string
		in   []StateValue
		want StateValue
	}{
		{"1 spin+idle", []StateValue{StateSpin, StateIdle}, StateSpin},
		{"2 spin+flash", []StateValue{StateSpin, StateFlash}, StateSpin},
		{"3 flash+idle", []StateValue{StateFlash, StateIdle}, StateFlash},
		{"4 flash+flash", []StateValue{StateFlash, StateFlash}, StateFlash},
		{"5 spin+spin", []StateValue{StateSpin, StateSpin}, StateSpin},
		{"6 idle+spin", []StateValue{StateIdle, StateSpin}, StateSpin},
		{"7 idle(after spin)+flash", []StateValue{StateIdle, StateFlash}, StateFlash},
		{"8 blind(idle)+spin", []StateValue{StateIdle, StateSpin}, StateSpin},
		{"9 idle+idle", []StateValue{StateIdle, StateIdle}, StateIdle},
		{"10 no pairs", nil, StateIdle},
	}
	for _, c := range cases {
		if got := combine(c.in); got != c.want {
			t.Errorf("%s: combine(%v) = %v, want %v", c.name, c.in, got, c.want)
		}
	}
}

func TestCombine_isTheRankMaximumOverEveryMultisetUpToSix(t *testing.T) {
	states := []StateValue{StateIdle, StateFlash, StateSpin}
	for n := 1; n <= 6; n++ {
		total := 1
		for i := 0; i < n; i++ {
			total *= 3
		}
		for c := 0; c < total; c++ {
			in := make([]StateValue, n)
			x, wantRank := c, 0
			for i := range in {
				in[i] = states[x%3]
				x /= 3
				if rankOf(in[i]) > wantRank {
					wantRank = rankOf(in[i])
				}
			}
			got := combine(in)
			if rankOf(got) != wantRank {
				t.Fatalf("combine(%v) = %v, want rank %d", in, got, wantRank)
			}
			// Order-free: the reverse gives the same answer.
			rev := make([]StateValue, n)
			for i := range in {
				rev[n-1-i] = in[i]
			}
			if combine(rev) != got {
				t.Fatalf("combine depends on order: %v vs %v", in, rev)
			}
			// Idempotent: adding the answer changes nothing.
			if combine(append(append([]StateValue{}, in...), got)) != got {
				t.Fatalf("combine is not idempotent on %v", in)
			}
			// Monotone: raising any one input never lowers the answer.
			for i := range in {
				for _, s := range states {
					if rankOf(s) >= rankOf(in[i]) {
						up := append([]StateValue{}, in...)
						up[i] = s
						if rankOf(combine(up)) < rankOf(got) {
							t.Fatalf("raising %v[%d] to %v lowered the answer", in, i, s)
						}
					}
				}
			}
		}
	}
}

func TestCombine_twoFlashDeadlinesRunToTheLater(t *testing.T) {
	// Row 4 through two real watchers: line A's draft ends at t0 with a 5 s
	// flash, line B's at t0+2s with a 5 s flash; the beacon flashes until t0+7s.
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	set := defaultSettings()
	set.FlashDuration = 5 * time.Second
	a, b := NewWatcher(set), NewWatcher(set)
	drafting := func(n int) *LineState { f := false; m := "m"; s := 0.5; return &LineState{N: n, Running: true, MatchID: &m, GameState: gameStateHeroSelection, DraftComplete: &f, SecondsSinceGSI: &s} }
	done := func(n int) *LineState { ls := drafting(n); tr := true; ls.DraftComplete = &tr; return ls }
	a.Decide(t0.Add(-time.Second), drafting(1))
	b.Decide(t0.Add(-time.Second), drafting(2))
	sa, _ := a.Decide(t0, done(1))
	sb, _ := b.Decide(t0, drafting(2))
	if combine([]StateValue{sa, sb}) != StateFlash {
		t.Fatal("A's draft end must flash the beacon")
	}
	sa, _ = a.Decide(t0.Add(2*time.Second), done(1))
	sb, _ = b.Decide(t0.Add(2*time.Second), done(2))
	for _, at := range []time.Duration{3 * time.Second, 6 * time.Second, 7 * time.Second} {
		sa, _ = a.Decide(t0.Add(at), done(1))
		sb, _ = b.Decide(t0.Add(at), done(2))
		if combine([]StateValue{sa, sb}) != StateFlash {
			t.Errorf("at t0+%v the beacon must still flash (B's deadline is t0+7s)", at)
		}
	}
	sa, _ = a.Decide(t0.Add(7*time.Second+time.Millisecond), done(1))
	sb, _ = b.Decide(t0.Add(7*time.Second+time.Millisecond), done(2))
	if combine([]StateValue{sa, sb}) != StateIdle {
		t.Error("after the later deadline the beacon is dark")
	}
}
```

  `watcher_test.go` (the F2 pin on one pair, and the backoff reader):

```go
func TestDecide_aPauseLiftingInsideTheFlashWindowHandsTheLightBackToTheFlash(t *testing.T) {
	// §7.2 row 7: the flash deadline is armed when the draft ends and a pause
	// never clears it. The user guide used to say the opposite.
	t0 := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	set := defaultSettings()
	set.FlashDuration = 60 * time.Second
	w := NewWatcher(set)
	m, s := "m", 0.5
	f, tr := false, true
	w.Decide(t0.Add(-time.Second), &LineState{N: 1, Running: true, MatchID: &m, GameState: gameStateHeroSelection, DraftComplete: &f, SecondsSinceGSI: &s})
	if st, _ := w.Decide(t0, &LineState{N: 1, Running: true, MatchID: &m, GameState: gameStateHeroSelection, DraftComplete: &tr, SecondsSinceGSI: &s}); st != StateFlash {
		t.Fatalf("draft end: %v, want flash", st)
	}
	paused := &LineState{N: 1, Running: true, MatchID: &m, GameState: "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS", Paused: true, DraftComplete: &tr, SecondsSinceGSI: &s}
	if st, _ := w.Decide(t0.Add(10*time.Second), paused); st != StateSpin {
		t.Fatalf("pause during the flash: %v, want spin", st)
	}
	live := *paused
	live.Paused = false
	if st, _ := w.Decide(t0.Add(30*time.Second), &live); st != StateFlash {
		t.Errorf("pause lifts at 30 s of a 60 s window: %v, want flash for what is left", st)
	}
	if st, _ := w.Decide(t0.Add(61*time.Second), &live); st != StateIdle {
		t.Errorf("after the window: %v, want idle", st)
	}
	// And with a window that ran out under the pause, the light goes dark.
	w2 := NewWatcher(defaultSettings()) // 15 s
	w2.Decide(t0.Add(-time.Second), &LineState{N: 1, Running: true, MatchID: &m, GameState: gameStateHeroSelection, DraftComplete: &f, SecondsSinceGSI: &s})
	w2.Decide(t0, &LineState{N: 1, Running: true, MatchID: &m, GameState: gameStateHeroSelection, DraftComplete: &tr, SecondsSinceGSI: &s})
	w2.Decide(t0.Add(5*time.Second), paused)
	if st, _ := w2.Decide(t0.Add(20*time.Second), &live); st != StateIdle {
		t.Errorf("pause lifts after the 15 s window: %v, want idle", st)
	}
}

func TestRetryAfterDelay_pinnedValues(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Duration{
		"":                                    throttleDefault, // absent
		"12":                                  12 * time.Second,
		"120":                                 throttleCap,
		"0":                                   0,
		"-5":                                  0,
		"+5":                                  throttleDefault, // not a Retry-After number
		"1.5":                                 throttleDefault,
		"soon":                                throttleDefault,
		now.Add(20 * time.Second).UTC().Format(http.TimeFormat): 20 * time.Second,
		now.Add(time.Hour).UTC().Format(http.TimeFormat):        throttleCap,
		now.Add(-time.Minute).UTC().Format(http.TimeFormat):     0,
		"Wed, 21 Oct 2015 07:28:00 GMT":       0, // the RFC's own example, long past
	}
	for h, want := range cases {
		if got := retryAfterDelay(h, now); got != want {
			t.Errorf("retryAfterDelay(%q) = %v, want %v", h, got, want)
		}
	}
}

func TestRetryAfterDelay_sweep(t *testing.T) {
	// Exact arithmetic over the whole range a header can name, well past the
	// pinned 12, 120 and the two bounds: the result is always
	// min(max(n, 0), cap) for a number and the same for a date's distance.
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	clamp := func(d time.Duration) time.Duration {
		if d < 0 {
			return 0
		}
		if d > throttleCap {
			return throttleCap
		}
		return d
	}
	for n := -100; n <= 100_000; n += 7 {
		want := clamp(time.Duration(n) * time.Second)
		if got := retryAfterDelay(strconv.Itoa(n), now); got != want {
			t.Fatalf("retryAfterDelay(%d) = %v, want %v", n, got, want)
		}
		at := now.Add(time.Duration(n) * time.Second)
		if got := retryAfterDelay(at.UTC().Format(http.TimeFormat), now); got != want {
			t.Fatalf("retryAfterDelay(date %+ds) = %v, want %v", n, got, want)
		}
	}
	if retryAfterDelay("9999999999999999999999", now) != throttleDefault {
		t.Error("an unparsable number lands on the default")
	}
}

func TestPollFailureStatus_a429IsThrottled(t *testing.T) {
	if got := pollFailureStatus(&pollError{Status: http.StatusTooManyRequests}); got != WatchThrottled {
		t.Errorf("429 → %v, want throttled", got)
	}
	for _, code := range []int{500, 502, 503} {
		if got := pollFailureStatus(&pollError{Status: code}); got != WatchLost {
			t.Errorf("%d → %v, want lost (unchanged)", code, got)
		}
	}
}

func TestWatchStatusLabel_throttledNamesTheLineAndTheWait(t *testing.T) {
	got := watchStatusLabel(WatchThrottled, 2, WatchDetail{RetryInSeconds: 12})
	if got != "▲ Dashboard: slowed by the dashboard (line 2, retry in 12s)" {
		t.Errorf("label = %q", got)
	}
}

func TestBeaconWatcherSettings_readsTheRecordNotTheMirror(t *testing.T) {
	cfg := Config{FlashSeconds: 15, PollIntervalMs: 750}
	b := BeaconRecord{FlashSeconds: 45, LobbyFlash: true, LobbyFlashSeconds: 9, PauseSide: pauseSideDire}
	got := beaconWatcherSettings(cfg, b)
	if got.FlashDuration != 45*time.Second || !got.LobbyFlash || got.LobbyFlashDuration != 9*time.Second || got.PauseSide != pauseSideDire || got.PollInterval != 750*time.Millisecond {
		t.Errorf("%+v", got)
	}
	if got := beaconWatcherSettings(cfg, BeaconRecord{}); got.FlashDuration != 15*time.Second || got.PauseSide != pauseSideBoth {
		t.Errorf("a zero record must land on the defaults: %+v", got)
	}
}
```

  `watch_loop_test.go` (the loop with several beacons and lines). The stub gains per-path answers:

```go
// linesDashboard answers GET /api/v0/lines/{n} per line: a status and body
// each, with a request count per line, safe to change between polls.
type linesDashboard struct {
	mu    sync.Mutex
	lines map[int]struct{ status int; body string; retryAfter string }
	polls map[int]int
}

func newLinesDashboard() *linesDashboard
func (d *linesDashboard) set(line int, status int, body string)             // answer this from now on
func (d *linesDashboard) throttle(line int, retryAfter string)              // 429 with that header from now on
func (d *linesDashboard) polls(line int) int
func (d *linesDashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) // parses the trailing number; unknown line → 404 {"error":"no such line"}

// twoBeaconConfig is Desk left on line 1 and Desk right on line 2, both
// keyed, bound to srv with the test token, polling as fast as the test asks.
func twoBeaconConfig(srv string) Config {
	c := defaultConfig()
	c.DashboardURL, c.APIToken = srv, testToken
	c.Beacons = []BeaconRecord{
		{ID: "b1", Key: "port:p-left", Name: "Desk left", Lines: []int{1}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
		{ID: "b2", Key: "port:p-right", Name: "Desk right", Lines: []int{2}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
	}
	c.BeaconSeq = 2
	return c
}

func stateOf(app *AppState, id string) StateValue {
	v, _ := app.View(id)
	return v.State
}

func TestWatchLoop_twoBeaconsTwoLines(t *testing.T) {
	// §14.3 step 5 through the loop: B pauses → Desk right spins alone; A
	// pauses too → both; A unpauses → only Desk right.
	dash := newLinesDashboard()
	dash.set(1, 200, livePayload)
	dash.set(2, 200, pausedPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	app.syncConfig(cfg)
	left, right := &countingBeacon{}, &countingBeacon{}
	plantKeyed(app, "port:p-left", left)
	plantKeyed(app, "port:p-right", right)
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "Desk right to spin", func() bool { return stateOf(app, "b2") == StateSpin })
	if stateOf(app, "b1") != StateIdle {
		t.Error("Desk left spun for Desk right's line")
	}
	dash.set(1, 200, pausedPayload)
	waitFor(t, "both to spin", func() bool { return stateOf(app, "b1") == StateSpin && stateOf(app, "b2") == StateSpin })
	dash.set(1, 200, livePayload)
	waitFor(t, "Desk left to stop", func() bool { return stateOf(app, "b1") == StateIdle })
	if stateOf(app, "b2") != StateSpin {
		t.Error("Desk right stopped when A unpaused")
	}
}

func TestWatchLoop_fetchesEachDistinctLineOnce(t *testing.T) {
	dash := newLinesDashboard()
	for _, l := range []int{1, 2} {
		dash.set(l, 200, livePayload)
	}
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	cfg.Beacons[0].Lines = []int{1, 2}
	cfg.Beacons = append(cfg.Beacons, BeaconRecord{ID: "b3", Key: "port:p3", Name: "Spare", Lines: []int{2}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth})
	cfg.BeaconSeq = 3
	app.syncConfig(cfg)
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)
	waitFor(t, "ten ticks", func() bool { return dash.polls(1) >= 10 })
	p1, p2 := dash.polls(1), dash.polls(2)
	if p2 < p1-1 || p2 > p1+1 {
		t.Errorf("line 2 (followed by three pairs) was polled %d times to line 1's %d; want one request per line per tick", p2, p1)
	}
}

func TestWatchLoop_oneBeaconTwoLines_aPauseOutranksAndTheFlashResumes(t *testing.T) {
	// §14.3 step 6: Desk left follows A and B. B pauses → spin. A's draft
	// ends under the pause → still spin (row 2). B unpauses inside the
	// window → flash for what is left (row 7), then dark.
	dash := newLinesDashboard()
	dash.set(1, 200, draftingPayload)
	dash.set(2, 200, pausedPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	cfg.Beacons = cfg.Beacons[:1]
	cfg.Beacons[0].Lines = []int{1, 2}
	app.syncConfig(cfg)
	b := &countingBeacon{}
	plantKeyed(app, "port:p-left", b)
	flash := 150 * time.Millisecond
	startWatchLoopWith(t, app, srv.Client(), func() Config { return cfg }, func(c Config, r BeaconRecord) WatcherSettings {
		s := beaconWatcherSettings(c, r)
		s.FlashDuration, s.PollInterval = flash, 5*time.Millisecond
		return s
	})
	waitFor(t, "the pause to spin", func() bool { return stateOf(app, "b1") == StateSpin })
	dash.set(1, 200, draftDonePayload) // A's draft ends under B's pause
	time.Sleep(flash / 3)
	if stateOf(app, "b1") != StateSpin || b.flashes != 0 {
		t.Fatalf("the pause must outrank the flash: state %v, flashes %d", stateOf(app, "b1"), b.flashes)
	}
	dash.set(2, 200, livePayload) // B unpauses inside the window
	waitFor(t, "the flash to resume", func() bool { return stateOf(app, "b1") == StateFlash })
	waitFor(t, "the window to run out", func() bool { return stateOf(app, "b1") == StateIdle })
	if b.flashes != 1 {
		t.Errorf("Flash was issued %d times, want 1 (the resume)", b.flashes)
	}
}

func TestWatchLoop_twoBeaconsOnOneLineFlashForTheirOwnLengths(t *testing.T) {
	// §14.3 step 7.
	dash := newLinesDashboard()
	dash.set(1, 200, draftingPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	cfg.Beacons[1].Lines = []int{1}
	app.syncConfig(cfg)
	short, long := &countingBeacon{}, &countingBeacon{}
	plantKeyed(app, "port:p-left", short)
	plantKeyed(app, "port:p-right", long)
	startWatchLoopWith(t, app, srv.Client(), func() Config { return cfg }, func(c Config, r BeaconRecord) WatcherSettings {
		s := beaconWatcherSettings(c, r)
		s.PollInterval = 5 * time.Millisecond
		if r.ID == "b1" {
			s.FlashDuration = 30 * time.Millisecond
		} else {
			s.FlashDuration = 300 * time.Millisecond
		}
		return s
	})
	waitFor(t, "the first poll", func() bool { return dash.polls(1) > 0 })
	dash.set(1, 200, draftDonePayload)
	waitFor(t, "both to flash", func() bool { return stateOf(app, "b1") == StateFlash && stateOf(app, "b2") == StateFlash })
	waitFor(t, "the short one to stop", func() bool { return stateOf(app, "b1") == StateIdle })
	if stateOf(app, "b2") != StateFlash {
		t.Error("the long flash stopped with the short one")
	}
	waitFor(t, "the long one to stop", func() bool { return stateOf(app, "b2") == StateIdle })
}

func TestWatchLoop_aBlindLineNeverDarkensAnotherLinesPause(t *testing.T) {
	// Row 8: line 1 answers 404, line 2 is paused; a beacon on both spins.
	dash := newLinesDashboard()
	dash.set(2, 200, pausedPayload) // line 1 unknown → 404
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	cfg.Beacons = cfg.Beacons[:1]
	cfg.Beacons[0].Lines = []int{1, 2}
	app.syncConfig(cfg)
	plantKeyed(app, "port:p-left", &countingBeacon{})
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)
	waitFor(t, "the beacon to spin", func() bool { return stateOf(app, "b1") == StateSpin })
	if s, _ := app.LineWatch(1); s != WatchNoLine {
		t.Errorf("line 1's status = %v, want no-line", s)
	}
	if s, _ := app.LineWatch(2); s != WatchOK {
		t.Errorf("line 2's status = %v, want ok", s)
	}
}

func TestWatchLoop_anUnpluggedBeaconsWatchersKeepRunningAndAReplugResumes(t *testing.T) {
	// §10.3 and §14.3 step 9.
	dash := newLinesDashboard()
	dash.set(2, 200, pausedPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	app.syncConfig(cfg)
	right := &countingBeacon{}
	plantKeyed(app, "port:p-right", right)
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)
	waitFor(t, "Desk right to spin", func() bool { return stateOf(app, "b2") == StateSpin })
	app.dropHandle(viewHandle(app, "b2"))
	v, _ := app.View("b2")
	if v.Connected || v.State != StateIdle {
		t.Fatalf("after the unplug: %+v", v)
	}
	time.Sleep(20 * time.Millisecond) // ticks go by with no light
	again := &countingBeacon{}
	plantKeyed(app, "port:p-right", again)
	waitFor(t, "the replugged light to spin", func() bool { return stateOf(app, "b2") == StateSpin && again.spins == 1 })
}

func TestWatchLoop_handsEveryDrivenBeaconBackDarkOnceWhenUnbound(t *testing.T) {
	dash := newLinesDashboard()
	dash.set(1, 200, pausedPayload)
	dash.set(2, 200, pausedPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	var mu sync.Mutex
	cfg := twoBeaconConfig(srv.URL)
	get := func() Config { mu.Lock(); defer mu.Unlock(); return cfg }
	app.syncConfig(cfg)
	left, right := &countingBeacon{}, &countingBeacon{}
	plantKeyed(app, "port:p-left", left)
	plantKeyed(app, "port:p-right", right)
	startWatchLoop(t, app, srv.Client(), get, 5*time.Millisecond)
	waitFor(t, "both to spin", func() bool { return stateOf(app, "b1") == StateSpin && stateOf(app, "b2") == StateSpin })
	mu.Lock()
	cfg.APIToken = "" // the token is cleared
	mu.Unlock()
	waitFor(t, "both to go dark", func() bool { return stateOf(app, "b1") == StateIdle && stateOf(app, "b2") == StateIdle })
	// Now by hand: the loop must leave it alone.
	app.Command("b1", StateSpin)
	time.Sleep(30 * time.Millisecond)
	if stateOf(app, "b1") != StateSpin || left.stops != 1 {
		t.Errorf("state %v, stops %d: the loop must hand back dark once and then leave the hand controls alone", stateOf(app, "b1"), left.stops)
	}
}

func TestWatchLoop_anUnboundBeaconIsDrivenByHandOnly(t *testing.T) {
	dash := newLinesDashboard()
	dash.set(1, 200, pausedPayload)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	app := NewAppState(nil)
	cfg := twoBeaconConfig(srv.URL)
	cfg.Beacons[1].Lines = nil // Desk right follows nothing
	app.syncConfig(cfg)
	plantKeyed(app, "port:p-left", &countingBeacon{})
	right := &countingBeacon{}
	plantKeyed(app, "port:p-right", right)
	app.Command("b2", StateFlash)
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)
	waitFor(t, "Desk left to spin", func() bool { return stateOf(app, "b1") == StateSpin })
	if stateOf(app, "b2") != StateFlash || right.stops != 0 || right.spins != 0 {
		t.Errorf("the loop touched an unbound beacon: %+v", right)
	}
}
```

  `watch_loop_cadence_test.go` (fake time; `runLoopFor` gains the app as a parameter and uses
  `beaconWatcherSettings`):

```go
func TestWatchLoop_a429BacksOffThatLineOnlyAndHonoursRetryAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dash := newLinesDashboard()
		dash.set(1, 200, livePayload)
		dash.set(2, 200, livePayload)
		app := NewAppState(nil)
		cfg := twoBeaconConfig("http://dashboard.test")
		cfg.PollIntervalMs = 2000
		app.syncConfig(cfg)
		plantKeyed(app, "port:p-left", &countingBeacon{})
		plantKeyed(app, "port:p-right", &countingBeacon{})
		dash.throttle(1, "12")
		runLoopFor(t, app, dash, cfg, 20*time.Second)
		// Line 1: asked at t=0 (the 429), then not again until t=12: 1 + (20-12)/2 = 5 polls.
		// Line 2: every 2 s for 20 s: 10 or 11 polls.
		if p := dash.polls(1); p < 4 || p > 6 {
			t.Errorf("throttled line polled %d times in 20 s, want about 5", p)
		}
		if p := dash.polls(2); p < 10 {
			t.Errorf("the other line polled %d times in 20 s, want its full cadence (10)", p)
		}
	})
}

func TestWatchLoop_aThrottledLineSaysSoWithItsWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dash := newLinesDashboard()
		dash.set(1, 200, livePayload)
		app := NewAppState(nil)
		cfg := boundConfig()
		app.syncConfig(cfg)
		plantKeyed(app, "port:p1", &countingBeacon{})
		dash.throttle(1, "") // no header: 10 s
		runLoopFor(t, app, dash, cfg, 3*time.Second)
		s, d := app.LineWatch(1)
		if s != WatchThrottled || d.RetryInSeconds < 7 || d.RetryInSeconds > 10 {
			t.Errorf("status %v, retry in %ds; want throttled with about 8 s left", s, d.RetryInSeconds)
		}
		if got := watchStatusLabel(s, 1, d); !strings.HasPrefix(got, "▲ Dashboard: slowed by the dashboard (line 1, retry in ") {
			t.Errorf("label %q", got)
		}
	})
}

func TestWatchLoop_a401OnAnyLineBacksOffTheWholeLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dash := newLinesDashboard()
		dash.set(1, 401, `{"error":"unauthorized"}`)
		dash.set(2, 200, livePayload)
		app := NewAppState(nil)
		cfg := twoBeaconConfig("http://dashboard.test")
		app.syncConfig(cfg)
		plantKeyed(app, "port:p-left", &countingBeacon{})
		plantKeyed(app, "port:p-right", &countingBeacon{})
		runLoopFor(t, app, dash, cfg, 65*time.Second)
		if p := dash.polls(2); p > 4 {
			t.Errorf("line 2 polled %d times in 65 s beside a rejected line; the token is one, so the whole loop backs off (want ≤ 3)", p)
		}
	})
}

func TestWatchLoop_a404And5xxKeepTheirCadence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dash := newLinesDashboard()
		dash.set(2, 503, `{"error":"down"}`) // line 1 unknown → 404
		app := NewAppState(nil)
		cfg := twoBeaconConfig("http://dashboard.test")
		app.syncConfig(cfg)
		plantKeyed(app, "port:p-left", &countingBeacon{})
		plantKeyed(app, "port:p-right", &countingBeacon{})
		runLoopFor(t, app, dash, cfg, 20*time.Second)
		if p := dash.polls(1); p < 10 {
			t.Errorf("a 404 line polled %d times in 20 s, want full cadence", p)
		}
		if p := dash.polls(2); p < 10 {
			t.Errorf("a 503 line polled %d times in 20 s, want full cadence (lost, unchanged)", p)
		}
		if s, _ := app.LineWatch(2); s != WatchLost {
			t.Errorf("503 → %v, want lost", s)
		}
	})
}

func TestWatchLoop_identifyEndsWithinTwoSecondsAtASlowCadence(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dash := newLinesDashboard()
		dash.set(1, 200, livePayload)
		app := NewAppState(nil)
		cfg := boundConfig()
		cfg.PollIntervalMs = 10000
		app.syncConfig(cfg)
		b := &countingBeacon{}
		plantKeyed(app, "port:p1", b)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			defer close(done)
			runWatchLoop(ctx, app, &http.Client{Transport: inProcessTransport{dash}}, func() Config { return cfg }, beaconWatcherSettings)
		}()
		time.Sleep(time.Second)
		synctest.Wait()
		app.Identify("b1", time.Now())
		if stateOf(app, "b1") != StateFlash || b.flashes != 1 {
			t.Fatal("Identify must flash at once")
		}
		time.Sleep(identifyFor + 500*time.Millisecond)
		synctest.Wait()
		if stateOf(app, "b1") != StateIdle || b.stops < 1 {
			t.Errorf("2.5 s after Identify at a 10 s cadence: state %v, stops %d; the loop must wake at the deadline", stateOf(app, "b1"), b.stops)
		}
		cancel()
		<-done
	})
}
```

- [ ] **Step 2: Run red.** Compile errors (`combine`, `WatchThrottled`, `beaconWatcherSettings`,
  the hook type). **Disarms** once implemented: `combine` returning the first answer →
  `theTenRows…` red on row 6; `fetchLines` fetching per pair instead of per line →
  `fetchesEachDistinctLineOnce` red; `throttled[l]` re-armed from the synthetic error → the 429
  test polls line 1 only once in 20 s (red: "about 5"); dropping the `Wake` case from the select
  → `identifyEnds…` red; `releaseToHand` leaving `hand` untouched → `handsEveryDrivenBeaconBack…`
  red (`left.stops` ≠ 1 is not it — the loop would re-spin it: state `spin` with `spins` 2).

- [ ] **Step 3: Implement** per the Interfaces block and the tick above. `getJSON` adds
  `RetryAfter: resp.Header.Get("Retry-After")` to the `pollError` it builds (`lines.go:109-112`).
  `retryAfterDelay`: `strings.TrimSpace`; `""` → default; all digits (an optional leading `-`
  allowed, nothing else) → `strconv.Atoi`, error → default; else `http.ParseTime` (error →
  default) and `t.Sub(now)`; clamp to `[0, throttleCap]`.

- [ ] **Step 4: The helper changes.** `pollEvery` returns `func(Config, BeaconRecord) WatcherSettings`;
  `startWatchLoopWith`'s parameter type follows; the two closures at `watch_loop_test.go:155,331`
  take `(c Config, r BeaconRecord)` and call `beaconWatcherSettings(c, r)`; `runLoopFor(t, app, h, cfg, d)`
  takes the app and passes `beaconWatcherSettings`; `watch_loop_cadence_test.go:163` passes
  `beaconWatcherSettings`; `main.go:116` passes `beaconWatcherSettings`. The T3 compat lines in
  the loop (`applyState(app, "b1", …)`, `c.LineNumber`) are gone with the rewrite.

- [ ] **Step 5: Verify** suite (`-race`), vet both, gofmt. Report the count of `TestWatchLoop_`
  tests before and after (57 before: 54 + 3 cadence).

- [ ] **Step 6: Commit** in two: `feat(tray): one watcher per beacon and line; a beacon shows the
  busiest of its lines; each line fetched once per tick` and `feat(tray): a line the dashboard
  slows down waits as long as it is told, up to 30 seconds, while the other lines keep their cadence`.

**Acceptance:**
- Every existing loop and `Decide` test passes with only the hook-type lines changed (named above).
- The ten rows and the 3¹…3⁶ sweep (1,092 combinations, each also checked reversed, idempotent and
  monotone) pass; row 7's resume is pinned on one pair.
- A tick makes one request per distinct line; a 429 waits per line for `Retry-After` clamped to
  [0, 30 s], 10 s when absent; a 401 backs off the whole loop; 404 and 5xx keep today's behaviour.
- Identify ends within `identifyFor` plus one tick's fetch at any cadence.

**Size:** L (about 330 lines of code, 600 of test). **Seat:** `cwd-implementer-opus5`: numerics
(the combine, the backoff) and the dashboard's real API.

**Plan gate:** rule 1: `TestCombine_isTheRankMaximumOverEveryMultisetUpToSix` and
`TestRetryAfterDelay_sweep` (−100 … 100,000 s in steps of 7, as numbers and as dates). Pinned
values: the ten rows; 12 s, 120 s, absent → 10 s, and the two bounds. **Known clean coincidences:**
the default 10 s equals the device manager's enumerate cadence and the cap 30 s equals both
`rejectedBackoff` and `feedLostAfterSeconds`; the sweep's step of 7 never lands on 10 or 30 from
−100 (−100 + 7k: 10 is not reachable, 30 is not reachable; 12 and 120 are not either — the table
pins those), so no pinned value is a sweep artefact. Rule 2: the loop sends GETs to the dashboard
with no body; the only joined list it emits is the log line `lines %v`, bounded by the record
count × 99. Rule 3: `retryAfterDelay` is a parser; its real-shape fixtures are the RFC 7231
example date, `http.TimeFormat` dates, and the integer form; an absent header is the omitted-key
case. Rule 4: none. Rule 5: none.

---

## T5a — The device manager with the fake bus

**Carries out:** §12 items 1–4 (enumerate every 10 s; skip open paths; skip a non-preferred usage
pair; open; bind, replug or refuse; the unplug callback; a displaced or duplicate handle on the
channel is ignored; one beacon failing darkens no other; quit closes every handle), §5.3's
run-time guard ("same identity", retried every enumerate; hand-over within one keepalive period
plus one enumerate), §14.2 (the fake bus, the deadlock guard), red-team notes 1 (the manager's identity check before the close), 2 (the displaced-callback
scenario is **replaced** by its reachable producer: the same handle delivered twice, by the
keepalive and by a failed command in one window; closed once), 5 (the tray word is T7's; here the
fact is recorded) and 6 (the vanished-path sweep, an addition the design does not name, makes the
common replug hand over within one enumerate; the design's 15 s bound stays true and is stated in
its wording fix).

**Files:**
- Create: `cmd/blinkybeacon-tray/manager.go`
- Create: `cmd/blinkybeacon-tray/manager_test.go`
- Create: `cmd/blinkybeacon-tray/fakebus_test.go`

**Interfaces:**
- Consumes: T3's `deviceSource`, `deviceHandle`, `deviceInfo`, `AppState.heldBy/bind/rebind/dropHandle/markSameIdentity/onDrop`.
- Produces:

```go
// deviceScanInterval is how often the manager looks for lights: today's
// retry cadence. A var so the suite can shorten it.
var deviceScanInterval = 10 * time.Second

// usagePair is one HID top-level collection: the pair a light's path reports.
type usagePair struct{ Page, Usage uint16 }

// deviceManager owns the open handles. Nothing else opens or closes a light.
type deviceManager struct {
	app     *AppState
	src     deviceSource
	prefer  *usagePair               // nil: open every path (identity_mode.go sets it, T0b)
	open    map[string]deviceHandle  // path → handle; in memory only
	paths   map[deviceHandle]string  // the reverse, for a drop
	dropped chan deviceHandle        // handles the registry has let go of, to close here
}

func newDeviceManager(app *AppState, src deviceSource, prefer *usagePair) *deviceManager

// run enumerates at once, then every deviceScanInterval, closes dropped
// handles as they arrive, and on ctx.Done stops and closes every handle.
func (m *deviceManager) run(ctx context.Context)

// scan is one enumerate pass (exported to the suite through the manager; the
// loop above calls it).
func (m *deviceManager) scan()
```

  `scan`, as the manager runs it:

```go
func (m *deviceManager) scan() {
	infos, err := m.src.Enumerate()
	if err != nil {
		log.Printf("Looking for beacons: %v", err)
		return
	}
	// A path that is open but no longer enumerated is an unplug the callback
	// may not have delivered (the channel was full, or the keepalive has not
	// run yet): treat it as one.
	seen := map[string]bool{}
	for _, info := range infos {
		seen[info.Path] = true
	}
	for path, h := range m.open {
		if !seen[path] {
			m.app.dropHandle(h) // identity-checked; hands h back through onDrop → m.dropped
		}
	}
	refused := map[string]bool{}
	for _, info := range infos {
		if _, isOpen := m.open[info.Path]; isOpen {
			continue
		}
		if m.prefer != nil && (info.UsagePage != m.prefer.Page || info.Usage != m.prefer.Usage) {
			continue // a light's second collection is never opened
		}
		h, err := m.src.Open(info)
		if err != nil {
			log.Printf("Beacon at %s: cannot open: %v", info.Path, err)
			continue
		}
		key := h.Key()
		if held := m.app.heldBy(key); held != nil {
			if held.Live() {
				// A second light reporting the same identity as one already
				// held: not opened, said so in the tray, tried again next time.
				refused[key] = true
				h.Close()
				continue
			}
			old, id, err := m.app.rebind(h) // a replug: the old handle is stale
			if err != nil {
				h.Close()
				continue
			}
			m.forget(old)
			old.Close()
			log.Printf("Beacon %s is back", id)
		} else {
			id, err := m.app.bind(h)
			if err != nil {
				h.Close()
				continue
			}
			log.Printf("Beacon %s connected", id)
		}
		m.open[info.Path] = h
		m.paths[h] = info.Path
		h.OnUnplug(func() { m.app.dropHandle(h) })
		h.StartIdleWorker()
	}
	for _, key := range m.app.keys() { // every key the registry holds
		m.app.markSameIdentity(key, refused[key])
	}
}

func (m *deviceManager) closeDropped(h deviceHandle) {
	path, known := m.paths[h]
	if !known {
		return // displaced, or already closed: a stale delivery
	}
	delete(m.open, path)
	delete(m.paths, h)
	h.Close()
}
```

  (`AppState.keys() []string` is a small addition to `beacons.go` this task makes: every entry's
  key.) `run`: `m.app.onDrop = func(h deviceHandle) { select { case m.dropped <- h: default: log… } }`
  with `dropped` buffered 64; the select loop over the ticker, `m.dropped` and `ctx.Done()`; on
  done, for every open handle `Stop()` then `Close()`.

  The fake bus (`fakebus_test.go`):

```go
// fakeBus is a USB bus the suite can plug lights into and pull them out of.
// Its handles count commands and run a keepalive the test ticks by hand.
type fakeBus struct {
	mu      sync.Mutex
	mode    identityMode
	devs    map[string]*fakeDevice // by path
	order   []string               // paths in enumeration order
	nextSeq int
	enumErr error
}

type fakeDevice struct {
	info    deviceInfo
	portKey string
	plugged bool
	handles []*fakeBusHandle // every handle ever opened on this path
}

// fakeBusHandle is one open light. Every command and the keepalive take mu,
// as the real library's worker and commands take the FarmBeacon mutex; the
// keepalive fires the unplug callback while holding it, so a Close or Stop
// from inside the callback deadlocks — which the test catches with a timeout.
type fakeBusHandle struct {
	bus      *fakeBus
	dev      *fakeDevice
	key      string
	mu       sync.Mutex
	cmds     []StateValue
	closes   int
	failNext bool
	dead     bool
	lastOK   atomic.Bool
	onUnplug func()
	kick     chan struct{}
}

func newFakeBus(mode identityMode) *fakeBus
// plug adds a light and returns its path. In sn mode the key is "sn:"+serial;
// in port mode "port:"+portKey.
func (b *fakeBus) plug(serial, portKey string, page, usage uint16) string
// unplug removes the path from enumeration and makes every command on its
// open handle fail; the handle's keepalive, on its next tick, records the
// failure and fires the unplug callback under the handle's mutex.
func (b *fakeBus) unplug(path string)
// move is unplug then plug of the same light at a new port: a new path.
func (b *fakeBus) move(path, newPortKey string) string
// failNext makes the handle's next command fail without unplugging.
func (b *fakeBus) failNext(path string)
// tick runs every started keepalive once: a write, or on a dead handle the
// failure and the callback.
func (b *fakeBus) tick()
func (b *fakeBus) handle(path string) *fakeBusHandle // the latest handle on the path
func (b *fakeBus) Enumerate() ([]deviceInfo, error)
func (b *fakeBus) Open(info deviceInfo) (deviceHandle, error) // refuses an unplugged path

func (h *fakeBusHandle) Spin() error  { return h.cmd(StateSpin) }
func (h *fakeBusHandle) Flash() error { return h.cmd(StateFlash) }
func (h *fakeBusHandle) Stop() error  { return h.cmd(StateIdle) }
func (h *fakeBusHandle) Close() error { h.mu.Lock(); defer h.mu.Unlock(); h.closes++; return nil }
func (h *fakeBusHandle) Key() string  { return h.key }
func (h *fakeBusHandle) OnUnplug(f func()) { h.mu.Lock(); defer h.mu.Unlock(); h.onUnplug = f }
func (h *fakeBusHandle) StartIdleWorker()  { go h.keepalive() }
func (h *fakeBusHandle) Live() bool        { return h.lastOK.Load() }

func (h *fakeBusHandle) cmd(s StateValue) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dead || h.failNext {
		h.failNext = false
		return errors.New("write failed: device gone")
	}
	h.cmds = append(h.cmds, s)
	return nil
}

// keepalive is the fake's worker: each kick is one 5-second write.
func (h *fakeBusHandle) keepalive() {
	for range h.kick {
		h.mu.Lock()
		if h.dead {
			h.lastOK.Store(false)
			if h.onUnplug != nil {
				h.onUnplug() // under h.mu, as the library does
			}
			h.mu.Unlock()
			return
		}
		h.lastOK.Store(true)
		h.mu.Unlock()
	}
}
```

  `lastOK` starts true at open (as the library's does). The test manager runs `scan()` by hand
  (no goroutine) and drains `m.dropped` by hand through `drainDropped()`, so every scenario is
  deterministic; one test runs `run(ctx)` with `deviceScanInterval` shortened to prove the loop.

- [ ] **Step 1: Write the failing tests** (`manager_test.go`)

```go
// newManagerUnderTest wires a registry over the fake bus with no store.
func newManagerUnderTest(t *testing.T, mode identityMode) (*AppState, *fakeBus, *deviceManager) {
	app := NewAppState(nil)
	bus := newFakeBus(mode)
	m := newDeviceManager(app, bus, nil)
	app.onDrop = func(h deviceHandle) { m.dropped <- h }
	return app, bus, m
}

func drainDropped(m *deviceManager) {
	for {
		select {
		case h := <-m.dropped:
			m.closeDropped(h)
		default:
			return
		}
	}
}

func TestManager_threeLightsBecomeThreeBeacons(t *testing.T) {
	for _, mode := range []identityMode{identitySerial, identityPort} {
		app, bus, m := newManagerUnderTest(t, mode)
		bus.plug("S1", "p1", 1, 1)
		bus.plug("S2", "p2", 1, 1)
		bus.plug("S3", "p3", 1, 1)
		m.scan()
		views := app.Views()
		if len(views) != 3 {
			t.Fatalf("mode %v: %d beacons, want 3", mode, len(views))
		}
		wantPrefix := map[identityMode]string{identitySerial: "sn:", identityPort: "port:"}[mode]
		for i, v := range views {
			if v.ID != fmt.Sprintf("b%d", i+1) || !v.Connected || !strings.HasPrefix(v.Key, wantPrefix) || len(v.Lines) != 0 {
				t.Errorf("mode %v: beacon %d = %+v", mode, i, v)
			}
		}
		if bus.handle(bus.order[0]).kick == nil {
			t.Error("the keepalive was not started")
		}
	}
}

func TestManager_oneFailingBeaconDarkensNoOther(t *testing.T) {
	app, bus, m := newManagerUnderTest(t, identityPort)
	p1, p2, p3 := bus.plug("", "p1", 1, 1), bus.plug("", "p2", 1, 1), bus.plug("", "p3", 1, 1)
	m.scan()
	app.Command("b1", StateSpin)
	app.Command("b2", StateSpin)
	app.Command("b3", StateFlash)
	bus.failNext(p2)
	if err := app.Command("b2", StateFlash); err == nil {
		t.Fatal("the failing command must report its error")
	}
	drainDropped(m)
	if bus.handle(p2).closes != 1 {
		t.Errorf("the failed handle was closed %d times, want 1", bus.handle(p2).closes)
	}
	v1, _ := app.View("b1")
	v3, _ := app.View("b3")
	if v1.State != StateSpin || !v1.Connected || v3.State != StateFlash || !v3.Connected {
		t.Errorf("the others changed: %+v %+v", v1, v3)
	}
	if v2, _ := app.View("b2"); v2.Connected {
		t.Error("the failed beacon must read unplugged")
	}
	// The light is still on the bus: the next scan re-opens it as the same beacon.
	m.scan()
	if v2, _ := app.View("b2"); !v2.Connected || v2.ID != "b2" {
		t.Errorf("after the rescan: %+v", v2)
	}
	if n := len(bus.devs[p1].handles); n != 1 {
		t.Errorf("a healthy light was re-opened (%d handles)", n)
	}
}

func TestManager_unplugByKeepaliveThenReplugSamePath(t *testing.T) {
	// Windows: a replugged light keeps its path. The keepalive notices the
	// unplug; the path is gone from enumeration until it is back; then the
	// "not already open" filter has nothing to skip and the light is bound again.
	app, bus, m := newManagerUnderTest(t, identityPort)
	p := bus.plug("", "p1", 1, 1)
	m.scan()
	app.Command("b1", StateSpin)
	bus.unplug(p)
	bus.tick() // the keepalive fails and fires the callback under its mutex
	drainDropped(m)
	v, _ := app.View("b1")
	if v.Connected || bus.handle(p).closes != 1 || len(m.open) != 0 {
		t.Fatalf("after the unplug: %+v, closes %d, open %d", v, bus.handle(p).closes, len(m.open))
	}
	bus.devs[p].plugged = true // back in the same port
	m.scan()
	v, _ = app.View("b1")
	if !v.Connected || v.State != StateIdle || len(app.Views()) != 1 {
		t.Errorf("after the replug: %+v (%d beacons)", v, len(app.Views()))
	}
}

func TestManager_aVanishedPathIsAnUnplugEvenBeforeTheKeepaliveRuns(t *testing.T) {
	app, bus, m := newManagerUnderTest(t, identityPort)
	p := bus.plug("", "p1", 1, 1)
	m.scan()
	bus.unplug(p) // no tick: the callback never fires
	m.scan()
	drainDropped(m)
	if v, _ := app.View("b1"); v.Connected {
		t.Error("a path missing from enumeration must count as unplugged")
	}
}

func TestManager_sameHandleDeliveredTwiceIsClosedOnce(t *testing.T) {
	// The reachable producer of a stale delivery: the keepalive fails and a
	// command fails in the same window, and both hand the handle over.
	app, bus, m := newManagerUnderTest(t, identityPort)
	p := bus.plug("", "p1", 1, 1)
	m.scan()
	bus.unplug(p)
	app.Command("b1", StateSpin) // fails: dropHandle, first delivery
	bus.tick()                   // the keepalive's callback: dropHandle again — ignored by the registry
	if len(m.dropped) != 1 {
		t.Errorf("%d handles on the channel, want 1: the registry's identity check dropped the second", len(m.dropped))
	}
	// And a handle the manager has already closed is ignored by it too.
	h := bus.handle(p)
	drainDropped(m)
	m.closeDropped(h)
	if h.closes != 1 {
		t.Errorf("closed %d times, want 1", h.closes)
	}
}

func TestManager_aReplugAtANewPathIsTakenOverWithinOneScan(t *testing.T) {
	// macOS: a replug is a new path for the same key, and the old path is
	// gone from enumeration. The vanished-path sweep drops the old handle and
	// the new path binds in the same scan, before any keepalive has run: the
	// hand-over is one enumerate, inside the design's bound of a keepalive
	// period plus an enumerate.
	app, bus, m := newManagerUnderTest(t, identitySerial)
	p := bus.plug("S1", "p1", 1, 1)
	m.scan()
	app.Command("b1", StateSpin)
	old := bus.handle(p)
	p2 := bus.move(p, "p9") // unplugged and back at a new port; no keepalive has run
	m.scan()
	drainDropped(m)
	v, _ := app.View("b1")
	if !v.Connected || v.SameIdentity || old.closes != 1 || len(app.Views()) != 1 || m.open[p2] == nil || m.paths[old] != "" {
		t.Errorf("after the replug scan: %+v, old closes %d, %d beacons, open[p2] %v", v, old.closes, len(app.Views()), m.open[p2] != nil)
	}
	if v.State != StateIdle {
		t.Error("a rebound light starts idle; the loop's next tick re-applies the wanted state")
	}
}

func TestManager_aStaleHandleStillEnumeratedIsReplacedNotRefused(t *testing.T) {
	// The design's "not live" branch, kept as the safety net for a handle
	// whose keepalive failed while its path stayed listed (a hub that keeps
	// enumerating a dead device): held but not live, so the next path with
	// its key replaces it instead of being refused.
	app, bus, m := newManagerUnderTest(t, identitySerial)
	p := bus.plug("S1", "p1", 1, 1)
	m.scan()
	old := bus.handle(p)
	old.lastOK.Store(false)        // the keepalive failed; nothing has dropped the handle yet
	p2 := bus.plug("S1", "p2", 1, 1) // the same light's key at a new path; the old path still listed
	m.scan()
	drainDropped(m)
	v, _ := app.View("b1")
	if !v.Connected || v.SameIdentity || old.closes != 1 || m.open[p2] == nil || m.paths[old] != "" || len(app.Views()) != 1 {
		t.Errorf("after the scan: %+v, old closes %d, open[p2] %v, %d beacons", v, old.closes, m.open[p2] != nil, len(app.Views()))
	}
}

func TestManager_aTrueTwinIsRefusedEveryScanAndNamed(t *testing.T) {
	// sn mode: a second light with the same serial while the first is live.
	app, bus, m := newManagerUnderTest(t, identitySerial)
	bus.plug("S1", "p1", 1, 1)
	m.scan()
	twin := bus.plug("S1", "p2", 1, 1)
	for i := 0; i < 3; i++ {
		m.scan()
		bus.tick()
	}
	v, _ := app.View("b1")
	if !v.SameIdentity || len(app.Views()) != 1 {
		t.Errorf("the twin must not become a beacon: %+v, %d beacons", v, len(app.Views()))
	}
	if bus.handle(twin).closes != 3 {
		t.Errorf("the twin's path was opened and closed %d times over three scans, want 3 (tried again every time)", bus.handle(twin).closes)
	}
	// The tray's words for it are T7's (beaconRowTitle); the fact is the view's SameIdentity.
}

func TestManager_aSecondCollectionPathIsNeverOpened(t *testing.T) {
	app, bus, m := newManagerUnderTest(t, identityPort)
	m.prefer = &usagePair{Page: 0xff00, Usage: 0x01}
	bus.plug("", "p1", 0xff00, 0x01)
	other := bus.plug("", "p1", 0x0001, 0x0002) // the same light's second collection
	m.scan()
	if len(app.Views()) != 1 || len(bus.devs[other].handles) != 0 {
		t.Errorf("%d beacons, second path opened %d times; want 1 and 0", len(app.Views()), len(bus.devs[other].handles))
	}
	v, _ := app.View("b1")
	if v.SameIdentity {
		t.Error("a skipped collection is not a twin")
	}
}

func TestManager_aCloseFromInsideTheUnplugCallbackDeadlocks(t *testing.T) {
	// The guard the design asks for (§12 item 2): the fake holds its mutex
	// while firing the callback, as the library does; a Close from inside
	// must hang, which this test turns into a failure after two seconds.
	bus := newFakeBus(identityPort)
	p := bus.plug("", "p1", 1, 1)
	infos, _ := bus.Enumerate()
	h, _ := bus.Open(infos[0])
	done := make(chan struct{})
	h.OnUnplug(func() { h.Close(); close(done) })
	h.StartIdleWorker()
	bus.unplug(p)
	go bus.tick()
	select {
	case <-done:
		t.Fatal("a Close inside the callback returned: the fake does not model the library's mutex")
	case <-time.After(200 * time.Millisecond):
		// hung, as the real library would: the manager must never do this
	}
}

func TestManager_quitStopsAndClosesEveryHandle(t *testing.T) {
	app, bus, m := newManagerUnderTest(t, identityPort)
	deviceScanInterval = 5 * time.Millisecond
	t.Cleanup(func() { deviceScanInterval = 10 * time.Second })
	p1, p2 := bus.plug("", "p1", 1, 1), bus.plug("", "p2", 1, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); m.run(ctx) }()
	waitFor(t, "both lights found by the loop", func() bool { return len(app.Views()) == 2 })
	app.Command("b1", StateSpin)
	cancel()
	<-done
	for _, p := range []string{p1, p2} {
		h := bus.handle(p)
		if h.closes != 1 || h.cmds[len(h.cmds)-1] != StateIdle {
			t.Errorf("%s: closes %d, last command %v; want stopped then closed", p, h.closes, h.cmds)
		}
	}
}

func TestManager_enumerateErrorsAreLoggedAndRetried(t *testing.T) {
	app, bus, m := newManagerUnderTest(t, identityPort)
	bus.enumErr = errors.New("hid init failed")
	bus.plug("", "p1", 1, 1)
	m.scan()
	if len(app.Views()) != 0 {
		t.Fatal("nothing can be found when enumeration fails")
	}
	bus.enumErr = nil
	m.scan()
	if len(app.Views()) != 1 {
		t.Error("the next scan finds the light")
	}
}
```

- [ ] **Step 2: Run red.** Compile errors. **Disarms** once implemented: `closeDropped` closing
  without the `paths` check → `sameHandleDeliveredTwice…` red (`closes == 2`); the vanished-path
  sweep removed → `aVanishedPath…` red; the `held.Live()` test inverted → `aStaleHandleStillEnumerated…` red (refused instead of
  replaced) and `aTrueTwin…` red (replaced instead of refused); the `prefer` check removed → the
  collection test red; `markSameIdentity` never cleared → `aReplugAtANewPath…` red once a twin test has run in the
  same registry — so add one line to `aTrueTwin…`: unplug the twin, scan, and expect
  `SameIdentity` false.

- [ ] **Step 3: Implement** `manager.go` and the fake (`AppState.keys()` is T3's).

- [ ] **Step 4: Verify** suite (`-race`: the callback runs on the fake keepalive goroutine), vet,
  gofmt.

- [ ] **Step 5: Commit** `feat(tray): a device manager finds every plugged-in beacon, keeps each open, and notices when one goes`
  and `test(tray): a fake USB bus the suite plugs lights into and pulls out of`.

**Acceptance:**
- Every §14.2 scenario in this task's list passes in both modes where the mode matters.
- A handle is closed exactly once in every scenario (`closes` asserted everywhere).
- The deadlock guard test hangs for its 200 ms when a Close is attempted inside the callback.
- The real manager's `run` is exercised once (the quit test).

**Size:** M (about 160 lines of code, 420 of test). **Seat:** `cwd-implementer-opus5`: the plan
holds the shape but not the interleavings; the deadlock guard and the identity checks are the
implementer's reasoning.

**Plan gate:** rule 1: n/a (no numeric module; the only number is the cadence). Rule 2: nothing is
sent to another program; the USB writes are the fake's in this task. Rule 3: n/a. Rule 4: none.
Rule 5: none.

---

## T6 — HTTP: `?beacon=`, `GET /beacons`, the aggregate `/status`

**Carries out:** §9.1 (the optional `beacon` query parameter; every row of the reply table), §9.2
(`/status` ignores query parameters; the aggregate), §9.3 (`GET /beacons`), D2 and D3 as
recommended. This is the contract S17's Companion module and every script will use; the exact
reply is also in "What S16 hands to S17" at the end of this plan.

**Files:**
- Modify: `cmd/blinkybeacon-tray/server.go` (131 lines at `1e4784c`; T3 changed its constructor
  and added `driveAll`)
- Modify: `cmd/blinkybeacon-tray/server_test.go` (134 lines, 6 tests: every assertion kept)

**Interfaces:**
- Consumes: T3's `AppState.Resolve`, `Command`, `CommandAll`, `Aggregate`, `AggregateOf`, `Views`,
  `LineWatch`.
- Produces (the wire shape):

```go
type beaconJSON struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Connected bool        `json:"connected"`
	State     StateValue  `json:"state"`
	Lines     []int       `json:"lines"` // never null: [] for none
	Watch     WatchStatus `json:"watch"`
}

type beaconsResponse struct {
	V       int          `json:"v"` // 1
	Beacons []beaconJSON `json:"beacons"`
}

// beaconWatch folds a beacon's lines into one word: "off" for no lines, "ok"
// when every line it follows is ok, else the status of its first line (in
// its lines' order) that is not ok.
func beaconWatch(app *AppState, v beaconView) WatchStatus

// resolveBeacons reads every ?beacon= value: trimmed, resolved by public id
// then exact name, duplicates collapsed to one id. It answers before anything
// is driven: an empty value is 400 "beacon must name a beacon"; an unknown
// one is 404 "no such beacon". ok is false when a reply was written.
func (s *HTTPServer) resolveBeacons(w http.ResponseWriter, r *http.Request) (ids []string, ok bool)

// handleControl is the one body of /spin, /flash and /stop.
func (s *HTTPServer) handleControl(w http.ResponseWriter, r *http.Request, want StateValue)
func (s *HTTPServer) handleBeacons(w http.ResponseWriter, r *http.Request)
```

  `handleControl`:

```go
func (s *HTTPServer) handleControl(w http.ResponseWriter, r *http.Request, want StateValue) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// The query string is the documented place; Go also fills r.Form from a
	// form-encoded body. A body that does not parse is not a reason to refuse
	// the no-parameter call the Companion module makes.
	if err := r.ParseForm(); err != nil {
		r.Form = r.URL.Query()
	}
	if len(r.Form["beacon"]) == 0 {
		s.driveAll(w, want) // today's replies, exactly (T3)
		return
	}
	ids, ok := s.resolveBeacons(w, r)
	if !ok {
		return
	}
	tried, failed := 0, 0
	for _, id := range ids {
		err := s.state.Command(id, want)
		switch {
		case errors.Is(err, errBeaconUnplugged):
			continue
		case err != nil:
			tried++
			failed++
		default:
			tried++
		}
	}
	switch {
	case tried == 0:
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon not connected"})
	case failed == tried:
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon disconnected"})
	default:
		sv, connected := s.state.AggregateOf(ids)
		s.writeJSON(w, http.StatusOK, statusResponse{State: sv, Connected: connected})
	}
}
```

  `handleBeacons`: `GET` only (405 otherwise); `Cache-Control: no-store`; views in registry order;
  `Lines` copied into a non-nil slice. The mux gains `mux.HandleFunc("/beacons", s.handleBeacons)`.

- [ ] **Step 1: Write the failing tests** (`server_test.go`; the six existing tests stay as they are
  after T3's setup line)

```go
// twoBeaconServer is a registry with Desk left (b1, line 1) plugged in and
// Desk right (b2, line 2) configured but unplugged, behind the HTTP server.
func twoBeaconServer(t *testing.T) (*AppState, *HTTPServer, *mockBeacon) {
	t.Helper()
	app := NewAppState(nil)
	cfg := defaultConfig()
	cfg.Beacons = []BeaconRecord{
		{ID: "b1", Key: "port:p-left", Name: "Desk left", Lines: []int{1}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
		{ID: "b2", Key: "port:p-right", Name: "Desk right", Lines: []int{2}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
	}
	cfg.BeaconSeq = 2
	app.syncConfig(cfg)
	left := &mockBeacon{}
	plantKeyed(app, "port:p-left", left)
	return app, NewHTTPServer(app, "127.0.0.1", 1337), left
}

func post(t *testing.T, srv *HTTPServer, target string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, target, nil) // no body, no content type: the Companion module's call
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	return w.Code, body
}

func TestHandleSpin_everyRowOfTheReplyTable(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		plugRight bool
		failLeft bool
		wantCode int
		wantBody map[string]any
		wantSpin bool // Desk left's Spin called
	}{
		{"no parameter, one connected", "/spin", false, false, 200, map[string]any{"state": "spin", "connected": true}, true},
		{"by id, connected", "/spin?beacon=b1", false, false, 200, map[string]any{"state": "spin", "connected": true}, true},
		{"by exact name, percent-encoded", "/spin?beacon=Desk%20left", false, false, 200, map[string]any{"state": "spin", "connected": true}, true},
		{"by id, unplugged", "/spin?beacon=b2", false, false, 503, map[string]any{"error": "beacon not connected"}, false},
		{"unknown id", "/spin?beacon=b9", false, false, 404, map[string]any{"error": "no such beacon"}, false},
		{"unknown name", "/spin?beacon=Spare", false, false, 404, map[string]any{"error": "no such beacon"}, false},
		{"empty value", "/spin?beacon=", false, false, 400, map[string]any{"error": "beacon must name a beacon"}, false},
		{"whitespace value", "/spin?beacon=%20%20", false, false, 400, map[string]any{"error": "beacon must name a beacon"}, false},
		{"several, one unknown: nothing driven", "/spin?beacon=b1&beacon=b9", false, false, 404, map[string]any{"error": "no such beacon"}, false},
		{"several, one empty: nothing driven", "/spin?beacon=b1&beacon=", false, false, 400, map[string]any{"error": "beacon must name a beacon"}, false},
		{"several, one unplugged, one connected", "/spin?beacon=b1&beacon=b2", false, false, 200, map[string]any{"state": "spin", "connected": true}, true},
		{"several, all unplugged", "/spin?beacon=b2", false, false, 503, map[string]any{"error": "beacon not connected"}, false},
		{"the same value twice: driven once", "/spin?beacon=b1&beacon=Desk%20left", false, false, 200, map[string]any{"state": "spin", "connected": true}, true},
		{"by id, the command fails", "/spin?beacon=b1", false, true, 503, map[string]any{"error": "beacon disconnected"}, true},
		{"no parameter, every command fails", "/spin", false, true, 503, map[string]any{"error": "beacon disconnected"}, true},
		{"several, all connected, all fail", "/spin?beacon=b1&beacon=b2", true, true, 503, map[string]any{"error": "beacon disconnected"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, srv, left := twoBeaconServer(t)
			var right *mockBeacon
			if c.plugRight {
				right = &mockBeacon{err: errors.New("gone")}
				plantKeyed(app, "port:p-right", right)
			}
			if c.failLeft {
				left.err = errors.New("gone")
			}
			code, body := post(t, srv, c.target)
			if code != c.wantCode || !reflect.DeepEqual(body, c.wantBody) {
				t.Errorf("%s → %d %v, want %d %v", c.target, code, body, c.wantCode, c.wantBody)
			}
			if left.spinCalled != c.wantSpin {
				t.Errorf("Desk left Spin called = %v, want %v", left.spinCalled, c.wantSpin)
			}
			if c.failLeft {
				if v, _ := app.View("b1"); v.Connected {
					t.Error("a failed command must drop the beacon for reconnection")
				}
			}
		})
	}
}

func TestHandleSpin_theSameValueTwiceDrivesOnce(t *testing.T) {
	app, srv, _ := twoBeaconServer(t)
	counting := &countingBeacon{}
	app.rebind(&fakeHandle{Beacon: counting, key: "port:p-left", live: true})
	post(t, srv, "/spin?beacon=b1&beacon=b1&beacon=Desk%20left")
	if counting.spins != 1 {
		t.Errorf("Spin issued %d times, want 1", counting.spins)
	}
}

func TestHandleSpin_noParameterDrivesEveryConnectedBeaconAndSkipsFailures(t *testing.T) {
	app, srv, left := twoBeaconServer(t)
	right := &mockBeacon{err: errors.New("gone")}
	plantKeyed(app, "port:p-right", right)
	code, body := post(t, srv, "/flash")
	if code != 200 || body["state"] != "flash" || body["connected"] != true {
		t.Errorf("one of two failing: %d %v, want 200 flash connected (the aggregate of what remains)", code, body)
	}
	if !left.flashCalled || !right.flashCalled {
		t.Error("both must be driven")
	}
	if v, _ := app.View("b2"); v.Connected {
		t.Error("the failed one is dropped for reconnection")
	}
}

func TestHandleStatus_isTheAggregateAndIgnoresQueryParameters(t *testing.T) {
	app, srv, _ := twoBeaconServer(t)
	plantKeyed(app, "port:p-right", &mockBeacon{})
	app.Command("b2", StateSpin)
	app.Command("b1", StateFlash)
	for _, target := range []string{"/status", "/status?beacon=b1", "/status?beacon=nonsense"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		srv.ServeHTTP(w, req)
		var body map[string]any
		json.NewDecoder(w.Body).Decode(&body)
		if w.Code != 200 || body["state"] != "spin" || body["connected"] != true || len(body) != 2 {
			t.Errorf("%s → %d %v, want 200 {state:spin, connected:true} and nothing else", target, w.Code, body)
		}
	}
	app.dropHandle(viewHandle(app, "b1"))
	app.dropHandle(viewHandle(app, "b2"))
	_, body := get(t, srv, "/status")
	if body["state"] != "idle" || body["connected"] != false {
		t.Errorf("none plugged in → %v, want idle,false", body)
	}
}

func get(t *testing.T, srv *HTTPServer, target string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	return w.Code, body
}

func TestHandleBeacons_shape(t *testing.T) {
	app, srv, _ := twoBeaconServer(t)
	app.Command("b1", StateSpin)
	app.SetLineWatch(1, WatchOK, WatchDetail{})
	app.SetLineWatch(2, WatchNoLine, WatchDetail{})
	spare := plant(app, &mockBeacon{}) // an in-memory third light
	req := httptest.NewRequest(http.MethodGet, "/beacons", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("%d %v", w.Code, w.Header())
	}
	want := fmt.Sprintf(`{"v":1,"beacons":[`+
		`{"id":"b1","name":"Desk left","connected":true,"state":"spin","lines":[1],"watch":"ok"},`+
		`{"id":"b2","name":"Desk right","connected":false,"state":"idle","lines":[2],"watch":"no-line"},`+
		`{"id":"%s","name":"","connected":true,"state":"idle","lines":[],"watch":"off"}]}`, spare)
	if got := strings.TrimSpace(w.Body.String()); got != want {
		t.Errorf("GET /beacons =\n%s\nwant\n%s", got, want)
	}
}

func TestHandleBeacons_watchIsTheFirstLineNotOK(t *testing.T) {
	app := NewAppState(nil)
	cfg := defaultConfig()
	cfg.Beacons[0].Lines = []int{3, 1, 2}
	app.syncConfig(cfg)
	app.SetLineWatch(3, WatchOK, WatchDetail{})
	app.SetLineWatch(1, WatchThrottled, WatchDetail{})
	app.SetLineWatch(2, WatchRejected, WatchDetail{})
	v, _ := app.View("b1")
	if got := beaconWatch(app, v); got != WatchThrottled {
		t.Errorf("watch = %v, want throttled (the first not-ok line in the beacon's order)", got)
	}
	app.SetLineWatch(1, WatchOK, WatchDetail{})
	app.SetLineWatch(2, WatchOK, WatchDetail{})
	if got := beaconWatch(app, v); got != WatchOK {
		t.Errorf("watch = %v, want ok", got)
	}
}

func TestHandleBeacons_wrongMethod(t *testing.T) {
	_, srv, _ := twoBeaconServer(t)
	code, _ := post(t, srv, "/beacons")
	if code != http.StatusMethodNotAllowed {
		t.Errorf("POST /beacons → %d, want 405", code)
	}
}

func TestHandleSpin_beaconInAFormBodyIsHonoured(t *testing.T) {
	app, srv, left := twoBeaconServer(t)
	plantKeyed(app, "port:p-right", &mockBeacon{})
	req := httptest.NewRequest(http.MethodPost, "/spin", strings.NewReader("beacon=b1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 || !left.spinCalled {
		t.Errorf("a form body carrying beacon= drives that beacon: %d, spin=%v", w.Code, left.spinCalled)
	}
	if v, _ := app.View("b2"); v.State != StateIdle {
		t.Error("the other beacon was driven")
	}
}

func TestHandleSpin_aMalformedBodyStillDrivesEveryBeacon(t *testing.T) {
	_, srv, left := twoBeaconServer(t)
	req := httptest.NewRequest(http.MethodPost, "/spin", strings.NewReader("%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 || !left.spinCalled {
		t.Errorf("a body that does not parse must not break the no-parameter call: %d", w.Code)
	}
}
```

- [ ] **Step 2: Run red.** Compile errors on `beaconWatch`, `/beacons` → 404 in the shape test.
  **Disarms** once implemented: resolve-then-drive merged into one loop → "several, one unknown"
  red (`left.spinCalled` true); dedupe dropped → `theSameValueTwice…` red; `Lines` left nil →
  the shape test red (`null`); `ParseForm` error returned as 400 → `aMalformedBody…` red; the
  unplugged branch counted as tried → "several, one unplugged" turns 200 into… still 200 — the
  disarm that bites that row is counting unplugged as *failed*: 503.

- [ ] **Step 3: Implement** as above. `resolveBeacons` resolves every value first, then dedupes by
  resolved id, keeping first-seen order.

- [ ] **Step 4: Verify** suite, vet both, gofmt. The six existing tests: unchanged bodies, green.

- [ ] **Step 5: Commit** `feat(tray): /spin, /flash and /stop take an optional beacon by id or name; GET /beacons lists every beacon; /status answers for the beacons as a whole`

**Acceptance:**
- Every row of §9.1's table is a passing case; the no-parameter rows are today's replies to the
  byte.
- `GET /beacons` matches the golden string; `lines` is never `null`; `Cache-Control: no-store`.
- `/status` ignores query parameters and answers the aggregate.

**Size:** M (about 120 lines of code, 260 of test). **Seat:** `cwd-implementer-opus5`: the HTTP
contract the Companion module and scripts depend on.

**Plan gate:** rule 1: n/a (the aggregate's numerics are T3's). **Rule 2, payload checklist:**
`/status` and the control replies are two fixed keys; error bodies are fixed strings; `GET /beacons`
is `v` plus a list of N records, each with a name of at most 40 runes and at most 99 line numbers
(the form's bound; a hand-edited file could hold more, so the loader's dedupe bounds it to distinct
positive integers — the encoder writes whatever count there is, which is finite and the operator's
own), so the reply is under about 500 bytes per beacon; a script reading it has no size surprise
below a few hundred beacons. Rule 3: the one parser is `?beacon=`, exercised on the real shapes
the Companion module sends (no body, no content type) and curl sends (percent-encoded name), and on
a body with the key, a malformed body and an empty value (the omitted-key cases). Rule 4: none.
Rule 5: none.

---

## T7 — The tray: eight slots, the ordering rule, the aggregate icon, the S14 hook

**Carries out:** §11.1 (the menu: the top rows, the three every-beacon controls, eight root-level
beacon submenus pre-created hidden before the separator and Quit, the ordering rule, the icon as
the §9.2 aggregate, the tooltip, the S14 boolean), D4 as recommended, §12's tray word for a refused
light (red-team note 5: a fact, no cause), §5.4 (Identify in each submenu).

**Files:**
- Modify: `cmd/blinkybeacon-tray/tray.go` (175 lines; `windows || darwin`)
- Create: `cmd/blinkybeacon-tray/tray_labels.go`, `tray_labels_test.go` (no build tag: the words)
- Create: `cmd/blinkybeacon-tray/tray_slots.go`, `tray_slots_test.go` (no build tag: the slot logic
  over a menu-item interface, with a fake that models Windows)
- Not touched: `icons.go` (the four icons stay; S14 adds its mark on its own branch)

**Interfaces:**
- Consumes: T3's `Views`, `Aggregate`, `AnyConnectedFollowsALine`, `Lines`, `LineWatch`,
  `LineLabel`, `Command`, `CommandAll`, `Identify`; T4's `watchStatusLabel`.
- Produces:

```go
// tray_labels.go
const traySlots = 8

// lineName is how the tray names a line: the dashboard's label when a poll
// has told us it, else "line N".
func lineName(label string, line int) string

// beaconDisplayName is the name, or "New beacon (b3)" for an unnamed one.
func beaconDisplayName(v beaconView) string

// beaconRowTitle is a slot's root row: "Desk left — Spinning · Line A, Line C",
// "Desk right — Unplugged · Line B", "New beacon (b3) — Idle · Unbound". At
// most four line names are listed, then "+N more". A beacon another light
// claims the identity of gets " · ▲ another light reports this identity and
// was not opened" — what happened, with no guess at why.
func beaconRowTitle(v beaconView, label func(line int) string) string

// beaconStatusRow: "● Spinning", "● Flashing", "● Idle", "○ Unplugged".
func beaconStatusRow(v beaconView) string

// beaconFollowingRow: "Following: Line A (watching · picks), Line C (line not
// running)" — each line with its watch words after today's sentences minus
// the "Dashboard: " prefix — or "Unbound". At most four, then "+N more".
func beaconFollowingRow(v beaconView, label func(int) string, watch func(int) (WatchStatus, WatchDetail)) string

// trayTopRow: "● Beacons: 2 of 3 plugged in"; "○ Beacons: none plugged in";
// "○ Beacons: none found" with no records; with more than traySlots records
// the suffix " (8 shown here)".
func trayTopRow(views []beaconView) string

// trayDashboardRow: with no line followed, "○ Dashboard: not configured" (or
// the no-token sentence); else today's sentence for the first line (ascending)
// that is not watching, naming the line; else "● Dashboard: watching · N lines"
// ("1 line").
func trayDashboardRow(lines []int, watch func(int) (WatchStatus, WatchDetail), label func(int) string) string

// trayTooltip: "BlinkyBeacon — Desk left: spinning (Line A) · Desk right:
// unplugged · Spare: idle"; "BlinkyBeacon — No beacon detected" with none; at
// most eight beacons listed, then "+N".
func trayTooltip(views []beaconView, label func(int) string) string

// trayIcon picks the icon for the aggregate: grey when nothing is plugged
// in, else orange/red/blue for spin/flash/idle. bound is the S14 hook —
// true when any connected beacon follows a line — unused until S14 chooses
// its mark.
func trayIcon(state StateValue, connected, bound bool) []byte

// tray_slots.go
// menuItem is what a slot needs of a systray item.
type menuItem interface {
	SetTitle(string)
	Enable()
	Disable()
	Show()
	Hide()
}

// slotMenu is one pre-created beacon submenu: the root and its six children.
type slotMenu struct {
	root, status, following, spin, flash, stop, identify menuItem
}

// slotWords is what a slot shows for its beacon this tick.
type slotWords struct {
	title, status, following string
	spin, flash, stop, identify bool // enabled
}

type slotTable struct {
	menus [traySlots]slotMenu
	ids   [traySlots]string    // "" for a free slot
	shown [traySlots]bool
	last  [traySlots]slotWords // what was last written, to write only on change
	mu    sync.Mutex
}

func newSlotTable(menus [traySlots]slotMenu) *slotTable

// sync is the ordering rule (§11.1): a beacon keeps its slot while it is
// listed (plugged or not); a beacon no longer listed frees its slot with
// exactly one Hide; a newly listed beacon takes the lowest free slot, and
// that slot is written — root title first (on Windows the retitle is what
// shows it), then its children, then Show — in this tick and never earlier;
// a shown slot's items are rewritten only when their words change; a hidden
// slot is never touched. Returns how many views had no slot.
func (t *slotTable) sync(views []beaconView, words func(beaconView) slotWords) (unshown int)

// beaconAt is the id in slot i, for the click goroutines.
func (t *slotTable) beaconAt(i int) string
```

  `tray.go` after this task: `onTrayReady` creates, in order: the top row (`mBeacons`), the
  dashboard row, the HTTP row, Settings…, separator, Spin/Flash/Stop (every beacon), separator,
  **eight slots** (each `AddMenuItem("", "")` then `Hide()`, with six `AddSubMenuItem` children:
  status and following disabled; the four actions), separator, Start with Windows, Quit. The 500 ms
  loop: `views := state.Views()`; `unshown := slots.sync(views, words)`; icon via
  `trayIcon(state.Aggregate(), state.AnyConnectedFollowsALine())`; tooltip; top row (with the
  `(8 shown here)` suffix when `unshown > 0`); dashboard row; the every-beacon buttons enabled by
  the aggregate as today (`tray.go:107-136`'s rules). Clicks: one goroutine per slot selecting on
  its four children's `ClickedCh`, calling `state.Command(slots.beaconAt(i), …)` and
  `state.Identify(slots.beaconAt(i), time.Now())`; errors logged.

- [ ] **Step 1: Write the failing tests.** `tray_labels_test.go`:

```go
func TestTrayLabels_beaconRows(t *testing.T) {
	label := func(l int) string { return map[int]string{1: "Line A", 3: "Line C"}[l] }
	v := beaconView{ID: "b1", Name: "Desk left", Lines: []int{1, 3}, Connected: true, State: StateSpin}
	if got := beaconRowTitle(v, label); got != "Desk left — Spinning · Line A, Line C" {
		t.Errorf("%q", got)
	}
	v.Connected, v.State = false, StateIdle
	v.Lines = []int{2}
	if got := beaconRowTitle(v, label); got != "Desk left — Unplugged · line 2" {
		t.Errorf("%q", got)
	}
	n := beaconView{ID: "b3", Connected: true, State: StateIdle}
	if got := beaconRowTitle(n, label); got != "New beacon (b3) — Idle · Unbound" {
		t.Errorf("%q", got)
	}
	n.SameIdentity = true
	if got := beaconRowTitle(n, label); got != "New beacon (b3) — Idle · Unbound · ▲ another light reports this identity and was not opened" {
		t.Errorf("%q", got)
	}
	many := beaconView{ID: "b2", Name: "Hub", Lines: []int{1, 2, 3, 4, 5, 6}, Connected: true}
	if got := beaconRowTitle(many, label); got != "Hub — Idle · Line A, line 2, Line C, line 4 +2 more" {
		t.Errorf("%q", got)
	}
	if got := beaconStatusRow(v); got != "○ Unplugged" {
		t.Errorf("%q", got)
	}
}

func TestTrayLabels_followingRow(t *testing.T) {
	label := func(l int) string { return map[int]string{1: "Line A"}[l] }
	watch := func(l int) (WatchStatus, WatchDetail) {
		if l == 1 {
			return WatchOK, WatchDetail{}
		}
		return WatchStopped, WatchDetail{}
	}
	v := beaconView{Lines: []int{1, 2}}
	if got := beaconFollowingRow(v, label, watch); got != "Following: Line A (watching · draft timing: picks), line 2 (line not running)" {
		t.Errorf("%q", got)
	}
	if got := beaconFollowingRow(beaconView{}, label, watch); got != "Unbound" {
		t.Errorf("%q", got)
	}
}

func TestTrayLabels_topRowAndTooltip(t *testing.T) {
	label := func(l int) string { return "Line A" }
	if got := trayTopRow(nil); got != "○ Beacons: none found" {
		t.Errorf("%q", got)
	}
	views := []beaconView{
		{ID: "b1", Name: "Desk left", Lines: []int{1}, Connected: true, State: StateSpin},
		{ID: "b2", Name: "Desk right", Connected: false},
		{ID: "b3", Name: "Spare", Connected: true, State: StateIdle},
	}
	if got := trayTopRow(views); got != "● Beacons: 2 of 3 plugged in" {
		t.Errorf("%q", got)
	}
	if got := trayTooltip(views, label); got != "BlinkyBeacon — Desk left: spinning (Line A) · Desk right: unplugged · Spare: idle" {
		t.Errorf("%q", got)
	}
	if got := trayTopRow(views[1:2]); got != "○ Beacons: none plugged in" {
		t.Errorf("%q", got)
	}
	nine := make([]beaconView, 9)
	for i := range nine {
		nine[i] = beaconView{ID: fmt.Sprintf("b%d", i+1), Name: fmt.Sprintf("L%d", i+1), Connected: true}
	}
	if got := trayTopRow(nine); got != "● Beacons: 9 of 9 plugged in (8 shown here)" {
		t.Errorf("%q", got)
	}
	if got := trayTooltip(nine, label); !strings.HasSuffix(got, " · L8: idle +1") {
		t.Errorf("%q", got)
	}
	if got := trayTooltip(nil, label); got != "BlinkyBeacon — No beacon detected" {
		t.Errorf("%q", got)
	}
}

func TestTrayLabels_dashboardRow(t *testing.T) {
	label := func(l int) string { return map[int]string{1: "Line A", 2: "Line B"}[l] }
	ok := func(l int) (WatchStatus, WatchDetail) { return WatchOK, WatchDetail{} }
	if got := trayDashboardRow(nil, ok, label); got != "○ Dashboard: not configured" {
		t.Errorf("%q", got)
	}
	if got := trayDashboardRow([]int{1, 2}, ok, label); got != "● Dashboard: watching · 2 lines" {
		t.Errorf("%q", got)
	}
	if got := trayDashboardRow([]int{1}, ok, label); got != "● Dashboard: watching · 1 line" {
		t.Errorf("%q", got)
	}
	mixed := func(l int) (WatchStatus, WatchDetail) {
		if l == 2 {
			return WatchNoLine, WatchDetail{}
		}
		return WatchOK, WatchDetail{}
	}
	if got := trayDashboardRow([]int{1, 2}, mixed, label); got != "▲ Dashboard: line 2 not found" {
		t.Errorf("%q", got)
	}
	throttled := func(l int) (WatchStatus, WatchDetail) { return WatchThrottled, WatchDetail{RetryInSeconds: 12} }
	if got := trayDashboardRow([]int{2}, throttled, label); got != "▲ Dashboard: slowed by the dashboard (line 2, retry in 12s)" {
		t.Errorf("%q", got)
	}
}

func TestTrayIcon_isTheAggregate(t *testing.T) {
	cases := []struct {
		state     StateValue
		connected bool
		want      []byte
	}{
		{StateIdle, false, iconDisconnected}, {StateSpin, false, iconDisconnected},
		{StateIdle, true, iconIdle}, {StateFlash, true, iconFlash}, {StateSpin, true, iconSpin},
	}
	for _, c := range cases {
		if got := trayIcon(c.state, c.connected, false); !bytes.Equal(got, c.want) {
			t.Errorf("trayIcon(%v,%v) picked the wrong icon", c.state, c.connected)
		}
		if got := trayIcon(c.state, c.connected, true); !bytes.Equal(got, c.want) {
			t.Errorf("bound=true must not change the icon until S14 chooses its mark")
		}
	}
}
```

  `tray_slots_test.go`:

```go
// fakeMenu models the Windows tray library (systray_windows.go:555-571,
// 680-690): items have ids in creation order; a visible item's position is
// its id-sorted rank among visible siblings; ANY update to a hidden item
// inserts it (SetTitle, Enable, Disable); Hide removes it; Show inserts it.
type fakeMenu struct {
	mu      sync.Mutex
	nextID  int
	visible map[int]bool
	ops     []menuOp // every call, in order, with the tick it happened in
	tick    int
}

type menuOp struct {
	tick   int
	id     int
	parent int // 0 for root
	kind   string // "title", "enable", "disable", "show", "hide"
	hidden bool   // the item was hidden when the op arrived
}

type fakeItem struct {
	m      *fakeMenu
	id     int
	parent int
	title  string
}

func (m *fakeMenu) newItem(parent int) *fakeItem // creates visible, as AddMenuItem does
func (m *fakeMenu) newSlotMenu() slotMenu        // a root item (hidden after creation) and six children
func (m *fakeMenu) rootOrder() []int             // visible root ids in position order
func (m *fakeMenu) next()                        // one tick

func (i *fakeItem) SetTitle(s string) { i.m.op(i, "title") ; i.title = s }
func (i *fakeItem) Enable()            { i.m.op(i, "enable") }
func (i *fakeItem) Disable()           { i.m.op(i, "disable") }
func (i *fakeItem) Show()              { i.m.op(i, "show") }
func (i *fakeItem) Hide()              { i.m.op(i, "hide") }

// op records the call; an update or show on a hidden item makes it visible;
// a hide makes it hidden.
func (m *fakeMenu) op(i *fakeItem, kind string)

// fixture: the tray as onTrayReady builds it — the top rows, the controls,
// a separator, eight hidden slots, a separator, two bottom rows.
func newTrayFixture() (*fakeMenu, *slotTable, []int /* every root id in creation order */)

func words(v beaconView) slotWords {
	return slotWords{title: beaconDisplayName(v), status: string(v.State), following: fmt.Sprint(v.Lines),
		spin: v.Connected && v.State != StateSpin, flash: v.Connected && v.State != StateFlash, stop: v.Connected && v.State != StateIdle, identify: v.Connected}
}

func view(id string, connected bool) beaconView { return beaconView{ID: id, Name: id, Connected: connected} }

// checkInvariants fails on: an op on a hidden slot item in a tick where that
// slot's root was not shown; a root title op that was not the first op on
// that slot in the tick it was shown; a visible root order that is not
// creation order; more than one op on a retiring slot.
func checkInvariants(t *testing.T, m *fakeMenu, created []int, tbl *slotTable)

func TestSlots_plugUnplugForgetReplug(t *testing.T) {
	m, tbl, created := newTrayFixture()
	seq := [][]beaconView{
		{view("b1", true)},
		{view("b1", true), view("b2", true), view("b3", true)},
		{view("b1", true), view("b2", false), view("b3", true)}, // b2 unplugged: keeps its slot
		{view("b1", true), view("b3", true)},                    // b2 forgotten: slot freed
		{view("b1", true), view("b3", true), view("b4", true)},  // b4 takes the freed slot
		{view("b3", true), view("b4", true)},                    // b1 vanished
		{view("b3", true), view("b4", true), view("b1", true)},  // b1 is back: lowest free slot
	}
	for _, views := range seq {
		m.next()
		if unshown := tbl.sync(views, words); unshown != 0 {
			t.Fatalf("tick %d: %d unshown", m.tick, unshown)
		}
		checkInvariants(t, m, created, tbl)
		shown := 0
		for i := range tbl.ids {
			if tbl.shown[i] {
				shown++
			}
		}
		if shown != len(views) {
			t.Fatalf("tick %d: %d slots shown for %d beacons", m.tick, shown, len(views))
		}
	}
	if tbl.ids[1] != "b1" {
		t.Errorf("b1 came back into slot %v, want the lowest free slot (1)", tbl.ids)
	}
}

func TestSlots_aNinthBeaconIsCountedNotShown(t *testing.T) {
	m, tbl, created := newTrayFixture()
	var views []beaconView
	for i := 1; i <= 9; i++ {
		views = append(views, view(fmt.Sprintf("b%d", i), true))
	}
	m.next()
	if unshown := tbl.sync(views, words); unshown != 1 {
		t.Errorf("unshown = %d, want 1", unshown)
	}
	checkInvariants(t, m, created, tbl)
	for _, id := range tbl.ids {
		if id == "b9" {
			t.Error("the ninth beacon took a slot")
		}
	}
	// It is shown as soon as one frees.
	m.next()
	if unshown := tbl.sync(views[1:], words); unshown != 0 || tbl.ids[0] != "b9" {
		t.Errorf("after b1 left: unshown %d, slot 0 = %q", unshown, tbl.ids[0])
	}
	checkInvariants(t, m, created, tbl)
}

func TestSlots_aShownSlotIsRewrittenOnlyWhenItsWordsChange(t *testing.T) {
	m, tbl, _ := newTrayFixture()
	m.next()
	tbl.sync([]beaconView{view("b1", true)}, words)
	before := len(m.ops)
	m.next()
	tbl.sync([]beaconView{view("b1", true)}, words)
	if len(m.ops) != before {
		t.Errorf("%d ops on an unchanged beacon, want 0", len(m.ops)-before)
	}
	m.next()
	v := view("b1", true)
	v.State = StateSpin
	tbl.sync([]beaconView{v}, words)
	titles := 0
	for _, op := range m.ops[before:] {
		if op.kind == "title" {
			titles++
		}
	}
	if titles == 0 {
		t.Error("a state change must rewrite the words")
	}
}

func TestSlots_theRuleIsBrokenByANaiveImplementation(t *testing.T) {
	// The fake's invariants must bite: writing a hidden slot's status row
	// before its root is an update on a hidden item outside the showing
	// sequence, and the fake reports it.
	m, _, created := newTrayFixture()
	m.next()
	slot := m.newSlotMenu()
	slot.status.SetTitle("● Idle") // the child first: wrong
	slot.root.SetTitle("b1")
	slot.root.Show()
	var found bool
	for _, op := range m.ops {
		if op.kind == "title" && op.hidden && op.parent != 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("the fake did not record the child update on a hidden parent")
	}
	_ = created
}
```

- [ ] **Step 2: Run red.** Compile errors. **Disarms** once implemented: `sync` writing the
  children before the root title → `checkInvariants` red; `sync` touching a free (hidden) slot to
  blank it → red; assigning new beacons to the highest free slot → `plugUnplugForgetReplug` red
  on `tbl.ids[1]`; `beaconRowTitle` without the four-line bound → the "Hub" case red;
  `trayTopRow` without the suffix → `aNinthBeacon…`'s top-row sibling red.

- [ ] **Step 3: Implement** `tray_labels.go`, `tray_slots.go`, then `tray.go`. `tray.go` keeps the
  every-beacon buttons' enable rules and the HTTP row; `mStatus`/`mBound`/`mWatch` become the
  top row and the dashboard row (the per-line words now live in each slot's following row).

- [ ] **Step 4: Verify** suite, vet Linux **and `GOOS=windows`** (this is the one wave-2 task whose
  main file only compiles under the tag), gofmt. Build the Windows exe once
  (`GOOS=windows … go build …`) to prove the tray package links.

- [ ] **Step 5: Commit** `feat(tray): one submenu per beacon with its state, lines and controls; the icon shows the busiest light`.

**Acceptance:**
- The slot invariants hold over every sequence in the tests; the fake's own self-test proves they
  bite.
- Every label function is table-tested; every joined list is bounded (four lines, eight beacons).
- The Windows build links; the hidden-parent re-show on real Windows is T10's hand step.

**Size:** M (about 320 lines of code, 330 of test). **Seat:** `cwd-implementer-opus5`: a
third-party API (`systray`) whose Windows behaviour the fake has to model exactly.

**Plan gate:** rule 1: n/a. **Rule 2, payload checklist:** menu titles and the tooltip go to the
OS; every joined list is bounded — four line names (each the dashboard's label, which `lineName`
cuts at 24 runes with "…"), eight beacons in the tooltip, names at most 40 runes — so no title
exceeds about 200 characters; a Windows menu item holds 256. Rule 3: n/a. Rule 4: none. Rule 5:
none. **S14 note for the master:** `wt/s14-bound-icon` also edits `tray.go` and `icons.go`;
whichever lands second merges by hand; the hook S14 needs is `trayIcon`'s `bound` argument.

---

## T8 — The settings page: per-beacon rows, the merge, Identify, Forget, Take over, the API block

**Carries out:** §11.2 in full (the rows, the fields keyed by public id, the lines checkboxes
from the cached list with the comma-separated fallback and its fourth reason, validation naming
the beacon, names escaped, the merge against the registry, Identify and Forget by public id with
409 for a gone id, Take over for unplugged records only, hidden when none: red-team note 7),
§10.5 (tombstones → 409 refusing the whole save; vanished → skipped with the note; an in-memory
row written only when it carries a name, a line or a Take over; an unmentioned record kept),
§5.4 (names: 1–40 runes, unique, never `b`+digits), §9's documentation on the page, D1 as
recommended. Red-team note 4 is pinned here: a tab from an earlier process naming an in-memory id
is refused by the form token and nothing is promoted.

**Files:**
- Modify: `cmd/blinkybeacon-tray/settings_handler.go` (547 lines: the form template's beacon
  section, `handleGet`, `handlePost`, `pickerUnavailableReason`, the API block; `lineField`,
  `linePickerHTML`, `lineNumberFieldHTML` are replaced by the per-beacon renderer)
- Create: `cmd/blinkybeacon-tray/settings_beacons.go` (the row renderer and the form parser)
- Create: `cmd/blinkybeacon-tray/settings_beacons_test.go`
- Modify: `cmd/blinkybeacon-tray/settings_handler_test.go` (834 lines + the 20 tests T3m moved:
  the old field names become `b1.*`; the one-line picker assertions become checkbox assertions)
- Modify: `cmd/blinkybeacon-tray/lines_cache_test.go` (`pickerMarkup` is the checkbox list's
  markup; nothing else)

**Interfaces:**
- Consumes: T3's `Views`, `IsTombstone`, `Identify`, `Forget`, `TakeOver`, `Promote`,
  `updateConfig`, `store.Get`; T0's `readsAsPublicID`; the line-list cache as today.
- Produces:

```go
// settings_beacons.go

// Field names: "<id>.name", "<id>.lines" (several), "<id>.lines_text" (the
// fallback), "<id>.flash_seconds", "<id>.lobby_flash", "<id>.lobby_flash_seconds",
// "<id>.pause_side", "<id>.takeover". The action buttons: name "action",
// values "identify:<id>" and "forget:<id>".
const (
	maxNameRunes = 40
	maxLine      = 99
)

// beaconRow is what one row of the form said about one beacon.
type beaconRow struct {
	ID       string
	Name     string
	Lines    []int
	Light    BeaconRecord // FlashSeconds, LobbyFlash, LobbyFlashSeconds, PauseSide
	TakeOver string       // the public id to absorb, or ""
}

// parseBeaconRows reads every "<id>.*" field. A row exists for an id when
// any of its fields is present. Errors name the beacon and the field:
// `Desk right: flash_seconds must be a whole number of seconds between 1 and 600, got "900"`.
func parseBeaconRows(form url.Values, names func(id string) string) (map[string]beaconRow, error)

// validateName: trimmed, 1–40 runes, not a public id in any case.
func validateName(name string) (string, error)

// parseLines reads the checkboxes, or the comma-separated text when the
// checkboxes were not rendered: each 1–99, duplicates collapsed, ascending.
func parseLines(ticked []string, text string) ([]int, error)

// beaconRowsHTML renders every row (configured in config order, then
// in-memory), with the lines control from the cached list or the text
// fallback with its reason, the Identify or Forget button, and the Take over
// select — present only on an in-memory row and only when at least one
// configured record is unplugged. Every operator string is escaped.
func beaconRowsHTML(views []beaconView, lines []lineSummary, listReason string, labels func(int) string) string

// pickerUnavailableReason gains a fourth word: a 429 on the list request is
// "slowed by the dashboard".

// settings_handler.go
// handlePost: csrf; action buttons first (identify:/forget: → act, 303 to
// /settings, no save; a gone or unplugged id → 409 "…reopen Settings");
// else the global fields as today, the rows, the checks (tombstone → 409
// naming the id, nothing written; duplicate names; names that read as ids),
// then ONE updateConfig for the configured rows and the global fields,
// Promote for each in-memory row that carries a name, a line or a Take over
// (Take over first: TakeOver(target, id); the row's typed values are not
// applied to the survivor), and the saved page redirecting to
// /settings?note=skipped:<id>,<id> when rows were skipped.
```

  The 409 text for a tombstone: `Beacon b2 was forgotten after this page was opened — reopen Settings`.
  The note shown on reload: `Beacon b3 is no longer listed; its row was skipped.` (only when
  `?note=skipped:` carries public ids of the exact shape; anything else is ignored). The API block
  (`apiDocsHTMLTmpl`) gains the rows for `?beacon=` and `GET /beacons`:

```html
<tr><td><code>POST</code></td><td><code>/spin?beacon=b1</code></td><td>Start the rotating light on one beacon, by its id (<code>b1</code>) or its exact name (<code>Desk%20left</code>). Repeat the parameter for several. Without it, every beacon.</td></tr>
<tr><td><code>GET</code></td><td><code>/beacons</code></td><td>Every beacon: id, name, whether it is plugged in, its state, the lines it follows, and its watch status.</td></tr>
```

  with the sentence: *A call naming a beacon that is not listed answers `404` with
  `{"error": "no such beacon"}`; one naming an unplugged beacon answers `503` with
  `{"error": "beacon not connected"}`.* And `curl -X POST %[1]s/spin?beacon=b1` and
  `curl %[1]s/beacons` among the examples.

- [ ] **Step 1: Write the failing tests** (`settings_beacons_test.go`)

```go
func TestParseLines_checkboxesAndFallback(t *testing.T) {
	got, err := parseLines([]string{"3", "1", "3"}, "")
	if err != nil || !reflect.DeepEqual(got, []int{1, 3}) {
		t.Errorf("%v %v", got, err)
	}
	got, err = parseLines(nil, " 2, 5 ,2 ")
	if err != nil || !reflect.DeepEqual(got, []int{2, 5}) {
		t.Errorf("%v %v", got, err)
	}
	if got, err := parseLines(nil, ""); err != nil || len(got) != 0 {
		t.Errorf("nothing ticked: %v %v", got, err)
	}
	for _, bad := range []string{"0", "100", "x", "1;2", "-1"} {
		if _, err := parseLines(nil, bad); err == nil {
			t.Errorf("parseLines(%q) accepted", bad)
		}
		if _, err := parseLines([]string{bad}, ""); err == nil {
			t.Errorf("a ticked %q accepted", bad)
		}
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"Desk left", " Desk left ", "b1x", strings.Repeat("é", 40)} {
		if _, err := validateName(ok); err != nil {
			t.Errorf("validateName(%q): %v", ok, err)
		}
	}
	for _, bad := range []string{"", "   ", "b1", "B2", " b01 ", strings.Repeat("é", 41), strings.Repeat("a", 41)} {
		if _, err := validateName(bad); err == nil {
			t.Errorf("validateName(%q) accepted", bad)
		}
	}
	if got, _ := validateName(" Desk left "); got != "Desk left" {
		t.Errorf("not trimmed: %q", got)
	}
}

func TestParseBeaconRows_namesTheBeaconInARefusal(t *testing.T) {
	form := url.Values{"b2.name": {"Desk right"}, "b2.flash_seconds": {"900"}}
	_, err := parseBeaconRows(form, func(id string) string { return "Desk right" })
	if err == nil || !strings.HasPrefix(err.Error(), "Desk right: flash_seconds must be") {
		t.Errorf("err = %v", err)
	}
	form = url.Values{"b1.name": {"A"}, "b3.lines": {"2"}}
	rows, err := parseBeaconRows(form, func(string) string { return "" })
	if err != nil || len(rows) != 2 || rows["b3"].Lines[0] != 2 || rows["b1"].Name != "A" {
		t.Errorf("%+v %v", rows, err)
	}
}

func TestBeaconRowsHTML_escapesAndKeysByPublicId(t *testing.T) {
	inject := `" onfocus="alert(1)`
	views := []beaconView{{ID: "b1", Name: inject, Key: "port:6&2a1b3c4d&0&0001", Lines: []int{3}, Connected: true, Light: BeaconRecord{FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth}}}
	three := 3
	html := beaconRowsHTML(views, []lineSummary{{N: &three, Label: "Line C", Running: true}}, "", nil)
	if strings.Contains(html, inject) {
		t.Error("the name was not escaped")
	}
	for _, want := range []string{`name="b1.name"`, `name="b1.lines" value="3" checked`, `name="b1.flash_seconds"`, `name="b1.pause_side"`, `name="action" value="identify:b1"`, `b1 · key port:6&amp;2a1b3c4d&amp;0&amp;0001`} {
		if !strings.Contains(html, want) {
			t.Errorf("row lacks %s:\n%s", want, html)
		}
	}
	if strings.Contains(html, `name="b1.takeover"`) || strings.Contains(html, `forget:b1`) {
		t.Error("a configured, plugged-in row offers neither Take over nor Forget")
	}
}

func TestBeaconRowsHTML_takeOverOnlyWhenARecordIsUnplugged(t *testing.T) {
	newLight := beaconView{ID: "b3", InMemory: true, Connected: true, Light: BeaconRecord{FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth}}
	plugged := beaconView{ID: "b1", Name: "Desk left", Connected: true, Light: newLight.Light}
	unplugged := beaconView{ID: "b2", Name: "Desk right", Connected: false, Light: newLight.Light}
	html := beaconRowsHTML([]beaconView{plugged, newLight}, nil, "unreachable", nil)
	if strings.Contains(html, `name="b3.takeover"`) {
		t.Error("no record is unplugged: the Take over control must not be rendered")
	}
	html = beaconRowsHTML([]beaconView{plugged, unplugged, newLight}, nil, "unreachable", nil)
	if !strings.Contains(html, `name="b3.takeover"`) || !strings.Contains(html, `<option value="b2">Desk right, moved from its old port</option>`) || strings.Contains(html, `<option value="b1">`) {
		t.Errorf("Take over must list exactly the unplugged records:\n%s", html)
	}
	if !strings.Contains(html, `forget:b2`) || strings.Contains(html, `identify:b2`) {
		t.Error("an unplugged row offers Forget, not Identify")
	}
	if !strings.Contains(html, `name="b1.lines_text"`) || !strings.Contains(html, "Line list unavailable (unreachable)") {
		t.Error("with no list the lines control is the text fallback with its reason")
	}
}

func TestBeaconRowsHTML_aFollowedLineNotInTheListIsKeptTicked(t *testing.T) {
	three := 3
	views := []beaconView{{ID: "b1", Name: "A", Lines: []int{7}, Connected: true}}
	html := beaconRowsHTML(views, []lineSummary{{N: &three, Label: "Line C"}}, "", nil)
	if !strings.Contains(html, `name="b1.lines" value="7" checked> Line 7 (not in the dashboard's list)`) {
		t.Errorf("%s", html)
	}
}

func TestPickerUnavailableReason_429(t *testing.T) {
	if got := pickerUnavailableReason(&pollError{Status: http.StatusTooManyRequests}); got != "slowed by the dashboard" {
		t.Errorf("%q", got)
	}
}
```

  `settings_handler_test.go` (new tests; the helper `twoBeaconForm()` replaces `beaconLightForm()`):

```go
// twoBeaconForm is the form a browser posts for Desk left (b1) and Desk
// right (b2) as the page renders them, every field present.
func twoBeaconForm() url.Values {
	return url.Values{
		"addr": {defaultAddr}, "port": {"1337"}, "dashboard_url": {""},
		"b1.name": {"Desk left"}, "b1.lines": {"1"}, "b1.flash_seconds": {"15"}, "b1.lobby_flash_seconds": {"10"}, "b1.pause_side": {pauseSideBoth},
		"b2.name": {"Desk right"}, "b2.lines": {"2"}, "b2.flash_seconds": {"15"}, "b2.lobby_flash_seconds": {"10"}, "b2.pause_side": {pauseSideBoth},
	}
}

// twoBeaconApp is a registry over a temp config holding Desk left (plugged
// in) and Desk right (unplugged), as twoBeaconServer builds it for the HTTP tests.
func twoBeaconApp(t *testing.T) *AppState {
	t.Helper()
	app := newTestApp(t)
	app.updateConfig(func(c *Config) error {
		c.Beacons = []BeaconRecord{
			{ID: "b1", Key: "port:p-left", Name: "Desk left", Lines: []int{1}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
			{ID: "b2", Key: "port:p-right", Name: "Desk right", Lines: []int{2}, FlashSeconds: 15, LobbyFlashSeconds: 10, PauseSide: pauseSideBoth},
		}
		c.BeaconSeq = 2
		return nil
	})
	plantKeyed(app, "port:p-left", &countingBeacon{})
	return app
}

func TestSettingsPost_roundTripsTwoBeacons(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	form := twoBeaconForm()
	form["b1.lines"] = []string{"1", "3"}
	form.Set("b1.flash_seconds", "45")
	form.Set("b2.name", "Shelf")
	form.Set("b2.pause_side", pauseSideDire)
	form.Set("b2.lobby_flash", "1")
	if w := submitSettings(t, h, form); w.Code != http.StatusOK {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	cfg := app.store.Get()
	if !reflect.DeepEqual(cfg.Beacons[0].Lines, []int{1, 3}) || cfg.Beacons[0].FlashSeconds != 45 || cfg.Beacons[1].Name != "Shelf" || cfg.Beacons[1].PauseSide != pauseSideDire || !cfg.Beacons[1].LobbyFlash {
		t.Errorf("saved %+v", cfg.Beacons)
	}
	if cfg.Beacons[0].Key != "port:p-left" || cfg.Beacons[1].Key != "port:p-right" || cfg.Beacons[0].ID != "b1" {
		t.Error("keys and ids never change through the form")
	}
}

func TestSettingsPost_refusesADuplicateNameAndANameThatReadsAsAnId(t *testing.T) {
	for _, bad := range map[string]string{"Desk left": "duplicate", "b7": "reads as an id", "B2": "reads as an id", "": "empty"} {
		app := twoBeaconApp(t)
		form := twoBeaconForm()
		form.Set("b2.name", bad)
		w := submitSettings(t, &settingsHandler{app: app}, form)
		if w.Code != http.StatusBadRequest {
			t.Errorf("name %q: %d, want 400", bad, w.Code)
		}
		if app.store.Get().Beacons[1].Name != "Desk right" {
			t.Errorf("name %q: a refused save wrote", bad)
		}
	}
}

func TestSettingsPost_countsNameLengthInRunes(t *testing.T) {
	app := twoBeaconApp(t)
	form := twoBeaconForm()
	form.Set("b2.name", strings.Repeat("é", 40))
	if w := submitSettings(t, &settingsHandler{app: app}, form); w.Code != http.StatusOK {
		t.Errorf("forty accented characters refused: %d", w.Code)
	}
	form.Set("b2.name", strings.Repeat("é", 41))
	if w := submitSettings(t, &settingsHandler{app: app}, form); w.Code != http.StatusBadRequest {
		t.Errorf("forty-one accepted: %d", w.Code)
	}
}

func TestSettingsPost_namingANewLightSavesItAndANamelessOneIsNotWritten(t *testing.T) {
	app := twoBeaconApp(t)
	id := plant(app, &countingBeacon{}) // b3, in memory
	h := &settingsHandler{app: app}
	form := twoBeaconForm()
	form.Set(id+".flash_seconds", "15") // the row is present but carries no name, line or take over
	submitSettings(t, h, form)
	if len(app.store.Get().Beacons) != 2 {
		t.Fatal("a nameless in-memory row was written")
	}
	form.Set(id+".name", "Spare")
	submitSettings(t, h, form)
	cfg := app.store.Get()
	if len(cfg.Beacons) != 3 || cfg.Beacons[2].ID != id || cfg.Beacons[2].Name != "Spare" || cfg.Beacons[2].Key == "" || cfg.BeaconSeq != 3 {
		t.Errorf("after naming: %+v seq %d", cfg.Beacons, cfg.BeaconSeq)
	}
	v, _ := app.View(id)
	if v.InMemory {
		t.Error("the promoted beacon still reads in-memory")
	}
}

func TestSettingsPost_aLineAloneAlsoPromotes(t *testing.T) {
	app := twoBeaconApp(t)
	id := plant(app, &countingBeacon{})
	form := twoBeaconForm()
	form[id+".lines"] = []string{"2"}
	submitSettings(t, &settingsHandler{app: app}, form)
	if cfg := app.store.Get(); len(cfg.Beacons) != 3 || cfg.Beacons[2].Name != "" || cfg.Beacons[2].Lines[0] != 2 {
		t.Errorf("%+v", cfg.Beacons)
	}
}

func TestSettingsPost_aRecordBoundAfterThePageOpenedSurvivesTheSave(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	get := httptest.NewRecorder()
	h.handleGet(get, httptest.NewRequest(http.MethodGet, "/settings", nil)) // the page is open
	plantKeyed(app, "port:p-right", &countingBeacon{})                       // Desk right is plugged in after
	form := twoBeaconForm()
	form.Del("b2.name") // the stale page knows only what it rendered; here: no b2 row at all
	for k := range form {
		if strings.HasPrefix(k, "b2.") {
			form.Del(k)
		}
	}
	w := submitSettings(t, h, form)
	if w.Code != http.StatusOK {
		t.Fatalf("%d", w.Code)
	}
	if cfg := app.store.Get(); len(cfg.Beacons) != 2 || cfg.Beacons[1].Name != "Desk right" || cfg.Beacons[1].Key != "port:p-right" {
		t.Errorf("an unmentioned record must be kept as it is: %+v", cfg.Beacons)
	}
}

func TestSettingsPost_aForgottenIdRefusesTheWholeSave(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	app.Forget("b2")
	form := twoBeaconForm()
	form.Set("b1.name", "Renamed")
	w := submitSettings(t, h, form)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "Beacon b2 was forgotten after this page was opened — reopen Settings") {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	if app.store.Get().Beacons[0].Name != "Desk left" {
		t.Error("something was written despite the 409")
	}
}

func TestSettingsPost_aVanishedRowIsSkippedAndTheRestApplies(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	id := plant(app, &countingBeacon{})
	app.dropHandle(viewHandle(app, id)) // the borrowed light left before the save
	form := twoBeaconForm()
	form.Set(id+".name", "Borrowed")
	form.Set("b1.name", "Renamed")
	w := submitSettings(t, h, form)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "/settings?note=skipped:"+id) {
		t.Errorf("%d %s", w.Code, w.Body.String())
	}
	cfg := app.store.Get()
	if len(cfg.Beacons) != 2 || cfg.Beacons[0].Name != "Renamed" {
		t.Errorf("%+v", cfg.Beacons)
	}
	page := getSettings(t, &settingsHandler{app: app}) // plain reload: no note
	if strings.Contains(page, "its row was skipped") {
		t.Error("the note must only show when asked for")
	}
	req := httptest.NewRequest(http.MethodGet, "/settings?note=skipped:"+id, nil)
	rec := httptest.NewRecorder()
	h.handleGet(rec, req)
	if !strings.Contains(rec.Body.String(), "Beacon "+id+" is no longer listed; its row was skipped.") {
		t.Error("the note did not render")
	}
	rec = httptest.NewRecorder()
	h.handleGet(rec, httptest.NewRequest(http.MethodGet, "/settings?note=skipped:<script>", nil))
	if strings.Contains(rec.Body.String(), "<script>") || strings.Contains(rec.Body.String(), "no longer listed") {
		t.Error("a note that is not public ids must be ignored")
	}
}

func TestSettingsPost_aTabFromAnEarlierProcessCannotPromoteAnInMemoryId(t *testing.T) {
	// The reuse of a never-saved id after a restart meets the merge only if a
	// stale tab can save at all; the form token is per process, so it cannot.
	app := twoBeaconApp(t)
	id := plant(app, &countingBeacon{})
	earlier := &settingsHandler{app: app}
	h := &settingsHandler{app: app}
	form := twoBeaconForm()
	form.Set("csrf", earlier.csrf())
	form.Set(id+".name", "Hijack")
	w := postSettings(t, h, form)
	if w.Code != http.StatusForbidden || len(app.store.Get().Beacons) != 2 {
		t.Errorf("%d, %d records: a stale tab promoted a light", w.Code, len(app.store.Get().Beacons))
	}
}

func TestSettingsPost_identifyActsOnThatBeaconAndSavesNothing(t *testing.T) {
	app := twoBeaconApp(t)
	left := &countingBeacon{}
	app.rebind(&fakeHandle{Beacon: left, key: "port:p-left", live: true})
	h := &settingsHandler{app: app}
	form := twoBeaconForm()
	form.Set("b1.name", "Half-typed")
	form.Set("action", "identify:b1")
	w := submitSettings(t, h, form)
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/settings" {
		t.Errorf("%d %s", w.Code, w.Header().Get("Location"))
	}
	if left.flashes != 1 || app.store.Get().Beacons[0].Name != "Desk left" {
		t.Errorf("flashes %d, name %q: Identify flashes and saves nothing", left.flashes, app.store.Get().Beacons[0].Name)
	}
	form.Set("action", "identify:b2") // unplugged
	if w := submitSettings(t, h, form); w.Code != http.StatusConflict {
		t.Errorf("Identify on an unplugged beacon: %d, want 409", w.Code)
	}
	form.Set("action", "identify:b9")
	if w := submitSettings(t, h, form); w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "reopen Settings") {
		t.Errorf("Identify on a gone id: %d", w.Code)
	}
}

func TestSettingsPost_forgetActsOnThatBeaconOnly(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	form := twoBeaconForm()
	form.Set("action", "forget:b2")
	if w := submitSettings(t, h, form); w.Code != http.StatusSeeOther {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	cfg := app.store.Get()
	if len(cfg.Beacons) != 1 || cfg.Beacons[0].ID != "b1" || !app.IsTombstone("b2") {
		t.Errorf("%+v", cfg.Beacons)
	}
	form.Set("action", "forget:b2")
	if w := submitSettings(t, h, form); w.Code != http.StatusConflict {
		t.Errorf("a second Forget of the same id: %d, want 409, never another row", w.Code)
	}
	form.Set("action", "forget:b1") // plugged in
	if w := submitSettings(t, h, form); w.Code != http.StatusConflict || len(app.store.Get().Beacons) != 1 {
		t.Errorf("Forget on a plugged-in beacon: %d", w.Code)
	}
}

func TestSettingsPost_takeOverKeepsTheOldRecordAndMovesTheLight(t *testing.T) {
	app := twoBeaconApp(t)
	h := &settingsHandler{app: app}
	moved := &countingBeacon{}
	id := plant(app, moved) // Desk right's light, found in a new port
	form := twoBeaconForm()
	form.Set(id+".takeover", "b2")
	form.Set(id+".name", "ignored: the row is the identity")
	if w := submitSettings(t, h, form); w.Code != http.StatusOK {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	cfg := app.store.Get()
	if len(cfg.Beacons) != 2 || cfg.Beacons[1].ID != "b2" || cfg.Beacons[1].Name != "Desk right" || cfg.Beacons[1].Key != "port:p"+fmt.Sprint(plantSeq) || !reflect.DeepEqual(cfg.Beacons[1].Lines, []int{2}) {
		t.Errorf("%+v", cfg.Beacons)
	}
	if _, ok := app.View(id); ok {
		t.Error("the new light's record survived")
	}
	v, _ := app.View("b2")
	if !v.Connected {
		t.Error("Desk right is not driven at once")
	}
	app.Command("b2", StateSpin)
	if moved.spins != 1 {
		t.Error("the moved handle is not b2's")
	}
	// Take over onto a plugged-in record, or from a configured row, is refused.
	form = twoBeaconForm()
	form.Set("b2.takeover", "b1")
	if w := submitSettings(t, h, form); w.Code != http.StatusBadRequest {
		t.Errorf("Take over from a configured row: %d, want 400", w.Code)
	}
}

func TestSettingsForm_rendersTheRowsFromTheRegistryWithTheCachedLines(t *testing.T) {
	app := twoBeaconApp(t)
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()
	app.updateConfig(func(c *Config) error { c.DashboardURL, c.APIToken = srv.URL, testToken; return nil })
	plant(app, &countingBeacon{}) // a new light
	body := getSettings(t, &settingsHandler{app: app, client: srv.Client()})
	for _, want := range []string{`name="b1.lines" value="3"`, `name="b2.lines" value="4"`, `Desk left`, `Desk right`, `New beacon`, `b3 · key port:`, `name="b3.takeover"`, `<option value="b2">Desk right, moved from its old port</option>`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(body, `name="line_number"`) {
		t.Error("the one-line picker is still on the page")
	}
}

func TestSettingsForm_documentsTheBeaconParameterAndTheList(t *testing.T) {
	body := getSettings(t, &settingsHandler{app: twoBeaconApp(t)})
	for _, want := range []string{"/spin?beacon=b1", "<code>/beacons</code>", "no such beacon", "curl http://127.0.0.1:1337/beacons"} {
		if !strings.Contains(body, want) {
			t.Errorf("API block lacks %s", want)
		}
	}
}
```

- [ ] **Step 2: Run red.** Compile errors; existing tests posting `line_number` fail. **Disarms**
  once implemented: Promote called for every in-memory row → `namingANewLight…` red on its first
  half; the tombstone check after the write → `aForgottenId…` red (name written); the Take over
  select rendered unconditionally → `takeOverOnlyWhenARecordIsUnplugged` red; `validateName` with
  `len()` → `countsNameLengthInRunes` red; the note rendered from the raw query →
  `aVanishedRow…`'s script case red.

- [ ] **Step 3: Implement** `settings_beacons.go`, then `settings_handler.go`'s template and
  handlers. Keep every rule the token field has today (`settings_handler.go:291-310`); the
  `dashboard_url` change still clears the token.

- [ ] **Step 4: Adapt the existing tests, honestly.** The field names in `beaconLightForm()` →
  `twoBeaconForm()`'s `b1.*` names (one helper; the tests that call it change nothing else);
  `line_number` in `TestSettingsPost_savesTheWatcherFields`, `savesTheLineWhetherOrNotThePickerRendered`,
  `keepsTheTokenWhenOnlyTheLineChanges`, `aFormWithoutTheNewFieldsSavesTheDefaults` → `b1.lines`;
  `TestSettingsPost_rejectsAnImpossibleLineNumber`'s expectation **changes**: a `b1.lines_text` of
  `0` is refused with 400 and nothing is saved (the page is stricter than the file, as every other
  field already is); the picker assertions in `picksTheLineFromTheDashboardList`,
  `showsALineWithNoNumberButRefusesToBindIt`, `keepsASavedLineTheDashboardDidNotList`,
  `fallsBackToTheTypedNumberAndSaysWhy`, `fallsBackWhenTheDashboardListsNoLines`,
  `showsTheWatcherFields` and `lines_cache_test.go`'s `pickerMarkup` become checkbox assertions
  (`<input type="checkbox" name="b1.lines" value="3" checked> Line C · running`; a line with
  `n: null` is rendered disabled and unticked; the fallback is `name="b1.lines_text"` with the
  reason). Measured at `1e4784c`: 33 lines in `settings_handler_test.go`, 16 in the moved tests,
  1 in `lines_cache_test.go` carry old names or markup. Report each changed assertion in the task
  report.

- [ ] **Step 5: Verify** suite, vet both, gofmt; render the page once in a browser against the
  Windows exe is T10's.

- [ ] **Step 6: Commit** in two: `feat(tray): the settings page shows one row per beacon — name, lines, light settings, Identify, Forget and Take over` and `test(tray): the settings tests post the per-beacon form`.

**Acceptance:**
- Every scenario in §14.2's list that names the settings page passes; the Take over control is
  absent when no record is unplugged.
- A refused save writes nothing (every 400/409 test checks the store afterwards).
- Every operator string on the page goes through `html.EscapeString` (the `inject` tests).
- The API block documents `?beacon=` and `GET /beacons` with copy-pasteable examples.

**Size:** L (about 420 lines of code, 520 of test, 50 adapted). **Seat:** `cwd-implementer-opus5`:
the operator's real config is written from this form, and the CSRF and escaping rules are a
security surface.

**Plan gate:** rule 1: n/a. **Rule 2, payload checklist:** the page lists N rows (bounded by the
registry), each with a name ≤ 40 runes, a key ≤ ~60 chars, ≤ 99 checkboxes; the 409 and the note
carry public ids only; the API block is constant text plus the escaped base URL. **Rule 3:** the
form parser ends with a smoke on the real-shape form a browser posts (`twoBeaconForm`, every field)
and the omitted-key shapes a browser produces (an unticked checkbox sends nothing; no lines
ticked; no `takeover`; a row absent altogether), plus the form from an earlier process. Rule 4:
none. Rule 5: none.

---

## T0b — Fix the identity mode and the preferred usage pair from the probe's file; the stop

**Carries out:** §3 (the probe settles the mode and the collection rule), §5.3 (one mode, a
constant; `MIXED`/`DUPLICATED`/a port-key collision is a stop), §14.1 (what to read in the file),
§12's correction of the upstream comment once the probe has shown what it meant. **Starts only when
`beacon-probe.txt` and the owner's step-4 note are in the master's hands.**

**Files:**
- Create: `cmd/blinkybeacon-tray/identity_mode.go`
- Modify: `cmd/blinkybeacon-tray/identity_test.go` (the probe's real path lines as fixtures)
- Modify: `pkg/fsbeacon/usbhid.go:3` (the package comment)

**Interfaces:**
- Consumes: T0's types.
- Produces:

```go
// identity_mode.go — what the owner's lights are, read from beacon-probe.txt
// on <date>: "serial numbers: <DISTINCT|NONE>".
const identityMode identityMode = identity<Serial|Port>

// preferredUsage is the collection the tray opens when a light shows one
// path per collection; nil when every light shows one path.
var preferredUsage *usagePair = nil // or &usagePair{Page: 0x…, Usage: 0x…}
```

- [ ] **Step 1: Read the file.** From the `summary` block take the mode (table in "The probe, and
  what waits on it"). From the `beacon N` entries take every `path` line and `port key` line into
  the test as fixtures. Check for a stop: `MIXED`, `DUPLICATED`, or two entries with one `port key`
  and one usage pair. **On a stop, this task is not built**; the master records the file against
  the design and the build waits at the end of wave 2.
- [ ] **Step 2: Check the instance segment's shape** against `windowsPortKey`: if the file shows
  two paths of one light (same light flashed for two `beacon N`) whose instance segments differ
  only in a trailing `&000N`, the port key must drop that trailing counter so both paths share one
  key; that is a one-line change to `windowsPortKey` **decided from the file**, with the two real
  paths as its test. If the file shows one path per light, nothing changes.
- [ ] **Step 3: Write the failing test** — the real lines:

```go
func TestIdentity_theOwnersLights(t *testing.T) {
	// Lines from beacon-probe.txt, <date>, verbatim.
	paths := []string{ /* every "path" line */ }
	keys := []string{ /* every "port key" line, in the same order */ }
	for i := range paths {
		if got := windowsPortKey(paths[i]); got != keys[i] {
			t.Errorf("path %d: windowsPortKey = %q, probe said %q", i, got, keys[i])
		}
	}
	seen := map[string]int{}
	for _, k := range keys {
		seen[matchKey(identityMode, "", k)]++ // in port mode; in serial mode use the serial lines
	}
	for k, n := range seen {
		if n > 1 && preferredUsage == nil {
			t.Errorf("key %q held by %d paths with no preferred collection: the build would refuse one", k, n)
		}
	}
}
```

  and, for the mode: `if identityMode != identity<X> { t.Fatal("the probe said <X>") }` with the
  summary line quoted in the test's comment.

- [ ] **Step 4: Implement** the two declarations. Correct `pkg/fsbeacon/usbhid.go:3` to what the
  probe showed: either *"Several beacons can be held open at once; each is addressed by its own
  path. The lights report no serial number, so a light is known by the USB port it is in."* or the
  serial-number version.
- [ ] **Step 5: Verify** suite, vet both, gofmt. **Commit** `feat(tray): the beacon identity is the <port|serial number>, as the owner's lights report it`.

**Acceptance:** the constant matches the file's summary; every real path yields the key the probe
printed; the comment at `usbhid.go:3` no longer says the library cannot tell beacons apart.
**Size:** XS. **Seat:** `cwd-implementer-opus5`: the owner's real data decides the constant.
**Plan gate:** rule 3: this is T0's real-input smoke. Rule 5: if the master writes this task's
two declarations by hand rather than dispatching it, that hunk gets the same `cwd-reviewer` review
as any other — note it in the retro.

---

## T5b — The real USB source and handle; the final `main.go`

**Carries out:** §12 (the device manager over the real library; quit closes every handle), §13
(the macOS identity line), §14.2 (`farmSource`, `farmHandle`, the `beacon_check.go` assertion),
§5.1 (open by path). **After T0b, T1, T3 and T5a.**

**Files:**
- Create: `cmd/blinkybeacon-tray/devices_farm.go` (`//go:build windows || darwin`)
- Create: `cmd/blinkybeacon-tray/devices_farm_windows.go`, `devices_farm_darwin.go` (the port key)
- Modify: `cmd/blinkybeacon-tray/beacon_check.go` (one assertion added)
- Modify: `cmd/blinkybeacon-tray/main.go` (the manager goroutine; `OnQuit` through it)

**Interfaces:**
- Consumes: T1's `fsbeacon.Enumerate/OpenPath/Info/LastAttemptOK/LocationID`; T0's `matchKey`,
  `windowsPortKey`, `darwinPortKey`; T0b's `identityMode`, `preferredUsage`; T5a's
  `newDeviceManager`, `run`.
- Produces:

```go
// devices_farm.go
type farmSource struct{ mode identityMode }

func (farmSource) Enumerate() ([]deviceInfo, error) // maps fsbeacon.DeviceInfo → deviceInfo
func (s farmSource) Open(info deviceInfo) (deviceHandle, error) {
	b, err := fsbeacon.OpenPath(fsbeacon.DeviceInfo{Path: info.Path, SerialNbr: info.Serial, UsagePage: info.UsagePage, Usage: info.Usage})
	if err != nil {
		return nil, err
	}
	h := &farmHandle{FarmBeacon: b, info: info}
	h.key = matchKey(s.mode, info.Serial, h.portKey()) // portKey: platform file; on macOS reads LocationID from the open handle
	return h, nil
}

type farmHandle struct {
	*fsbeacon.FarmBeacon
	info deviceInfo
	key  string
}

func (h *farmHandle) Key() string          { return h.key }
func (h *farmHandle) OnUnplug(f func())    { h.OnError = f }
func (h *farmHandle) Live() bool           { return h.LastAttemptOK() }
// Flash, Spin, Stop, Close, StartIdleWorker: promoted from *fsbeacon.FarmBeacon

// devices_farm_windows.go
func (h *farmHandle) portKey() string { return windowsPortKey(h.info.Path) }
// devices_farm_darwin.go
func (h *farmHandle) portKey() string {
	id, err := h.LocationID()
	if err != nil {
		log.Printf("Beacon at %s: no location id: %v", h.info.Path, err)
		return ""
	}
	return darwinPortKey(id)
}

// beacon_check.go
var _ deviceHandle = (*farmHandle)(nil)
```

  `main.go`: after the HTTP server starts,
  `mgr := newDeviceManager(appState, farmSource{mode: identityMode}, preferredUsage)`,
  `mgrCtx, stopMgr := context.WithCancel(…)`, `go mgr.run(mgrCtx)`; `OnQuit` calls `stopWatch()`,
  `stopMgr()`, waits for `mgr.done` (a channel `run` closes after it has stopped and closed every
  handle; add `done chan struct{}` to the manager in this task), then shuts the HTTP server down.

- [ ] **Step 1: The failing check** is `go vet` under `GOOS=windows` (`undefined: farmSource`) and
  the compile-time assertion. **Disarm:** remove `Live()` from `farmHandle` — the assertion in
  `beacon_check.go` fails to compile under `GOOS=windows`.
- [ ] **Step 2: Implement.** Keep `farmHandle` free of logic beyond the mapping: everything the
  suite can test lives in T5a.
- [ ] **Step 3: Verify.** `go vet` under `GOOS=windows` clean; `go build` the Windows exe with the
  release line and record its sha256 and the SHA it was built from; push and read the `build`
  workflow: Windows and both macOS legs green (the darwin file compiles only there); download the
  macOS arm64 artifact and `unzip -l` it. Suite on Linux unchanged (these files are not compiled).
- [ ] **Step 4: Commit** `feat(tray): the tray opens every beacon it finds, by path, and lets each go when it is unplugged`.

**Acceptance:** vet and the three CI legs green; the exe built and hashed; `OnQuit` closes every
handle before the process exits (the quit test in T5a covers the manager; here it is the wiring,
checked by hand in T10 step 12).
**Size:** M (about 120 lines; nothing the suite runs). **Seat:** `cwd-implementer-opus5`: the
third-party API surface, on two platforms, one of which cannot be compiled here.
**Plan gate:** rule 2: this task is where the 10-byte HID reports are sent, through the unchanged
library; nothing joined, nothing variable. Rule 4: the darwin file compiles nowhere here; the CI
leg is its only check, and it is named as such in the retro. Rule 5: none.

---

## T9 — Docs

**Carries out:** the design's T9 row: `BEACON-DASHBOARD.md` §1 (plug in every beacon), §2 (the
picker is per beacon, and a beacon can follow several lines), §4 (light settings per beacon), §5
(the tray rows: the top row, the dashboard row, one submenu per beacon), §7 (the config sample
becomes §10.1's, and **lines 351–354 are corrected**: *"A pause outranks the flash. If a pause
begins during a flash the beacon switches to spinning; when the pause lifts, the flash resumes
for whatever is left of its window, and the light goes dark when the window has run out."*),
§8 (drop "Only one line at a time"); `README.md`'s HTTP API table (the `?beacon=` rows and
`GET /beacons`, exactly the reply in "What S16 hands to S17"). The settings page's own API text is
T8's.

**Files:** `BEACON-DASHBOARD.md` (the eight sections named, by line: 49–79, 80–114, 162–220,
221–270, 344–419, 420–424 at `1e4784c`), `README.md:86-93`.

- [ ] **Step 1:** Write each section against the shipped behaviour, step by step with §14.3; every
  sentence that names a tray row, a field or a reply is checked against the code (`tray_labels.go`,
  `settings_beacons.go`, `server.go`).
- [ ] **Step 2:** The reviewer reads the guide with the Windows exe running on their own PC where
  possible, else against the test fixtures (`GET /beacons`'s golden string, the tray label tests).
- [ ] **Step 3: Commit** `docs: the user guide and README for several beacons, each following one or more lines`.

**Acceptance:** no sentence in the guide contradicts a test; the pause/flash sentence matches
`TestDecide_aPauseLiftingInsideTheFlashWindowHandsTheLightBackToTheFlash`.
**Size:** S. **Seat:** `cwd-implementer`: transcription of text this plan and the design hold.
**Plan gate:** n/a throughout.

---

## T10 — The owner's hands-on test, the Windows hidden-slot check, CI

**Carries out:** §14.3, with red-team notes 6 and 7 applied to the steps; T7's hand step; §13's
CI check. The last task; nothing ships before it.

**What the master sends the owner** (in one message, with the files attached or linked):

1. `BB-DASH.exe` built from the final SHA with the release line, its sha256, and the SHA. Built
   twice, same hash, recorded in the retro.
2. The steps below, numbered, each with its *Expect* line. The owner replies with the step number
   and what they saw where it differs; a step with no reply is read as *as expected*.
3. A request for three files at the end: `blinkybeacon-config.json` (the token blanked), the
   tray's log if any was written, and `beacon-probe.txt` if it was re-run.

**The steps** (the design's §14.3, with the two wording fixes):

1. **Upgrade in place.** Quit the old tray. Put the new `BB-DASH.exe` beside the old
   `blinkybeacon-config.json`. Plug in **one** light only. Start the tray. *Expect:* the tray
   behaves as before: the top row `● Beacons: 1 of 1 plugged in`, one submenu `Beacon 1 — Idle ·
   <the old line>`, the light reacts to a pause on it. Open Settings: one row, `Beacon 1`, following
   the old line, with the old light settings. *(This is the real-input smoke of plan-gate rule 3 for
   the loader: the owner's own file.)*
2. **Name it.** Type `Desk left`, Save. *Expect:* the row says `Desk left`; the config file now has
   a `beacons` list with `"id": "b1"` and a `key`.
3. **Plug in the second light.** Within ten seconds, *expect:* a `New beacon (b2) — Idle · Unbound`
   submenu and a new row on the settings page, following nothing. Pausing Line A spins Desk left only.
4. **Identify.** Click Identify on the new row. *Expect:* the second light flashes for about two
   seconds; the first does not. Name it `Desk right`, tick Line B, Save.
5. **Two lines, two lights.** Pause Line B. *Expect:* Desk right spins, Desk left dark. Pause Line A
   too. *Expect:* both spin. Unpause A. *Expect:* only Desk right spins.
6. **One light, two lines.** On Desk left tick Line A **and** Line B, set its Draft-End Flash to
   `60`, Save. Pause B. *Expect:* both spin. End a draft on A while B is paused. *Expect:* both keep
   spinning. Unpause B **within the minute**. *Expect:* Desk left **flashes** for what is left of the
   60 s, then goes dark; Desk right dark. Put the flash back to `15`, Save; repeat, but wait **more
   than 15 s** before unpausing. *Expect:* Desk left goes dark.
7. **Different settings on one line.** Desk left's draft flash 5 s, Desk right's 20 s, both on
   Line A only. End a draft on A. *Expect:* both flash; Desk left stops after about 5 s, Desk right
   after about 20.
8. **The Companion buttons and the routes.** The module's Spin button: *expect* both lights spin;
   Stop stops both. From a terminal: `curl -X POST http://127.0.0.1:1337/flash?beacon=b1`, then
   `curl -X POST "http://127.0.0.1:1337/flash?beacon=Desk%20left"`. *Expect:* only Desk left
   flashes, both times; `curl http://127.0.0.1:1337/status` says `{"state":"flash","connected":true}`;
   `curl http://127.0.0.1:1337/beacons` lists both with their states. (Untick the lines first for a
   longer look; the watcher corrects a bound light on its next tick.)
9. **Unplug mid-pause.** Pause B; Desk right spins. Pull its USB. *Expect:* within about five seconds
   the submenu says `Desk right — Unplugged · Line B`; Desk left is untouched. Plug it back into the
   **same** port. *Expect:* within about ten seconds it is `Desk right` again and **spins**.
10. **Port move.** Move Desk right to a **different** port. *Expect, if the probe found no serial
    numbers:* `Desk right — Unplugged` stays and `New beacon (b3)` appears. On the settings page the
    new row has a control `This light is: — / Desk right, moved from its old port`; choose Desk
    right, Save. *Expect:* one row, `Desk right`, **still `b2`**, in the new port, following B, and
    spinning at once if B is still paused; `b3` is gone; `curl -X POST http://127.0.0.1:1337/flash?beacon=b2`
    still flashes it. **If no record is unplugged** when a new light appears (say, Desk right had
    been forgotten), the new row shows no such control: it is only offered while some beacon is
    unplugged. *If the probe found serial numbers:* nothing changes but the port; it is `Desk right`
    as soon as it is found.
11. **A third light, never configured.** Plug in the third. *Expect:* `New beacon`, follows nothing;
    the module's Spin button spins all three. Unplug it. *Expect:* it vanishes from the tray and the
    page; nothing was written to the config for it.
12. **Quit** with all lights spinning. *Expect:* all go dark.
13. **The hidden-slot check (Windows).** With three lights: unplug the second, **Forget** it on the
    settings page (its submenu disappears), plug it back in (a `New beacon` submenu appears — in the
    place the forgotten one had, between the other two), open it: *expect* six rows, status,
    following and the four buttons, and Identify flashes that light.
14. **Token revoked** (optional): revoke the token. *Expect:* every bound light goes dark on the
    next poll, the dashboard row says `▲ Dashboard: token rejected`; after re-minting and pasting,
    `watching` returns within thirty seconds.

**What comes back:** the owner's reply per step; the three files. The master records each step's
outcome in the retro, hash-matches the exe, and reads the `build` workflow on the final SHA
(Windows, macOS arm64 with the library tests, macOS amd64). A step that failed is a task-review
finding against the task that owns the behaviour (the table at the top), fixed on the build branch
and re-run from that step.

**Size:** the owner's hour. **Seat:** none; the master and the owner.

---

## Where each red-team note landed

| # | The note | Placed in | How |
|---|---|---|---|
| 1 | Make the callback's "mark unplugged" identity-checked, or move the mark into the manager after its check | **T3** (`dropHandle` marks only the entry that holds exactly that handle, else ignores it; `TestRegistry_dropHandleIgnoresAHandleNoEntryHolds`) and **T5a** (`closeDropped` closes only a handle it still lists) | the callback is `app.dropHandle(h)`, identity-checked twice: at the mark and at the close |
| 2 | T5's "displaced handle's late callback" has no reachable producer | **T5a**: the synthetic scenario is **dropped**; its reachable cousin replaces it — one handle delivered twice (a failed command and the keepalive in one window), closed once (`TestManager_sameHandleDeliveredTwiceIsClosedOnce`) | the library's worker exits after one failure, so no handle fires twice; two *deliveries* of one handle do happen, and the same check covers them |
| 3 | `LastAttemptOK` must start `true` at open | **T1** (`newFarmBeacon` stores true; the worker records the initial write; `TestFarmBeacon_lastAttemptOKStartsTrue`) | |
| 4 | A never-saved in-memory id reused after a restart meets the merge | **Set aside with a reason, and pinned in T8**: the form token is per process (`settingsHandler.csrf`, `settings_handler.go:151-159`, minted once per handler, and a restart makes a new handler), so a tab from before a restart cannot save at all — 403, nothing written (`TestSettingsPost_aTabFromAnEarlierProcessCannotPromoteAnInMemoryId`). Within one process the counter advances in memory and is never reused. **And** T2/T3 give the reservation anyway: `updateConfig` stamps `beacon_seq` with the live counter on every save, so an in-memory id issued before any later save is reserved on disk (`TestRegistry_theLiveCounterIsReservedByTheNextSaveOfAnything`) | the alternative the note offered ("bump on the first save after an in-memory id was issued") is what `updateConfig` does |
| 5 | The `port`-mode twin cannot occur with two lights at once; the tray sentence must not promise a cause | **T7** (`beaconRowTitle`: ` · ▲ another light reports this identity and was not opened` — what happened, no why; `TestTrayLabels_beaconRows`), **T5a** (`markSameIdentity`), and the design's §5.3 and §12 item 1 wording (below) | |
| 6 | §5.3 "never stuck longer than one enumerate": worst case is a keepalive period plus an enumerate (≈15 s) | the design's §5.3 and §12 item 1 wording (below); **T5a** keeps the bound and tightens it in practice: a path missing from enumeration is dropped at the next scan (`TestManager_aVanishedPathIsAnUnplugEvenBeforeTheKeepaliveRuns`), an addition the design does not name — **for the master to keep or strike** | the 15 s bound is stated; the sweep makes the common case one enumerate |
| 7 | Step 10 and T8: say what the page shows when no record is unplugged | **T8** (`beaconRowsHTML` renders no Take over control then; `TestBeaconRowsHTML_takeOverOnlyWhenARecordIsUnplugged`), **T10** step 10, and the design's §11.2 and §14.3 step 10 wording (below) | |

## The design's wording fixes made on this branch (notes 5–7)

Four sentences in `docs/specs/2026-10-20-several-beacons.md`, listed for the PR:

- §5.3, last sentence: "Loud, never wrong, and never stuck behind a stale handle for longer than
  one enumerate." → "… for longer than one keepalive period and one enumerate, about 15 s."
- §12 item 1, third bullet: "a true twin (in `sn` mode a duplicate serial bought later; in `port`
  mode a second light Windows gave the same instance segment): the path is not opened and the
  tray says so" → "a second light reporting an identity already held: the path is not opened and
  the tray says so, without naming a cause"; and "hands over within one enumerate, at most about
  10 s" → "hands over within one keepalive period and one enumerate, about 15 s".
- §11.2, the Take over bullet: adds "When no configured record is unplugged, the row shows no such
  control."
- §14.3 step 10: adds the same sentence after "choose it, Save."

## The plan gate, answered task by task

| Task | Rule 1 numeric sweep | Rule 2 payload | Rule 3 loader/parser smoke | Rule 4 negative env | Rule 5 controller hunks |
|---|---|---|---|---|---|
| T0 | public ids to 10⁷; pinned `b1`–`b3` are plain members | — | Windows path shape (real lines in T0b) | — | — |
| T1 | — | the three unchanged 10-byte reports | — | Linux without `libudev.h` (compile fails: documented quirk) and a light-less macOS runner (SKIP) | — |
| T2 | 500 random id lists, numbers to 10⁶; pinned `b01`, duplicate `b1`, `B2`, `beacon_seq: 1`; the sample's `3` coincides with nothing | the JSON file; lists bounded by record count | v0.5.0, v0.7.0, v0.8.0 (exact bytes), keys-omitted, new shape; load writes nothing; the owner's file in T10 step 1 | — | — |
| T3m | — | — | — | — | — |
| T3 | every multiset of ≤ 6 (connected, state) pairs; pinned: the §9.2 table; one-beacon pass-through is the intended coincidence | today's replies, unchanged | — | — | — |
| T4 | 3¹…3⁶ combine, checked reversed, idempotent, monotone; `Retry-After` −100…100,000 s as numbers and dates; pinned 12 s, 120 s, absent; coincidences named: 10 s = enumerate cadence, 30 s = `rejectedBackoff` = `feedLostAfterSeconds` | dashboard GETs, no body; the log's line list bounded | `Retry-After` on the RFC's example date, `http.TimeFormat`, integers, absent | — | — |
| T5a | — | — (the fake receives the writes) | — | — | — |
| T6 | — | `/status`, control replies: two fixed keys; `/beacons`: N × (≤ 40-rune name + ≤ 99 ints); error bodies fixed | `?beacon=` as the module (no body), curl (percent-encoded), a form body, a malformed body, empty | — | — |
| T7 | — | menu titles: ≤ 4 lines (labels cut at 24 runes), ≤ 8 beacons in the tooltip, names ≤ 40 runes | — | — | — |
| T8 | — | page rows bounded as `/beacons`; 409 and the note carry public ids only | the browser's full form; unticked boxes, no lines, no takeover, absent row; an earlier process's form | — | — |
| T0b | — | — | the probe's real `path` and `port key` lines | — | if hand-written by the master: same review, noted in the retro |
| T5b | — | the HID reports through the unchanged library | — | the darwin file compiles only on the macOS CI leg; named in the retro | — |
| T9 | — | — | — | — | — |
| T10 | — | — | the owner's real config file loads as one beacon | — | — |

No test in this plan shells out to git. The only tests that import a cgo package are the
library's (T1), and their negative environment is recorded there.

## What S16 hands to S17

The Companion module's picker (`clovisd/blinkybeacon-companion`, its own stage in its own
repository) uses exactly this. The tray's side is built by T6 and documented by T8 and T9.

**`GET /beacons`** — no authentication (D3; the owner ruled `/status` stays open, and this is the
same kind of read), `Cache-Control: no-store`, `Content-Type: application/json`, `405` for any
other method:

```json
{
  "v": 1,
  "beacons": [
    {"id": "b1", "name": "Desk left",  "connected": true,  "state": "spin", "lines": [1, 3], "watch": "ok"},
    {"id": "b2", "name": "Desk right", "connected": false, "state": "idle", "lines": [2],    "watch": "no-line"},
    {"id": "b3", "name": "",           "connected": true,  "state": "idle", "lines": [],     "watch": "off"}
  ]
}
```

- `v` is `1`. `beacons` is in the tray's order: configured beacons in config order (unplugged ones
  included, `connected: false`), then lights seen but not yet saved, in order of appearance.
- `id`: `b` and a number; lowercase letters and digits only; stable for a saved beacon across
  restarts and port moves; never reused after a Forget. `name`: the operator's, `""` until named
  (the picker shows `New beacon (b3)` then). `state`: `idle` | `spin` | `flash`. `lines`: the
  dashboard line numbers it follows, `[]` for none (never `null`). `watch`: one of `off`,
  `no-token`, `ok`, `lost`, `rejected`, `no-line`, `stopped`, `no-game-yet`, `quiet`, `throttled`.

**`?beacon=` on `POST /spin`, `/flash`, `/stop`** — the paths and the no-parameter replies are
unchanged (`{"state", "connected"}`; `503 {"error": "beacon not connected"}` with none plugged in;
`503 {"error": "beacon disconnected"}` when every command failed). With the parameter:

- The value is a public id (`b1`) or the exact name (`Desk left`), percent-encoded as any query
  value (`?beacon=Desk%20left`); the id is tried first, and a name can never look like an id.
  The picker should send the **id**.
- Repeat the parameter for several: `?beacon=b1&beacon=b2`. The same beacon twice is driven once.
- Replies: `200 {"state", "connected"}` about the named set when at least one named beacon took
  the command; `503 {"error": "beacon not connected"}` when every named beacon is unplugged;
  `503 {"error": "beacon disconnected"}` when every named connected beacon's command failed (they
  are dropped for reconnection); `404 {"error": "no such beacon"}` when any value names nothing,
  and `400 {"error": "beacon must name a beacon"}` when any value is empty or whitespace — both
  before anything is driven; `405` for a wrong method.
- `GET /status` ignores the parameter and answers for the beacons as a whole (D2): `connected` when
  any is plugged in; `state` the busiest connected beacon's (`spin` > `flash` > `idle`).
- The token (S17/S18) will ride a header; the query string says *which*. The control routes with
  `?beacon=` get the same token check as without, when S18 lands.

## Changes if the owner picks an alternative

| Decision | The alternative | What changes |
|---|---|---|
| **D1** the tray guesses a port move | re-bind the one unplugged record to the one new light by itself | **T5a** `scan`: when `bind` would create an in-memory entry and exactly one configured record is unplugged and no other light is new this scan, call `app.TakeOver(thatID, newID)` instead; one new test (`TestManager_aLoneNewLightTakesTheLoneUnpluggedRecord`) and the inverse (two unplugged: no guess). **T8**: the Take over control stays for the cases the guess does not cover. **T10** step 10's expectation. The design's own warning (a borrowed light inherits a drawer light's lines) stands. In `sn` mode nothing changes. |
| **D2** `connected` only when every configured beacon is plugged in | | **T3** `Aggregate`: `connected = every configured entry has a handle` (in-memory entries do not count); the sweep's expectation; **T6**'s `/status` tests; the Companion module's "Beacon connected" feedback goes red with a spare in a drawer. `state` unchanged. |
| **D3** `GET /beacons` behind the token | | **T6**: nothing now — S18 adds the check to four routes instead of three; T8's API text says "needs the token" on that row. |
| **D4** a flat list instead of submenus | | **T7**: `tray_slots.go` builds 8 × 4 root items (`Desk left: Spin`, …) plus a disabled status row each, same hide/show rule, same fake; `tray_labels.go` adds the `Name: Action` titles; the tests' fixture changes; the design's eight-slot limit and ordering rule stay. T10 step 13 changes shape. |
| **D5** drop the mirror keys | | **T2**: `mirrorFirstBeacon` and the five fields go; the loader reads them only when `beacons` is absent (migration keeps working); `TestConfig_theMirrorIsWrittenFromTheFirstBeacon` is replaced by "no mirror key is written"; T9's config sample. A downgrade then reads `line_number` as missing and binds the first beacon to line 1. |
| **The probe says `DISTINCT`** | (the recommendation is whichever the file says) | **T0b** sets `identitySerial`; `TestManager_aTrueTwinIsRefusedEveryScanAndNamed` is the live guard; **T10** step 10's serial branch. No other task changes. |

## Self-review

Run with the design open beside the plan after the last section was written.

1. **Spec coverage.** §1 → T6/T7/T8/T10; §2 D1–D5 → the table above; §3 → T0b and "The probe";
   §4 → T10 (the probe steps are the owner's, done); §5.1 → T1, T5b; §5.2 → T0, T0b; §5.3 → T0,
   T0b, T3, T5a; §5.4 → T3 (`Identify`, `Resolve`), T8 (names); §6 → T2, T3; §7.1 → T4; §7.2 →
   T4; §7.3 → T3, T4; §8.1 → T4; §8.2 → no code (arithmetic); §9.1 → T6; §9.2 → T3, T6; §9.3 →
   T6; §10.1 → T2; §10.2 → T2 (step 1), T3 (step 2), T10 (step 3); §10.3 → T3, T4; §10.4 → T3;
   §10.5 → T2, T3, T8; §11.1 → T7; §11.2 → T8; §12 → T3, T5a, T5b; §13 → T5b (the darwin line),
   no other code; §14.1 → done (the probe); §14.2 → T3, T5a; §14.3 → T10; §14.4 → every task;
   §15 → this plan; §16 → no code; §17 → no code. The amendments' rulings are each carried by the
   section they changed. **No section is unplanned.**
2. **Placeholders.** The only angle brackets are in T0b, where the value is the probe file's and
   the plan says so; no "TBD", no "handle edge cases".
3. **Type consistency.** Everything two wave-2 tasks both name is declared in wave 1 (T3) so
   that no wave-2 task depends on another's landing order: `beaconView` (`ID, Name, Key, Lines,
   Light, InMemory, Connected, State, Hand, Identifying, SameIdentity`), `releaseToHand`,
   `keys()`, `WatchThrottled`, `WatchDetail.RetryInSeconds`, `errBeaconGone/Unplugged/InUse`,
   `settingsHandler.app`, and the test helpers `plant`, `plantKeyed`, `viewHandle`,
   `fakeHandle`, `newTestApp`, `newTestAppNoTemp`. `countingBeacon`, `mockBeacon` and
   `flashCounter` stay in the files that define them today. `stateOf`, `twoBeaconConfig` and
   `linesDashboard` are T4's and used only there; `twoBeaconServer`, `post`, `get` are T6's;
   `twoBeaconForm`, `twoBeaconApp` are T8's; `beaconRowTitle` and the other words are T7's and
   named by no other task's test. The signatures named in a task's Consumes block match the
   Produces block of the task that owns them, checked by grep over this file for each name.
4. **Review Focus.** Each of the five has its named test in its task (T3, T8, T4, T7, T6).
