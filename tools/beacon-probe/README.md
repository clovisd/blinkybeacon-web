# beacon-probe

A one-off command-line check of every Farming Simulator beacon light plugged
into this PC. It reports what the HID layer knows about each one (path,
serial number, manufacturer and product strings, interface and release
numbers, and the port key that would tell serial-less lights apart), and
with `-flash` it flashes each light in turn so you can see which is which.

It changes no setting. The only file it writes is `beacon-probe.txt` beside
the executable, which it appends to on every run.

## Run it

1. Plug in every beacon, and **quit the tray app first**. The tray re-sends
   its own command to each light every five seconds, which would fight the
   flash here; on macOS it also holds each light exclusively.
2. Double-click `beacon-probe.exe`, or run it from a terminal:

   ```
   beacon-probe.exe
   beacon-probe.exe -flash
   beacon-probe.exe -flash -seconds 3
   ```

3. Send back `beacon-probe.txt`.

## Build from source (WSL2 / Linux, for Windows)

From inside this directory:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -trimpath -ldflags='-buildid=' -o beacon-probe.exe .
```

The same line without `GOOS`, `GOARCH` and `CC` builds it on a Mac. This is
its own Go module, so it builds nowhere into the tray or the root module's
tests.
