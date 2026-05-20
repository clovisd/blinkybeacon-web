# Blinkybeacon

_A set of utilities for working with beacon lights, currently just the USB one included with the Farming Simulator 22 Collector's Edition._

> **This is a fork of [duckfullstop/blinkybeacon](https://github.com/duckfullstop/blinkybeacon)** that adds `blinkybeacon-tray`, a Windows system tray utility with an embedded HTTP server for controlling the beacon from external tools (BitFocus Companion, scripts, browsers, etc.).

---

## blinkybeacon-tray

A Windows system tray app that:

- Owns the USB HID connection to the beacon (sole process — the beacon can only be opened by one program at a time)
- Runs an HTTP server (default `127.0.0.1:1337`) controllable from Companion, scripts, or any HTTP client
- Provides a right-click tray menu with Spin / Flash / Stop controls
- Shows beacon state in the tray icon tooltip
- Configurable bind address and port — supports multiple instances on different ports
- Settings UI accessible from the tray menu (opens in browser)
- Optional auto-start at Windows login via registry

### Download

Pre-built `.exe` available on the [Releases](../../releases) page.

### Build (cross-compile from Linux/WSL2)

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

Requires `gcc-mingw-w64-x86-64` (`sudo apt install gcc-mingw-w64-x86-64`).

### Build (Windows native)

```powershell
go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

### Configuration

**Tray menu → Settings…** opens a browser-based settings page where you can change the bind address and port. Settings are saved to `blinkybeacon-config.json` next to the `.exe` and applied immediately (HTTP server restarts on the new address).

Command-line flags override saved config on first launch and save for future runs:

```
blinkybeacon-tray.exe                           # use saved config (default: 127.0.0.1:1337)
blinkybeacon-tray.exe --addr 0.0.0.0 --port 1338
```

### HTTP API

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| `POST` | `/spin` | Start spinning | `200 {"state":"spin","connected":true}` · `503` if beacon not connected |
| `POST` | `/flash` | Start flashing | `200 {"state":"flash","connected":true}` · `503` if beacon not connected |
| `POST` | `/stop` | Stop beacon | `200 {"state":"idle","connected":true}` · `503` if beacon not connected |
| `GET` | `/status` | Current state | `200 {"state":"spin"\|"flash"\|"idle","connected":true\|false}` |
| `GET` | `/settings` | Settings UI | HTML form for address/port configuration |
| `POST` | `/settings` | Save settings | Saves config and restarts HTTP server |

### BitFocus Companion module

A companion module for this HTTP API is available at [clovisd/blinkybeacon-companion](https://github.com/clovisd/blinkybeacon-companion).

### Protocol notes

The beacon supports exactly three modes — stop, spin (rotating amber), and flash (strobe). No brightness, speed, or colour control has been found; the USB HID protocol uses three fixed 10-byte reports with no additional parameters. See [pkg/fsbeacon/usbhid.go](pkg/fsbeacon/usbhid.go) for the raw bytes.

---

## Why on earth

Were you lucky enough to have the Farming Simulator 22 Collector's Edition magically appear on your desk?
If so, you'll already appreciate the fact that it has a super cool USB beacon included —
and the best part about it is that it synchronises with the in-game tractor! The immersion factor is truly off the scale.

But this begs the question: if the game can make the super cool siren blink and spin, why can't _we_ hook that into whatever else takes our fancy?

Here's some really silly ideas:
* Have the beacon go off whenever your local sportsball team scores a goal
* Make the beacon spin when your server is down or on fire
* Strobe the beacon when it's time for a dance party

## Original utilities

This repo also contains the original `fsbeacon` package and CLI from upstream — see [pkg/fsbeacon/README.md](pkg/fsbeacon/README.md) for details.

## License

MIT — see [LICENSE](LICENSE).
