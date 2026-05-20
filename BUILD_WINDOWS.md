# Building blinkybeacon-tray on Windows

The tray app uses `go-hid` which requires CGO and the Windows HID drivers. Cross-compiling from Linux requires `mingw-w64` which is not set up in this repo's dev environment. Build on Windows instead.

## Prerequisites

1. Install Go 1.22+ from https://go.dev/dl/
2. Install Git for Windows
3. Clone this repo

## Build

```powershell
git clone https://github.com/YOUR_USERNAME/blinkybeacon.git
cd blinkybeacon
go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

The `-H windowsgui` flag suppresses the console window so only the tray icon appears.

## Run

Double-click `blinkybeacon-tray.exe`. The USB beacon must be plugged in.

## Test the API

```powershell
Invoke-RestMethod -Uri http://localhost:1337/status -Method Get
Invoke-RestMethod -Uri http://localhost:1337/spin   -Method Post
Invoke-RestMethod -Uri http://localhost:1337/flash  -Method Post
Invoke-RestMethod -Uri http://localhost:1337/stop   -Method Post
```

## Known limitation

If the beacon is physically unplugged while idle (no commands are being issued), the app will not detect the disconnect until the next command attempt fails. This is a v1 limitation. The reconnect logic works correctly after any failed command.
