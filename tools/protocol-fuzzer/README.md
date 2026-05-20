# BlinkyBeacon Protocol Fuzzer

A command-line tool for exploring the USB HID protocol of the FS22 beacon light.

Used to test whether undocumented modes, brightness, or speed control exist. Conclusion: the device supports exactly three modes (stop, spin, flash) with no additional parameters.

See [TESTING_GUIDE.md](TESTING_GUIDE.md) for the full test methodology and results.

## Usage

```
blinkybeacon-fuzzer.exe list
blinkybeacon-fuzzer.exe replay spin
blinkybeacon-fuzzer.exe send 00 FF 07 FF 50 FF 1C 8E B0 B8
blinkybeacon-fuzzer.exe sweep 2 00 0F 1500
blinkybeacon-fuzzer.exe interactive
```

## Build from source (WSL2 / Linux)

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -o blinkybeacon-fuzzer.exe .
```
