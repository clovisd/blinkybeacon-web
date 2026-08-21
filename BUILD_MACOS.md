# Building blinkybeacon-tray for macOS

The tray app needs cgo on macOS: `systray` talks to Cocoa and `go-hid` to IOKit.
That means a Mac (or a macOS CI runner) with the Xcode command line tools —
**it cannot be cross-compiled from Linux** without an Apple SDK, which is why
the Windows build has a cross-compile recipe and this one does not.

Go 1.25+ is required (`go.mod` says `go 1.25.0`).

## Option A — GitHub Actions (what releases use)

[`.github/workflows/build.yml`](.github/workflows/build.yml) builds both
architectures on a macOS runner on every push to `main`, and attaches them to the
GitHub release on a `v*` tag:

| Artifact | Contents |
|---|---|
| `blinkybeacon-tray-macos-arm64.zip` | Apple Silicon: `BlinkyBeacon.app` + the bare `blinkybeacon-tray` binary |
| `blinkybeacon-tray-macos-amd64.zip` | Intel: the same pair |

Each zip has a `.sha256` beside it. Run the workflow by hand from the Actions
tab (`workflow_dispatch`) to get artifacts for any branch.

## Option B — build on a Mac

```bash
xcode-select --install        # once: clang + the SDK

# For the machine you are on:
CGO_ENABLED=1 go build -trimpath -ldflags='-buildid=' -o blinkybeacon-tray ./cmd/blinkybeacon-tray/

# Or name the architecture (both build from either kind of Mac):
CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags='-buildid=' -o blinkybeacon-tray ./cmd/blinkybeacon-tray/
CGO_ENABLED=1 GOARCH=amd64 go build -trimpath -ldflags='-buildid=' -o blinkybeacon-tray ./cmd/blinkybeacon-tray/

# Wrap it in an app bundle (no Terminal window, no Dock icon):
scripts/macos-bundle.sh blinkybeacon-tray BlinkyBeacon.app
```

`go test ./cmd/blinkybeacon-tray/` runs the suite, including the macOS-only
start-at-login tests.

## Running it

- The bare binary works from a terminal: the icon appears in the menu bar and the
  HTTP server starts on `127.0.0.1:1337`.
- `BlinkyBeacon.app` is the same binary with an `Info.plist` that hides the Dock
  icon (`LSUIElement`). Drop it in `/Applications` or `~/Applications`.
- **The builds are not signed or notarized.** Gatekeeper will refuse the first
  launch: right-click → **Open**, or clear the quarantine flag:
  `xattr -dr com.apple.quarantine BlinkyBeacon.app`.
- The config file lives in `~/Library/Application Support/BlinkyBeacon/blinkybeacon-config.json`
  (on Windows it sits next to the `.exe`).
- **Start at login** writes `~/Library/LaunchAgents/com.blinkybeacon.tray.plist`
  pointing at the binary you launched; move the app and toggle it off and on again.

## USB access

No driver or permission prompt is needed: the beacon is a plain HID device and
macOS lets user processes open those. If the app says the beacon is not
connected, check `system_profiler SPUSBDataType | grep -A8 -i 340d` for vendor
`0x340d` / product `0x1710`.
