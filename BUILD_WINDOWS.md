# Building blinkybeacon-tray for Windows

The tray app uses `go-hid`, which needs CGO and the Windows HID libraries. There
are two ways to produce the `.exe`, and **cross-compiling from Linux is the one
that is actually used** for releases.

Go 1.25+ is required (`go.mod` says `go 1.25.0`).

## Option A — cross-compile from Linux (verified)

Install the mingw-w64 toolchain, then build:

```bash
sudo apt-get install -y gcc-mingw-w64-x86-64          # provides x86_64-w64-mingw32-gcc

GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -ldflags='-H windowsgui' -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

Verified 2026-08-19 on Ubuntu/WSL2 with Go 1.27.0 and
`x86_64-w64-mingw32-gcc (GCC) 13-win32`. Produces a ~16.9 MB binary:

```
$ file blinkybeacon-tray.exe
blinkybeacon-tray.exe: PE32+ executable (GUI) x86-64, for MS Windows, 22 sections
```

> Earlier revisions of this file said cross-compiling "requires mingw-w64 which
> is not set up in this repo's dev environment" and told you to build on Windows
> instead. That is no longer true, and it is why this project handed back source
> rather than a binary for longer than it needed to. Install the toolchain above
> and the cross-compile works first time.

Note this only applies to `./cmd/blinkybeacon-tray/`. The HID packages
(`pkg/fsbeacon`, `cmd/fsbeacon`) do **not** build natively on Linux without
`libudev-dev`; that is the Linux CGO layer and is unrelated to the Windows
cross-build, which supplies its own HID implementation via mingw.

## Option B — build natively on Windows

1. Install Go 1.25+ from https://go.dev/dl/
2. Install Git for Windows
3. Clone this repo

```powershell
git clone https://github.com/YOUR_USERNAME/blinkybeacon.git
cd blinkybeacon
go build -ldflags="-H windowsgui" -o blinkybeacon-tray.exe ./cmd/blinkybeacon-tray/
```

The `-H windowsgui` flag suppresses the console window so only the tray icon
appears. It matters in both options.

## Run

Double-click `blinkybeacon-tray.exe`. The USB beacon must be plugged in.

For the dashboard watcher (flash at draft end, spin during a pause), see
[BEACON-DASHBOARD.md](BEACON-DASHBOARD.md).

## Test the API

```powershell
Invoke-RestMethod -Uri http://localhost:1337/status -Method Get
Invoke-RestMethod -Uri http://localhost:1337/spin   -Method Post
Invoke-RestMethod -Uri http://localhost:1337/flash  -Method Post
Invoke-RestMethod -Uri http://localhost:1337/stop   -Method Post
```

## Testing

`go test ./cmd/blinkybeacon-tray/...` runs on Linux without any HID libraries —
the tray package keeps a local `Beacon` interface so the tests never pull in
CGO. `go test ./...` will fail on Linux in `pkg/fsbeacon` for the reason above;
that is expected.

## Notes

Unplug-while-idle **is** detected: the app starts the beacon's idle worker as
soon as it connects (`StartIdleWorker`, `cmd/blinkybeacon-tray/main.go`), so a
disconnect surfaces without waiting for a command to fail. This was fixed in
`feb5456` and `67b95e4`; earlier revisions of this file listed it as a known
limitation, which is out of date.
