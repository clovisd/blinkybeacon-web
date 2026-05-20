# Blinkybeacon

_A set of utilities for working with beacon lights, currently just the USB one included with the Farming Simulator 22 Collector's Edition._

> **This is a fork of [duckfullstop/blinkybeacon](https://github.com/duckfullstop/blinkybeacon)** that adds `blinkybeacon-tray`, a Windows system tray utility with an embedded HTTP server for controlling the beacon from external tools (BitFocus Companion, scripts, browsers, etc.).

---

## blinkybeacon-tray (added in this fork)

A Windows system tray app that:

- Owns the USB HID connection to the beacon (sole process)
- Runs an HTTP server on `localhost:1337`
- Provides a right-click tray menu with direct Spin / Flash / Stop controls
- Shows beacon state in the tray icon tooltip
- Optional auto-start at Windows login via registry

### Build (Windows, native Go)

```powershell
go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

### Build (cross-compile from Linux/WSL2)

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

Requires `gcc-mingw-w64-x86-64` (`sudo apt install gcc-mingw-w64-x86-64`).

### HTTP API

Base URL: `http://localhost:1337`  (configurable via `--port` flag)

| Method | Path | Description | Response |
|--------|------|-------------|----------|
| `POST` | `/spin` | Start spinning | `200 {"state":"spin"}` · `503` if disconnected |
| `POST` | `/flash` | Start flashing | `200 {"state":"flash"}` · `503` if disconnected |
| `POST` | `/stop` | Stop beacon | `200 {"state":"idle"}` · `503` if disconnected |
| `GET` | `/status` | Current state | `200 {"state":"spin"\|"flash"\|"idle","connected":true\|false}` |

The server only listens on `localhost` — not exposed on the network.

### BitFocus Companion module

A companion module that uses this HTTP API is available at [clovisd/blinkybeacon-companion](https://github.com/clovisd/blinkybeacon-companion).

---

## Why on earth

Were you lucky enough to have the Farming Simulator 22 Collector's Edition magically appear on your desk?
If so, you'll already appreciate the fact that it has a super cool USB beacon included -
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
