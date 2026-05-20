# BlinkyBeacon Protocol Testing Guide

This tool sends raw HID reports to the FS22 USB beacon (VID 0x340D / PID 0x1710)
to discover undocumented protocol features.

## The Known Packets

Run `blinkybeacon-fuzzer.exe dump` to see these annotated.

```
       [0]   [1]   [2]   [3]   [4]   [5]   [6]   [7]   [8]   [9]
stop:  00    FF    00    00    64    00    32    9E    D7    0D
spin:  00    FF    01    66    C8    FF    AD    52    81    D6
flash: 00    FF    07    FF    50    FF    1C    8E    B0    B8
```

**What we know for certain:**
- Byte 0 is always `0x00` (HID report ID)
- Byte 1 is always `0xFF`
- Byte 2 changes across modes: `0x00`=stop, `0x01`=spin, `0x07`=flash
- The game sends these every 5 seconds to keep the beacon alive

**What we don't know:** whether bytes 3-9 are meaningful parameters
(speed, brightness, color, pattern) or just fixed checksum/padding values.

---

## Setup

1. Copy `blinkybeacon-fuzzer.exe` to your Windows machine
2. Plug in the beacon
3. Open a **normal Command Prompt** (not admin needed)
4. Confirm the device is found:
   ```
   blinkybeacon-fuzzer.exe list
   ```

---

## Test Series

Work through these in order. **Record what you observe** — use the
observation checklist at the end.

---

### Test 1 — Baseline confirmation

Confirm the three known sequences work as expected before changing anything.

```
blinkybeacon-fuzzer.exe replay spin
blinkybeacon-fuzzer.exe replay flash
blinkybeacon-fuzzer.exe replay stop
```

Expected: spin spins, flash flashes, stop turns off. If these fail, USB
access is blocked (try running as Administrator or check device drivers).

---

### Test 2 — Mode byte (byte 2): find intermediate modes

Values `0x02`–`0x06` are between spin and flash. Try each one from idle.

```
blinkybeacon-fuzzer.exe sweep 2 00 0F 1500
```

Hold time 1500ms gives you 1.5 seconds to observe each. Watch for:
- New flash patterns (e.g. double-flash, slow flash)
- New spin speeds
- Both light and motor active at once
- No visible change (dead values)
- Device ignoring the command entirely

**High-value targets:** `0x02`, `0x03`, `0x04`, `0x05`, `0x06`

If anything looks interesting, send it manually to confirm:
```
blinkybeacon-fuzzer.exe send 00 FF 02 66 C8 FF AD 52 81 D6
```

---

### Test 3 — Byte 3: speed or rate?

In spin `0x66` (102), in flash `0xFF` (255), in stop `0x00`.
This could be rotation speed, flash rate, or intensity.

Sweep from zero to max with the spin command as base:
```
blinkybeacon-fuzzer.exe sweep 3 00 FF 300
```

300ms per step. Observe whether the motor speed changes.
If it's speed-controlled: slower values = slower spin, higher = faster.

Narrow down the range once you find the threshold:
```
blinkybeacon-fuzzer.exe sweep 3 40 90 600
```

---

### Test 4 — Byte 4: brightness or duty cycle?

In stop `0x64` (100), in spin `0xC8` (200), in flash `0x50` (80).
Could control LED brightness or flash duty cycle.

```
blinkybeacon-fuzzer.exe sweep 4 00 FF 300
```

Watch the LED intensity. If brightness-controlled, lower values = dimmer.

---

### Test 5 — Byte 5: unknown (0x00 vs 0xFF)

Stop has `0x00`, spin and flash both have `0xFF`. Binary flag?

```
blinkybeacon-fuzzer.exe send 00 FF 01 66 C8 00 AD 52 81 D6
blinkybeacon-fuzzer.exe send 00 FF 01 66 C8 FF AD 52 81 D6
```

Compare — does changing byte 5 from `0xFF` to `0x00` affect anything?

---

### Test 6 — Bytes 6-9: checksum or padding?

These vary across commands and could be a checksum (meaning they'd be
ignored if wrong) or actual parameters.

**Test A — are they verified?**
Send a valid spin command with bytes 6-9 zeroed out:
```
blinkybeacon-fuzzer.exe send 00 FF 01 66 C8 FF 00 00 00 00
```
If the beacon still spins normally, these bytes are not a checksum.
If the command is silently ignored, they are probably validated.

**Test B — set them all to 0xFF:**
```
blinkybeacon-fuzzer.exe send 00 FF 01 66 C8 FF FF FF FF FF
```

**Test C — sweep byte 6 with everything else from spin:**
```
blinkybeacon-fuzzer.exe sweep 6 00 FF 200
```

---

### Test 7 — Report size: does the device accept longer/shorter reports?

The device might support a longer report format with more parameters.
This requires the `interactive` mode so you can send different-length payloads:
```
blinkybeacon-fuzzer.exe interactive
> send 00 FF 01 66 C8 FF AD 52 81 D6
> send 00 FF 01 66 C8 FF AD 52 81 D6 00 00
```

Note: the tool enforces 10 bytes in `send` mode. To try other lengths,
you'd need to modify the source and rebuild.

---

### Test 8 — Feature reports

Some HID devices use *feature reports* (a different channel than output reports)
for configuration. The tool doesn't currently send feature reports — if the
above tests yield nothing, this is worth adding to the code.

---

### Test 9 — Byte 2 extended: values above 0x07

```
blinkybeacon-fuzzer.exe sweep 2 07 FF 500
```

Long sweep — about 3 minutes. Run it and walk away.
Values above 0x07 might activate undocumented modes.

---

### Test 10 — Combined: speed + mode

If tests 2 and 3 find something, combine them. For example if `0x03`
is a new mode and `0x80` is half-speed:
```
blinkybeacon-fuzzer.exe send 00 FF 03 80 C8 FF AD 52 81 D6
```

---

## Observation Checklist

For each interesting value found, record:

| Byte | Value | LED behavior | Motor behavior | Notes |
|------|-------|-------------|---------------|-------|
| 2    | 0x01  | steady      | spinning      | baseline spin |
| 2    | 0x07  | flashing    | off           | baseline flash |
| ...  | ...   | ...         | ...           | ...   |

**LED observations:**
- [ ] Is it on / off / flashing / pulsing?
- [ ] Does brightness change with any byte?
- [ ] Does flash rate change?
- [ ] Any color change? (unlikely but possible on some revisions)

**Motor observations:**
- [ ] Is it spinning at all?
- [ ] Does speed change?
- [ ] Any direction change?
- [ ] Any stuttering/stepping pattern?

**Failure modes:**
- [ ] Beacon goes silent (command rejected — note what value triggered it)
- [ ] Beacon freezes (need to unplug/replug)
- [ ] Nothing observable changes

---

## Rebuilding After Changes

If you want to modify the tool (e.g. try different report lengths or add
feature report support), rebuild from WSL:

```bash
cd /home/clovisd/projects/blinkybeacon-fuzzer
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  /home/clovisd/go-sdk/bin/go build -o blinkybeacon-fuzzer.exe .
```

Then copy `blinkybeacon-fuzzer.exe` to Windows.
