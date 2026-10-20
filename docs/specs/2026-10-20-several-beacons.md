# Several beacons, each following one or more lines — the S16 design

**Status: DESIGN, for the owner's approval.** This document ships no code. Its one companion is
`tools/beacon-probe/`, a read-only check the owner runs once on real lights; its findings close
the unknowns named in §3 before the build starts. Stage row: `docs/ROADMAP-STAGES.md` S16.

What the owner chose on 2026-10-20 (MASTER-DEV `sessions/HANDOFF-blinkybeacon.md` §0d, answers
2, 7 and 8), and what this design builds on:

- **"Both":** several beacons on one PC, and each beacon follows one or more dashboard lines.
- **Companion buttons: "All by default, pick per button":** the module's existing Spin, Flash and
  Stop buttons keep driving every beacon at once; each button gains an optional choice of beacon,
  shipped in the module release that also carries the token (S17).
- **Hardware:** three or more beacons on the owner's desk to test with.
- Earlier wording the owner accepted: in "one light, several lines", *any line's draft end flashes
  it, any line's pause spins it, and the tray menu says which line*; with several beacons each has
  *its own line and light settings*.

The first half of this document (§1–§4) is written to be read and decided on. The second half
(§5–§16) is the builder's, and every claim about today's code cites the file and line it rests on.

---

## 1. What it looks like at the desk

Three lights are plugged into one PC. The tray app finds all three. On the settings page each one
has a row: a **name** you type ("Desk left", "Desk right", "Spare"), the **lines it follows**
(tick any of the dashboard's official lines, or none), and its own **light settings** (how long the
draft-end flash runs, whether a new lobby flashes and for how long, whose pauses spin it). An
**Identify** button beside each row flashes that light for two seconds so you can see which
physical light the row is.

- A beacon that follows **one line** behaves exactly as the whole app does today.
- A beacon that follows **several lines** lights for any of them: a pause on any of its lines spins
  it, a draft ending on any of its lines flashes it. When its lines disagree, the busier state
  wins, in the order the app already uses for one line: **spinning beats flashing beats dark**.
  A pause that lifts while a flash still has time left hands the light back to the flash for what
  is left, as the app does today for one line.
- A beacon that follows **no line** is yours to drive by hand from the tray menu or the HTTP
  routes, exactly like an unbound tray today.
- A light you plug in for the first time shows up as a new beacon that follows nothing, so it never
  lights for a line nobody assigned it to.

The tray menu gets one entry per beacon, with its state, its lines, and its own Spin, Flash, Stop
and Identify. The top-level Spin, Flash and Stop stay and drive every beacon, as the Companion
module's buttons do. The tray icon shows the busiest light.

The HTTP routes the Companion module uses keep their paths and replies. `POST /spin`, `/flash`
and `/stop` with nothing more drive every beacon; with `?beacon=b1` (the short id the tray gives
each beacon, or its name) they drive one. `GET
/status` keeps answering `{"state", "connected"}`, now for the beacons as a whole. One new route,
`GET /beacons`, lists them with ids, names and states, so the module's beacon picker (S17) and
any script can find out what is plugged in.

A config file from `dashboard-v0.8.0` loads without the operator touching it, as one beacon
following one line with today's settings.

## 2. Decisions for the owner

Five, each with the recommendation first. Everything else in this document is settled from the
code, the libraries or the owner's words, and says so where it is.

**D1 · When a light is moved to another USB port (if these lights carry no serial number).**
If the probe (§3) finds no serial numbers, a beacon is known only by the port it is in, like a
chair at a desk. Moving the light to another port makes it look like a new, unassigned beacon,
and the old row shows "unplugged".
- **Recommended: the port is the seat.** The row keeps its name, id, lines and settings; the
  moved light appears as a new beacon that follows nothing; on the settings page the new row
  offers "This light is Desk left, moved" in one click, after which Desk left is in its new port
  and nothing else has changed. Predictable, and never wrong.
- Alternative: the tray guesses. When exactly one row is unplugged and exactly one new light
  appears, it re-binds the row to the new light by itself. Convenient, and wrong in the one case
  it cannot see: Desk right is unplugged in a drawer, a borrowed light goes into a fresh port, and
  the guess hands the borrowed light Desk right's lines, which is exactly what §10.4 promises never
  happens to a light nobody assigned.
- A swap of two lights between two ports behaves the same under both options: in port mode each
  port's row matches whatever light sits in it, so nothing is unplugged, nothing is new, and there
  is nothing to guess. The two lights trade names; Identify tells them apart.
- If the probe finds distinct serial numbers, this decision is moot: the light carries its
  identity with it, and a port move changes nothing.

**D2 · What `GET /status` says for several beacons.** The Companion module reads this route every
two seconds and colours its buttons from it: "Beacon connected", "active (spin or flash)",
"spinning", "flashing", "idle" (the module's `src/feedbacks.ts:6-60`).
- **Recommended: the busiest light.** `connected` is true when **any** beacon is plugged in;
  `state` is `spin` if any beacon spins, else `flash` if any flashes, else `idle`. The module's
  "Beacon connected" stays green while a spare is unplugged, and "active" means a light is on
  somewhere. Table in §9.2.
- Alternative: `connected` only when **every** configured beacon is plugged in. Honest about the
  spare, but the module's feedback goes red for a light nobody is using, and a Stream Deck button
  that only ever drives Desk left reads "disconnected" because Desk right is in a drawer.

**D3 · Whether `GET /beacons` is open, like `/status`, or behind the token (S18).** The owner ruled
that `/status` stays open when the token lands (2026-10-20, answer 4).
- **Recommended: open, exactly like `/status`.** It is a read of the same kind, it reveals the
  names the operator typed and the states `/status` already shows, and the Companion module's
  picker needs it before any button has been configured. The three control routes with
  `?beacon=` get the same token check as without.
- Alternative: behind the token with `/spin` and the others. Then the picker in the module cannot
  list beacons until the token is pasted into the module's settings, which is the same release, so
  it is workable but one more thing to get right at setup.

**D4 · The tray menu: one submenu per beacon, or a flat list.**
- **Recommended: a submenu per beacon** at the menu's root ("Desk left — Spinning · Line A,
  Line C ▸" opening to its status, its lines, Spin, Flash, Stop, Identify), under the top-level
  Spin, Flash and Stop that drive every beacon. The menu library supports submenus and can show or
  hide items, and a re-shown item returns to its own place (its position follows the order the
  items were created in, §11.1), so the tray pre-creates eight hidden beacon submenus **before**
  the separator and Quit at start and shows them as beacons appear, each in its own slot. Eight
  beacons is the menu's limit, not the app's.
- Alternative: a flat list, "Desk left: Spin / Desk left: Flash / …" for every beacon. No submenu
  clicks, but fifteen rows for three beacons, and the same eight-slot pre-creation and rule.

**D5 · Keep the old one-beacon keys in the config file, mirrored from the first beacon.** The new
file holds a `beacons` list (§10.1). The old top-level keys (`line_number`, `flash_seconds`,
`lobby_flash`, `lobby_flash_seconds`, `pause_side`) can either stay, written from the first beacon
on every save, or go.
- **Recommended: keep them, mirrored.** A `dashboard-v0.8.0` build pointed at the new file then
  still runs the first beacon on its line, so going back a release after trying this one keeps
  that one light working. The real cost of a downgrade stays: the older build's Save writes its
  own keys only (`settings_handler.go:311-322`), so the first time Save is pressed there the
  `beacons` list is gone, and the other beacons' names, lines and settings with it; they come
  back only from a copy of the file. The mirror buys a working first beacon, not a round trip.
  The price is five redundant keys in a file few people open.
- Alternative: drop them. A cleaner file; a downgrade reads `line_number` as missing and binds the
  beacon to line 1.

## 3. Unknowns, and the probe step that settles each

| Unknown | Why it matters | Settled by |
|---|---|---|
| **Does this light report a USB serial number, and is it distinct per unit?** | Decides what a beacon's identity is (§5.3): a serial travels with the light; without one the port is the identity, and D1 applies. A serial **duplicated** across units is worse than none: Windows builds the device's instance id from the serial the light reports (`hid_windows.c:508-535`), so two such lights get the same path segment whatever ports they are in, and neither the serial nor the port can tell them apart. | Probe step 2: the `summary` block says `DISTINCT`, `NONE`, `MIXED` or `DUPLICATED`. `DISTINCT` → serial mode; `NONE` → port mode; **`MIXED` or `DUPLICATED` → stop**: the build does not pick a mode, and the probe file comes back to this design (§5.3, §14.1). |
| **Is one light one HID path?** A device with several top-level collections shows one path per collection on Windows, all sharing the instance segment and differing in usage page and usage. | Two paths for one light must not become two beacons, and the tray must open the one that takes the command. | Probe step 2: two `beacon N` entries with the same `port key` and **different** `usage page / usage` lines are one light with two collections; the same `port key` **and** the same usage pair are two lights Windows cannot tell apart (a duplicated serial: stop, as above). Step 4 confirms it: one light flashing twice in a row, or its second path saying `write FAILED`. If a light shows two paths, the match key is the shared instance segment, and the build opens, per key, the path whose write the owner **saw flash the light** in step 4 (step 4 asks which light answered each `beacon N`); if both paths flash it, the first in enumeration order. T0 records the usage pair to prefer. |
| **Does the Windows path stay the same across a reboot, and change across a port move?** | The port-as-seat rule (§5.3) rests on Windows building a serial-less device's instance id from the hub and port. Read from Windows' own documentation, not measured here. | Probe steps 2 and 3: compare `port key` before and after the move; the owner may also run step 2 again after a reboot. |
| **Can several beacons with the same vendor and product id be held open at once, and does writing to one path light only that light?** | Upstream's library header says it *"cannot differentiate between multiple beacons … due to a hardware limitation"* (`pkg/fsbeacon/usbhid.go:3`). Everything in `hidapi` says paths are per device interface and opening by path is exact (§5.1); the comment most likely means "no serial number", but the lights have to show it. | Probe step 4: `-flash` opens every beacon and holds them open together, then flashes each in turn naming it. One light at a time flashing, in order, settles it. If more than one light flashes at once, **stop and report**: that is the kickoff's first halt condition. |
| **macOS** — whether the location id is stable across replug and reboot, and whether several lights open cleanly through IOKit. | §13. No Mac with a light exists (`docs/PUNCHLIST.md:30`, owner: "not now"). | Unmeasurable until a Mac and a light meet. The probe builds on macOS (its `extra_darwin.go` prints the location id); that file compiled nowhere in this session, so it is unverified. |
| **macOS: does opening the light trip a permission prompt?** Input Monitoring is asked for keyboards and mice; whether this HID device counts is unknown, and the app has never been run against a light on a Mac. | A prompt at first start is a setup step the guide would have to name; a silent refusal is a dark light. | The first Mac run, with the punchlist's macOS item (`docs/PUNCHLIST.md:30`). The probe does not settle it. |
| **How fast `hid.Enumerate` is on Windows with a dozen HID devices present.** | Sets how often the tray can look for newly plugged lights (§12). `hidapi` opens every HID interface on the system read-only to read its vendor id (`hid_windows.c:806-864`). | The design keeps today's 10-second retry as the look-up cadence, so nothing rests on it. A later tuning can measure it with the probe's timing. |

## 4. The owner's hands-on steps, in short

The full step-by-step is §14.1 (the probe) and §14.3 (the built feature on three lights). The
probe first:

1. Plug in every beacon, and **quit the tray first**: the tray re-sends its own command to every
   light it holds every five seconds, which would fight the probe's flash (and on a Mac it holds
   each light exclusively, so the probe could not open them at all).
2. Run `beacon-probe.exe` (double-click is fine). Send back the `beacon-probe.txt` it writes
   beside itself. If its summary says `DUPLICATED` or `MIXED`, or two beacons show the same
   `port key` with the same usage, send the file and stop there: the design has to be revisited
   before anything is built.
3. Move one beacon to another USB port and run it again. Same file, appended.
4. Run `beacon-probe.exe -flash` to see which light is which.

The exe is `beacon-probe.exe`, sha256
`5302d50d7a434c86346e76fe2d6d4345607ade27a7acf83181fa34078356b0e4`, built from commit
`fcc9126b7dae47a307de460265a69de9a0a4ee1a` with the release build line minus `-H windowsgui`
(§14.1 has the line). The same line run twice gave the same hash.

---

## 5. Telling beacons apart

### 5.1 What the library gives you

`github.com/sstallion/go-hid` v0.14.1 (`go.mod:9`) wraps `hidapi` 0.14.0 (`hidapi.h:50-60`).
Per attached device it reports (`hid.go:104-116`): `Path`, `VendorID`, `ProductID`, `SerialNbr`,
`ReleaseNbr`, `MfrStr`, `ProductStr`, `UsagePage`, `Usage`, `InterfaceNbr`, `BusType`.

Three ways to open:
- `hid.OpenFirst(vid, pid)` (`hid.go:178-184`) — what the tray uses today
  (`pkg/fsbeacon/usbhid.go:56`). On both platforms it enumerates and opens the **first** match
  (`hid_windows.c:880-921`, `hid_darwin.c:808-845`). With three lights plugged in it opens one of
  them, and which one is the enumeration order.
- `hid.Open(vid, pid, serial)` (`hid.go:164-173`) — the first match whose serial string equals
  the one given. Useless when serials are empty.
- `hid.OpenPath(path)` (`hid.go:188-197`) — exactly the device the path names. **This is the one
  the tray moves to**, after `hid.Enumerate(vid, pid, fn)` (`hid.go:143-154`) has listed the paths.

Opening is shared on Windows (`CreateFileW` with `FILE_SHARE_READ|FILE_SHARE_WRITE`,
`hid_windows.c:334`), so a second program can open a light the tray holds; it is **exclusive on
macOS** by default (`hid_init` calls `hid_darwin_set_open_exclusive(1)`, `hid_darwin.c:470`;
`IOHIDDeviceOpen` with `kIOHIDOptionsTypeSeizeDevice`). That is why the probe says "quit the
tray first": on Windows the reason is the tray's five-second keepalive rewriting the light's
state (`pkg/fsbeacon/usbhid.go:70-77`), on macOS the open itself fails.

### 5.2 What the path is, per platform

**Windows.** The path is the device-interface path, verbatim (`hid_windows.c:713`), of the shape

```
\\?\hid#vid_340d&pid_1710#6&2a1b3c4d&0&0000#{4d1e55b2-f16f-11cf-88cb-001111000030}
```

The segment between the second and third `#` is the **device instance id**. Windows builds it
from the device's USB serial number when it has one, and otherwise from the parent hub's devnode
and the port number, which is the `6&2a1b3c4d&0&0000` shape above (the `&` marks a bus-generated
id; `hidapi` reads it the same way, refusing to call such a segment a serial at
`hid_windows.c:528-531`). Consequences, from Windows' device-installation documentation
("Instance IDs", "USB device identifiers"), **to be confirmed by probe step 3**:

| Event | Serial present | No serial |
|---|---|---|
| Reboot, same port | same path | same path (the devnode is persisted) |
| Move to another port | same path | **new path** (a new devnode for that port) |
| Another unit of the same model in the same port | its own path | **the same path** — Windows cannot tell them apart, so neither can we |

**macOS.** The path is `DevSrvsID:<IORegistryEntryID>` (`hid_darwin.c:572-595`), a 64-bit id the
kernel hands the registry entry when the device attaches. It is **not** stable: a replug, even into
the same port, gets a new entry, and so does a reboot. It identifies a light only while it stays
plugged in. The stable handle on a Mac is the **location id** (`kIOHIDLocationIDKey`), which
encodes the USB controller and port path; it is readable only from an open device
(`hid_darwin.c:1483-1492`, exposed as `(*hid.Device).GetLocationID()` at `hid_darwin.go:33`).
Serial numbers come from `kIOHIDSerialNumberKey` (`hid_darwin.c:410-412`).

### 5.3 The identity rule

A beacon's **match key** is a string with a prefix saying what kind it is:

- `sn:<serial>` when the light reports a serial number, **and** the probe showed the owner's
  units report distinct ones. The light keeps its identity in any port, on any PC.
- `port:<key>` otherwise. On Windows the key is the instance-id segment of the path, lower-cased
  (`6&2a1b3c4d&0&0000`); on macOS it is the location id in hex (`0x14200000`). The port is the
  seat: whatever light sits in it is that beacon (D1).

**The build picks one mode from the probe's file and fixes it as a constant** (task T0, §15):
`DISTINCT` → `sn`; `NONE` → `port`. **`MIXED` or `DUPLICATED` is a stop**, and so is a file in
which two entries share a `port key` with the same usage page and usage (§3): the build does not
pick a mode, and the master brings the probe file back to this design, because a light Windows
cannot tell apart from its twin has no identity this design can give it (`hid_windows.c:508-535`:
the instance id is built from the serial the light reports, so a duplicated serial collapses two
ports onto one path segment). Mixing the two modes at run time was considered and refused: a key
that depends on what else is plugged in changes under the operator's feet. One run-time guard
stays, for a twin bought later or a second path on a live key: a newly enumerated path whose key
is already held by a **live** handle is reported in the tray as "cannot be told apart" and is not
opened; it is tried again at every enumerate. "Not live" means marked unplugged, or its last
keepalive attempt failed; there is no age test. A key held by a handle that is not live is a
**replug**, and the new path takes it over (§12, item 1). Loud, never wrong, and never stuck
behind a stale handle for longer than one keepalive period and one enumerate, about 15 s.

**Two ids per beacon.** The string above is the **match key**, internal: it is stored in the
record (`key`) and used only to recognise a light; it never travels in a URL or a form. Each
record also gets a **public id**: `b` and a number (`b1`, `b2`, …), assigned by the tray from a
counter in the config file (`beacon_seq`, §10.1) and **never reused** after a Forget. The public
id is what travels in `?beacon=`, `GET /beacons`, the settings form and the Companion picker; it
is lowercase letters and digits only, so it needs no encoding anywhere.

### 5.4 Naming, and finding out which light is which

- The operator names a beacon on the settings page (§11.2). Names are free text, trimmed, 1–40
  characters, unique among the configured beacons (the save refuses a duplicate by name), and
  shown everywhere the beacon is mentioned: tray, settings, logs, `GET /beacons`. A name that
  would read as a public id (`b` followed only by digits, in any case, after trimming) is refused
  at save, so a `?beacon=` value is never ambiguous. A beacon that has not been named shows as
  `New beacon` with its public id beside it (`b3`).
- **Identify** flashes one beacon for two seconds and then returns it to whatever it was doing.
  It is a per-beacon **override with a deadline** (§7.3), the same shape as the watcher's own
  flash deadline, so a beacon spinning for a pause flashes for two seconds and goes back to
  spinning. It sits in each beacon's tray submenu and beside each settings row, and rides the
  settings form's own POST with its CSRF token (§11.2), so no new write route is added.
- **How this meets S9.** S9 (`docs/ROADMAP-STAGES.md:63`) wants a test button in the tray's
  settings giving *"a short double flash"*, per beacon and for all. Identify and test are one
  action with two reasons: "which light is this" and "does this light work". S16 ships the
  function `identify(beacon)` as a plain two-second flash; S9 changes that one function's pattern
  to the double flash and adds the all-beacons button beside the rows. Nothing else moves.

## 6. The model

```
Beacon
  id        string        the public id, "b1", "b2", … (§5.3): assigned from beacon_seq, never reused
  key       string        the match key, "sn:…" or "port:…" (§5.3); "" for a record not yet matched to hardware (§10.2)
  name      string        the operator's; "" until named ("Beacon 1" for the record migrated from an old file, §10.2)
  lines     []int         zero or more dashboard line numbers (the N in /api/v0/lines/N)
  light     LightSettings flash_seconds · lobby_flash · lobby_flash_seconds · pause_side
```

Global, unchanged: `addr`, `port`, `dashboard_url`, `token`, `poll_interval_ms`. One dashboard,
one token, one cadence per tray.

**Light settings belong to the beacon, not to each line it follows.** The owner's words give it
("its own line and light settings"), and the code agrees: the three settings answer "how does
*this* light announce things", which is a property of where the light sits and who is watching it,
not of which game is on. Per line would mean a beacon following two lines carries two draft-flash
lengths and needs a tiebreak when both drafts end in the same minute; per beacon has no such
case. It also maps the `dashboard-v0.8.0` file onto the new one with no loss: its three settings
(`config.go:31-43`) become the first beacon's. The one setting that could be argued per line is
`pause_side` (a desk's team is Radiant on one line and Dire on another). With several lines per
beacon "this desk's team" is not one side anyway; a beacon that must spin for Radiant on line A
and Dire on line C is two beacons, or a later per-line override. Settled: per beacon.

**A beacon with no lines is driven by hand only.** Its tray row says `Unbound`, exactly the word
the tray uses today for the whole app (`watcher.go:583-586`), and the watcher never touches it.
The hand controls stay live on every beacon; on one that follows lines, the watcher corrects a hand
command on its next tick, which is today's rule (`BEACON-DASHBOARD.md` §7, "The manual controls
still work").

## 7. One beacon, several lines

### 7.1 Where today's state machine stays, and what is added around it

Today's `Watcher` (`watcher.go:245-265`) is one line's edge detector **and** one light's policy in
one struct: it remembers the previous poll's draft and game state, keeps the two once-per-match
guards, arms a flash deadline from the light settings it was built with (`WatcherSettings`,
`watcher.go:274-281`), and answers `Decide(now, lineState) → (StateValue, WatchStatus)`
(`watcher.go:325-381`). Its precedence is already the rule this design needs: a pause outranks the
flash, the flash outranks dark (`watcher.go:371-380`; `BEACON-DASHBOARD.md:351`).

The design keeps that state machine **unchanged** and runs **one `Watcher` per (beacon, line)
pair**. A beacon following lines A and C has two watchers; two beacons following line A have one
each. The guards are therefore per line *within* a beacon, which is what the kickoff asks ("each
line's once-per-match guards stay per line"): two beacons on one line each flash once for that
match, because they are two lights. The light settings in each watcher are its beacon's, so two
beacons on one line can flash for different lengths.

The alternative, splitting `Watcher` into a per-line detector emitting events and a per-beacon
policy, is cleaner on paper and a rewrite of the most-tested file in the tray
(`watcher_test.go`, 1,328 lines; `watch_loop_test.go`, 1,574 lines). Refused for this stage; the
pair-watcher reuses every `Decide` test as it is (what changes in the loop tests is in §14.2).

### 7.2 The rule when a beacon's lines disagree

Per tick each pair answers one of `idle`, `flash`, `spin`. The beacon shows the **highest-ranked
answer among its pairs**, ranked `spin > flash > idle`. Written out as cases:

| # | Line A says | Line B says | The beacon shows | Why |
|---|---|---|---|---|
| 1 | spin (paused) | idle | spin | a pause outranks everything, as for one line today |
| 2 | spin (paused) | flash (draft just ended) | spin | the pause outranks the flash (`BEACON-DASHBOARD.md:351`); B's flash deadline keeps running inside B's watcher |
| 3 | flash | idle | flash | |
| 4 | flash (ends at T1) | flash (ends at T2 > T1) | flash until T2 | each pair keeps its own deadline; the light flashes while any is live |
| 5 | spin | spin | spin | two pauses; the light stops when **both** lift |
| 6 | flash just ended | spin (still paused) | spin | B's pause was there all along, outranked nothing, and now shows |
| 7 | spin lifts | flash (B's deadline still ahead) | flash | B's deadline was armed when its draft ended and is honoured for what is left of it |
| 8 | blind (unreachable, stopped, quiet, 404) → idle | spin | spin | a blind line contributes dark and **never darkens another line's pause** |
| 9 | idle | idle | idle | |
| 10 | (no lines) | | whatever the hand set | the watcher does not touch an unbound beacon |

Properties the build's test pins (task T4, numeric, plan-gate rule 1): the combine is the maximum
under a total order, so it is commutative, associative, idempotent, and monotone in each input;
the result for N pairs never depends on their order; and over every multiset of up to six answers
the result equals the rank maximum. The sweep covers all 3^6 combinations rather than the ten
rows above, which are the pinned values.

Once-per-match guards: in row 2, line B's draft-end flash is **spent** (its guard set) even though
the light showed spin, and B's deadline keeps running underneath the pause. If the pause lifts
before that deadline, the flash shows for what is left of it (row 7); if the deadline has passed,
the light goes dark. That is what today's `Decide` does for one line (`watcher.go:371-380`: the
pause is checked first, then `flashUntil`, which a pause never clears), and the precedence the S79
design set ("paused > flash-window > idle", its §5.2). The user guide's sentence that the light
"does not go back to flashing when the pause lifts" (`BEACON-DASHBOARD.md:351-354`) overstates
the code; T9 corrects it and T4 pins the resume. Nothing about the guards changes.

### 7.3 The wanted state of a beacon

```
wanted(beacon, now) =
    if identifyUntil(beacon) > now           → flash            (§5.4; a 2 s override)
    else if beacon follows no line           → the hand state   (last Spin/Flash/Stop)
    else                                     → max over its pairs' answers this tick
```

Applied with today's idempotence rule: a USB command is issued only when the beacon's current
state differs from the wanted one (`applyState`, `watcher.go:552-573`), per beacon.

## 8. Polling

### 8.1 Shape

One loop, one tick. Each tick:

1. Read the config (as today, every tick: `watch_loop.go:35`). Compute the set of **distinct
   lines** followed by any configured beacon, plugged in or not (§10.3: an unplugged beacon's
   watchers keep running, as today's does with no beacon connected).
2. Fetch each distinct line **once**, concurrently, each fetch bounded by today's `pollTimeout`
   of 1.5 s (`watch_loop.go:17`). Wait for all. One `time.Now()` for the tick.
3. Feed each line's result (or its failure) to every (beacon, line) watcher that follows it;
   compute each beacon's wanted state (§7.3); apply (§7.3).
4. Record per-line watch status and label (as `SetWatchStatus`/`SetWatchLabel` do today,
   `watch_loop.go:114-128`), now keyed by line.
5. Sleep `pollDelay(interval, status)` (`watcher.go:143-147`).

What carries over unchanged: the cadence `poll_interval_ms` (500–10,000 ms, default 2,000;
`config.go:98-102`), the rebuild-on-settings-change rules (`watch_loop.go:72-94`, now per pair),
the 30-second backoff after a refused token (`watcher.go:139`). The token is one for the whole
tray, so a 401 on any line backs off the whole loop; a 404 (`no such line`) is per line and
leaves the others at cadence.

Concurrent fetches rather than one after another: with one line a hung dashboard costs one
`pollTimeout` per tick; serial fetches would cost one per line, so four lines could stretch a tick
to six seconds. Concurrent, a tick is bounded by one timeout whatever the line count.

**A throttled line.** The dashboard shipped no rate limit of its own in v0, and the Cloudflare
front in front of it can throttle (S79 §4.3); today's loop treats every answer that is not a 401
or a 404 as `unreachable` at full cadence (`watcher.go:164-175`). The loop gains one case, **per
line**: an HTTP **429** backs that line off until the moment `Retry-After` names (a number of
seconds or an HTTP date), **capped at 30 s**, and 10 s when the header is absent or unreadable.
Meanwhile that line's pairs see a failed poll (blind, dark, as any failed poll), its status is
`throttled`, and the other lines keep their cadence. 5xx and unreachable stay exactly as today:
`lost`, full cadence. The tray's words for it: `▲ Dashboard: slowed by the dashboard (line 2,
retry in 12s)`; `GET /beacons` reports `throttled`.

### 8.2 Requests per second, at the worst realistic desk

Requests depend on **distinct lines**, not beacons. Three beacons on one line are one request per
tick.

| Setup | Lines | Interval | Requests/s from this PC |
|---|---|---|---|
| The owner's desk: 3 beacons, 2 lines between them | 2 | 2,000 ms (default) | 1.0 |
| Same, at the fastest setting | 2 | 500 ms | 4.0 |
| Every official line of a busy event followed by some beacon | 8 | 2,000 ms | 4.0 |
| Same, fastest setting | 8 | 500 ms | **16.0** |

Plus the settings page's line list, at most once per five seconds while the page is open
(`lines.go:131`). The only load figures that exist were measured **before `/api/v0` existed, on
the dashboard's page route** `GET /line/1/state` (a 4.6 KB viewer-tier body): four pollers at
2 Hz for 10 s, 84 polls, **0 errors**, p95 8.3 ms (DOTA-DASHBOARD
`docs/superpowers/specs/2026-08-12-s79-beacon-companion-design.md` §3.2). The `/api/v0`
projection that then shipped is a ~200-byte cache read with no disk on the request path (S79
§4.2), so those figures bound its cost from above rather than measure it; on that basis S79 §4.3
sized "ten companions ≈ 20 req/s" as negligible and shipped **no rate limiting in v0**, noting
that Cloudflare can throttle abuse. The arithmetic above stands: the worst row, 16 req/s, sits
under that sizing, and the realistic rows are a tenth of it. The fan-out does land its requests
together at each tick, a burst of up to eight `/ping`-class reads. **No dashboard change is needed
for load**, §16 asks for none, and §8.1's handling of a 429 is what the tray does if the front
ever disagrees.

## 9. The HTTP API

### 9.1 The three control routes

Paths and replies unchanged (`server.go:25-28`, `:76-131`). What is added is an **optional
`beacon` query parameter**:

```
POST /spin                      drives every connected beacon
POST /spin?beacon=<id>          drives that one
POST /spin?beacon=<id>&beacon=<id>   drives those
```

`<id>` is the beacon's **public id** (`b1`, `b2`, …, §5.3) **or its exact name**; the id is tried
first, and a name can never look like an id because the save refuses one (§5.4). Every value is
**percent-encoded like any query value**: `?beacon=b1` needs nothing, `?beacon=Desk%20left` is
the name `Desk left`. The Companion module builds its URLs by template (`src/index.ts:44`), so
S17's picker sends the public id, and a typed name goes through `encodeURIComponent`. The handler
reads the values with `r.ParseForm()` + `r.Form["beacon"]`, which Go fills from the query string
and, for a POST with a form body, from the body too; the documented way is the query string.

Why a query parameter and not a form field: the Companion module posts with **no body and no
content type** (`src/index.ts:44`, `fetch(url, {method: 'POST'})`), so a form field would make
every caller grow a body and a header, while `?beacon=` is one more token on the URL the module
already builds; `curl -X POST http://127.0.0.1:1337/spin?beacon=b1` (or
`"…/spin?beacon=Desk%20left"` by name) is a one-liner for a script; and the token (S18) will ride
a header, leaving the query string to say *which*.

Replies:

| Call | Reply |
|---|---|
| no `beacon`, at least one beacon connected | `200 {"state", "connected"}` for the beacons as a whole, after the command (the §9.2 aggregate). A beacon whose command fails is dropped for reconnection, as today (`server.go:86-89`), and the rest go on; 200 because the aggregate is true of what remains. If **every** connected beacon's command fails: `503 {"error": "beacon disconnected"}`, today's text (`server.go:88`), and all are dropped for reconnection. |
| no `beacon`, none connected | `503 {"error": "beacon not connected"}`, today's text (`server.go:83`) |
| `beacon=<id>`, known and connected | `200 {"state", "connected"}` **about that beacon**; if its command fails, `503 {"error": "beacon disconnected"}` and it is dropped for reconnection |
| `beacon=<id>`, known and unplugged | `503 {"error": "beacon not connected"}` |
| `beacon=<id>`, unknown (neither a public id nor a name) | `404 {"error": "no such beacon"}`; nothing is driven |
| `beacon=` present but empty or whitespace | `400 {"error": "beacon must name a beacon"}`; nothing is driven |
| several values, any one unknown or empty | the 404 or 400 above, before anything is driven |
| several values, all known, some unplugged and some connected | the connected ones are driven; `200 {"state", "connected"}` about the named set (the §9.2 aggregate over those beacons, so `connected` is true) |
| several values, all known, all unplugged | `503 {"error": "beacon not connected"}` |
| several values, all known and connected, every command fails | `503 {"error": "beacon disconnected"}`; all dropped for reconnection |
| the same value twice | driven once |
| wrong method | `405`, as today |

The no-parameter rows are today's replies exactly, status and body. The six tests in
`server_test.go` keep every assertion; their one setup line changes with the registry (§14.2 says
how), so they pass changed, not unchanged.

### 9.2 `GET /status`

Keeps exactly `{"state": …, "connected": …}` (`server.go:17-20`, `:67-74`) and ignores any
query parameter. Across several beacons (D2):

| Beacons | `connected` | `state` |
|---|---|---|
| none plugged in (none configured, or all unplugged) | `false` | `idle` |
| at least one plugged in | `true` | `spin` if any connected beacon spins; else `flash` if any flashes; else `idle` |

Only **connected** beacons vote on `state`; an unplugged beacon's last state is not a light anyone
can see. The same aggregate colours the tray icon (§11.1). It is the same rank maximum as §7.2,
so task T3's property test covers both.

### 9.3 `GET /beacons` — new

```json
{
  "v": 1,
  "beacons": [
    {"id": "b1", "name": "Desk left",  "connected": true,  "state": "spin", "lines": [1, 3], "watch": "ok"},
    {"id": "b2", "name": "Desk right", "connected": false, "state": "idle", "lines": [2],    "watch": "ok"},
    {"id": "b3", "name": "",           "connected": true,  "state": "idle", "lines": [],     "watch": "off"}
  ]
}
```

- `v` is the shape's version, `1`. `beacons` is in config order, then newly seen unnamed beacons
  in the order they were found; it includes configured beacons that are unplugged
  (`connected: false`) so a picker can offer a beacon that is temporarily off.
- `id` is the **public id** (§5.3); the match key is internal and not published. `name` is as
  §5.4 (`""` until the operator names it). `state` is the beacon's own, one of
  `idle`/`spin`/`flash`. `lines` is what it follows. `watch` is `ok` when every line it follows is
  `ok`, otherwise the status of its first line (in `lines` order) that is not `ok`, in the tray's
  own words (`watcher.go:95-109`: `off`, `no-token`, `ok`, `lost`, `rejected`, `no-line`,
  `stopped`, `no-game-yet`, `quiet`, plus `throttled` from §8.1); `off` for a beacon with no lines.
- `GET` only; `405` otherwise. JSON, `Cache-Control: no-store`.
- **S18:** open, like `/status` (D3). If the owner picks the alternative, the check is the same
  one the control routes get, nothing route-specific.

The Companion module's picker (S17) reads `GET /beacons` to populate a dropdown of names and
sends the chosen `id` as `?beacon=`. A button with no choice sends nothing, which drives every
beacon, as today.

## 10. The config file

### 10.1 The new shape

```json
{
  "addr": "127.0.0.1",
  "port": 1337,
  "dashboard_url": "https://dashboard.example.com",
  "token": "the-token-you-were-given",
  "poll_interval_ms": 2000,
  "beacon_seq": 3,
  "beacons": [
    {
      "id": "b1",
      "key": "port:6&2a1b3c4d&0&0001",
      "name": "Desk left",
      "lines": [1, 3],
      "flash_seconds": 15,
      "lobby_flash": false,
      "lobby_flash_seconds": 10,
      "pause_side": "both"
    }
  ],

  "line_number": 1,
  "flash_seconds": 15,
  "lobby_flash": false,
  "lobby_flash_seconds": 10,
  "pause_side": "both"
}
```

The last five keys are the mirror of the first beacon (D5): written on every save from
`beacons[0]` (its first line, or `1` if it follows none), read by a `dashboard-v0.8.0` build and
ignored by this one whenever `beacons` is present. `beacon_seq` is the public-id counter (§5.3):
the next beacon gets `b<beacon_seq+1>`, and it only ever goes up, so a forgotten `b2` is never
handed to another light. `id` is the public id, `key` the match key.

The ids are loaded with forgiveness of their own, because this file is hand-editable: on load,
`beacon_seq` becomes `max(beacon_seq, the highest number among the records' ids)`, so a file whose
counter was edited down, or written by hand without one, cannot hand out an id that is already
in use; and a record whose `id` is missing, malformed (anything but `b` and digits, or digits
with a leading zero such as `b01`) or a duplicate of an earlier record's is **re-issued** the
next id from the counter, with a log line naming the old value and the new. T2 tests both.

Per-beacon values load with the same forgiveness as today's top-level ones (`config.go:151-167`):
a missing or out-of-range number lands on the documented default; an unknown `pause_side` loads
as `both`; a line number under 1 is dropped from `lines`; duplicates in `lines` are collapsed. The
settings page stays stricter (`settings_handler.go:260-266`). The file stays owner-only and written
atomically (`config.go:186-227`), unchanged.

### 10.2 Loading a `dashboard-v0.8.0` file

A v0.8.0 file (`BEACON-DASHBOARD.md` §7's sample) has no `beacons` key. Load rule:

1. Parse as today. If `beacons` is absent or empty, **synthesise one record** from the top-level
   keys: `id: "b1"` (the counter starts at 1), `key: ""`, `name: "Beacon 1"`,
   `lines: [line_number]`, light settings from the four top-level keys. Nothing is written yet;
   loading stays a pure read.
2. A record with `key: ""` is **unmatched**: it binds to the first connected beacon that no other
   record claims, in enumeration order, the moment the device manager (§12) sees one. On binding,
   the record takes that beacon's match key and the config is saved through the one writer
   (§10.5), so the next start matches by key. With one light plugged in this is today's behaviour
   exactly: the first (only) beacon found is driven by the configured line with the configured
   settings, and the HTTP routes, tray and settings page read as they do today.
3. With several lights plugged in at that first start, the unmatched record binds to the first one
   enumerated and the others appear as new beacons following nothing (§10.4). The operator sorts
   it on the settings page, with Identify to tell them apart; the settings row shows each
   beacon's public id and match key, and `Beacon 1` (`b1`) is whichever light was enumerated
   first.

The build's loader test (task T2; a parser, plan-gate rule 3) ends with a read-only smoke on a
real-shape v0.8.0 fixture with keys omitted the way old files omit them (a v0.5.0 file has none
of the light keys; a v0.7.0 file has no `poll_interval_ms`), and a v0.8.0 file with every key.

### 10.3 A beacon unplugged, and back

The record stays. The beacon's tray row and `GET /beacons` say `unplugged` (`connected: false`).
Its (beacon, line) watchers keep running: the flash deadline and the guards are the record's, not
the handle's, exactly as the single watcher survives a USB drop today (the loop never looks at the
beacon except in `applyState`, which skips while disconnected, `watcher.go:553-555`). When the
same match key reappears it is re-opened, re-bound to its record, and the wanted state is applied
on the next tick: a replug mid-pause resumes spinning. (`docs/PUNCHLIST.md:31` records that this
has never been checked on hardware for one beacon; §14.3 step 9 checks it for several.)

### 10.4 A beacon seen for the first time

It gets an **in-memory** record: its match key, a public id taken from the counter (the counter
advances in memory and is written with the record at the first save), `name: ""`, `lines: []`,
default light settings. It follows nothing, so no line can light it; the hand controls and a call
without a beacon choice do drive it (the owner: existing buttons "drive every beacon at once").
It is listed in the tray, on the settings page and in `GET /beacons`. **Nothing is written to the
config** until the operator names it or gives it a line on the settings page; a light borrowed
for an afternoon leaves no record behind, and its never-saved public id may be handed out again
after a restart, which is the one case the never-reused rule does not cover. Forgetting a
configured beacon is a button on its settings row, enabled while it is unplugged.

### 10.5 One writer

Today the config has one writer, the settings POST, which saves and hands the new config to
`main.go` through `restartCh`, and that restarts the HTTP server (`settings_handler.go:322-340`,
`main.go:49-83`). Binding a record to a light (§10.2) would be a second writer, and two writers
with no rule means a settings tab opened before a bind could save the pre-bind record over it.
The design has **one writer**:

- A `configStore` owns the live `Config` behind a mutex: `Get()` (what `getCfg` is today,
  `main.go:42-46`) and `Update(func(*Config) error)`, which applies the change to the live config,
  writes it with today's atomic owner-only save (`config.go:189-227`), and publishes it. Every
  change goes through it: the settings POST, a bind, Take over, Forget. Nothing else calls
  `saveConfig`.
- The HTTP server restarts only when `addr` or `port` changed in an update; a bind never touches
  them, so it saves without a restart. `restartCh` carries the restart, not the config.
- **The settings POST merges, by public id, against the registry**: the configured records
  plus the in-memory ones of §10.4. Each row carries its record's public id in its field names
  (`b1.name`, §11.2). For a configured id the form's name, lines and light settings replace the
  record's. For an in-memory id they do the same, and the record is **promoted** (written) only
  when its row carries a name, at least one line, or a Take over; otherwise it stays in memory,
  so a nameless borrowed light is never written by someone else's save. That is how naming a new
  light works (§14.3 step 4). A record in the registry that the form does not mention (a light
  bound after the page was opened) is **kept** as it is. The registry keeps **tombstones**, in
  memory, of the public ids forgotten since the tray started: a row whose id is a tombstone
  **refuses the whole save** with `409` and the sentence "Beacon b2 was forgotten after this page
  was opened — reopen Settings", and nothing is written. Any other unknown id is a **vanished**
  row (a light unplugged before the save): it is **ignored**, the rest of the save applies, and
  the page says so on reload ("Beacon b3 is no longer listed; its row was skipped"). A Forget
  before a restart therefore reads as vanished after it, which is safe: the record is gone either
  way. The record's match key,
  bound state and public id are never on the form and never change through it, **with one
  exception: Take over** (`b3.takeover=b2`, §11.2), which gives the record `b2` the match key of
  the light the row belongs to. So a stale tab can neither undo a bind nor resurrect a forgotten
  record, and a fresh light can be named the moment it appears.

## 11. The tray menu and the settings page

### 11.1 The tray menu

```
● Beacons: 2 of 3 plugged in                    (disabled row)
● Dashboard: watching · 2 lines                 (disabled row: today's sentence for the first line that is not watching, naming it; else this)
HTTP: 127.0.0.1:1337
Settings…
─────────────────────────
Spin            (every beacon)
Flash           (every beacon)
Stop            (every beacon)
─────────────────────────
Desk left — Spinning · Line A, Line C        ▸  ● Spinning
                                                 Following: Line A (watching · picks), Line C (watching · game state)
                                                 ─────
                                                 Spin
                                                 Flash
                                                 Stop
                                                 Identify
Desk right — Unplugged · Line B              ▸  …
New beacon (b3) — Idle · Unbound             ▸  …
─────────────────────────
Start with Windows
Quit
```

- `getlantern/systray` v1.2.2 has `AddSubMenuItem` (`systray.go:142`), `Hide` (`:189`) and `Show`
  (`:194`). **What the library really does on Windows:** an item's place in its parent is its
  **id-sorted position**, and ids are handed out in creation order (`systray.go:64`). Showing an
  item inserts it at that position (`addToVisibleItems` appends and then sorts the visible list
  by id, `systray_windows.go:680-690`; the insert uses the resulting index, `:564-571`), so a
  hidden item that is shown again **returns to its own place**, not to the bottom. And **any
  update to a hidden item shows it**: `SetTitle`, `Enable`, `Disable` and `Check` all go through
  `update()` (`systray.go:160-221`), and `addOrUpdateMenuItem` inserts an item that is not
  currently visible (`systray_windows.go:555-571`); `Hide` removes it (`:619-635`). The macOS
  implementation is its own code path (`systray_nonwindows.go:44-88`) and, like everything macOS
  here, is unverified. **So:** `onTrayReady` creates **eight** beacon submenus, hidden, each with
  its six children, **before the separator and Quit**, so each slot has a fixed place in the root
  menu whatever is shown or hidden around it. **The ordering rule:** the 500 ms sync loop
  (`tray.go:69-141`) updates **visible slots only**; a slot is assigned to a beacon in order of
  first appearance, and the retitle that gives it its beacon is what shows it, so that retitle
  happens **in the tick the slot is meant to appear and never earlier**, followed by its children
  and by `Show()` (a no-op on Windows, where the retitle already showed it); `Hide()` is the only
  call made on a slot being retired, and a retired slot is reused by the next new beacon. Eight is
  the menu's limit; a ninth beacon is still driven and listed by HTTP and the settings page, and
  the top row says `9 beacons (8 shown here)`. T7's fake menu models exactly this (an update to a
  hidden item makes it visible at its id-sorted position; `Show` and `Hide` move it in and out of
  that position), and its unit test fails on any update to a hidden slot outside the tick that
  shows it, and on any visible slot out of creation order. Whether a hidden submenu parent
  re-shows with its children intact on real Windows is a hand step in T10.
- Per-beacon rows reuse today's words: the status row's four states (`tray.go:107-136`), the
  bound row's `Bound: Line C` / `Bound: line 3 (unnamed)` / `Unbound` (`watcher.go:583-590`), one
  per followed line, and the watch row's sentences (`watcher.go:602-629`) per line.
- **The icon** is the §9.2 aggregate: grey when no beacon is plugged in, otherwise the busiest
  connected beacon's colour (orange spin > red flash > blue idle; `icons.go:60-69`). **S14's bound
  mark** (being built on `wt/s14-bound-icon`; the owner: *"blue with a brain or something saying
  its smart controlled"*) is shown when **any** connected beacon follows at least one line: the
  tray is smart-controlled if any light is. The hook S14 needs is one boolean,
  `anyConnectedBeaconFollowsALine()`; S14's own constant chooses the mark. The tooltip lists each
  beacon: `BlinkyBeacon — Desk left: spinning (Line A) · Desk right: unplugged · Spare: idle`.

### 11.2 The settings page

Same page, same single form, same CSRF token (`settings_handler.go:138-171`). The `Dashboard
Watcher` section loses its line picker; the `Beacon Light` section becomes the per-beacon block:

```
Bind Address [127.0.0.1]   Port [1337]

Dashboard Watcher
  Dashboard URL [https://dashboard.example.com]
  Dashboard API Token [••••••••]  (saved)

Beacons                                     Your dashboard's lines, read just now: Line A · running, Line B · stopped, Line C · running
┌─────────────────────────────────────────────────────────────────────────────────┐
│ ● Desk left                       plugged in · spinning            [Identify]  │
│   Name  [Desk left            ]   b1 · key port:6&2a1b3c4d&0&0001               │
│   Lines [x] Line A · running   [ ] Line B · stopped   [x] Line C · running      │
│   Draft-End Flash [15] s   New-Lobby Flash [ ] on, [10] s   Spin For Pauses From [Both sides ▾] │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ○ Desk right                      unplugged                         [Forget]   │
│   Name  [Desk right           ]   b2 · key port:6&2a1b3c4d&0&0002               │
│   Lines [ ] Line A   [x] Line B   [ ] Line C                                    │
│   Draft-End Flash [15] s   New-Lobby Flash [x] on, [10] s   Spin For Pauses From [Radiant only ▾] │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ● New beacon                      plugged in · idle                 [Identify]  │
│   Name  [                     ]   b3 · key port:6&2a1b3c4d&0&0003               │
│   This light is  [ — ▾ | Desk right, moved from its old port ]   (unplugged records only) │
│   Lines [ ] Line A   [ ] Line B   [ ] Line C                                    │
│   Draft-End Flash [15] s   New-Lobby Flash [ ] on, [10] s   Spin For Pauses From [Both sides ▾] │
└─────────────────────────────────────────────────────────────────────────────────┘
[Save & Apply]

HTTP API   (the existing block, with ?beacon= and GET /beacons added)
```

- **Lines** are checkboxes built from the same cached list the picker uses today
  (`settings_handler.go:433-453`, `lines.go:174-202`); a line a beacon follows that is not in the
  list is kept, ticked, as `Line 3 (not in the dashboard's list)`, for the reason the picker keeps
  it today (`settings_handler.go:526-534`). When the list is unavailable the control falls back
  to a text field `line numbers, comma-separated` with the same one-line reason (`token
  rejected`, `unreachable`, `no token`, and a fourth, `slowed by the dashboard`, when the list
  request itself was answered 429; `pickerUnavailableReason`, `settings_handler.go:464-476`).
- **Form fields** are keyed by the record's public id, which is safe in a field name: `b1.name`,
  `b1.lines` (multi-value), `b1.flash_seconds`, `b1.lobby_flash`, `b1.lobby_flash_seconds`,
  `b1.pause_side`, `b1.takeover` (the public id of the record to absorb). The match key is shown
  on the row as text and is not a form value. Validation per beacon is today's (`formSeconds`,
  `validPauseSide`), naming the beacon and field in the refusal: `Desk right: flash_seconds must
  be …`. Names are escaped on render (`html.EscapeString`, as every operator string is on this
  page), refused when duplicated, and refused when they would read as a public id (§5.4). The
  save is a merge by public id against the registry (§10.5): naming a new light saves it, a row
  for a light that vanished is skipped, and a row for a forgotten id refuses the whole save.
- **Identify** and **Forget** are buttons of the same form with `name="action"` values
  `identify:b1` and `forget:b2`; the handler looks the public id up in the live config and acts on
  that record only. An id that is no longer there is refused with `409` ("reopen Settings"), never
  applied to another row. Both act and redirect back to `/settings` without saving the rest of the
  form, so a half-edited page is not committed by an Identify click, and both carry the CSRF
  token by being in the form.
- **Take over** (`b3.takeover=b2`, the control reads "This light is Desk right, moved from its
  old port") treats the **row as the identity and the light as what moved**: the old record `b2`
  keeps its name, public id, lines and light settings and is given the new light's match key; the
  new light's in-memory record `b3`, and its never-saved id, are dropped. In the same `Update`,
  the **open handle, its current light state and its `identifyUntil` move** from the dropped
  record to the surviving one, so Desk right is driven at once, without a replug. The list offers
  **unplugged** records only; when no configured record is unplugged, the row shows no such
  control. One save through the one writer (D1, §10.5). A Companion button holding `b2` keeps
  driving Desk right after a port move.

## 12. Hot-plug and failure

Today: one goroutine opens the first beacon, retrying every 10 s (`main.go:85-110`); the
library's keepalive worker writes the current command every 5 s and calls `OnError` on a failed
write (`pkg/fsbeacon/usbhid.go:70-80`), which `main.go:95` turns into `SetBeacon(nil)`; the
goroutine then closes the handle and goes back to retrying.

New: a **device manager** goroutine owning a map `match key → open handle`, talking to the
library through the seam in §14.2:

1. Every 10 s (today's cadence, kept): `Enumerate()`. For each path not already open (the
   manager holds each open handle's path in memory, and nothing persists it): **skip the path if
   T0 recorded a preferred usage pair and this path's is not it** (§3: a light's second collection
   is never opened); otherwise `Open`, read the match key from the handle (`Key()`, §5.3; on macOS
   that is the location id, read after open), then:
   - key not held → bind to the record with that key, or to an unmatched record (§10.2), or
     create an in-memory record (§10.4); `StartIdleWorker()`; register the unplug callback with
     `OnUnplug`.
   - key held by a handle that is **not live** (marked unplugged, or its last keepalive attempt
     failed, read from `LastAttemptOK()`; no age test) → a **replug**: close the old handle on
     this goroutine, bind the new one to the record, start its keepalive. This is the macOS replug
     (a new path for the same light); on Windows a replugged light keeps its path, so the "not
     already open" filter above decides first.
   - key held by a **live** handle → a second light reporting an identity already held: the
     path is not opened and the tray says so, without naming a cause (in `port` mode two lights
     present at once never share a key, so the tray cannot know why). The path is tried again at
     **every** enumerate, so a stale handle whose keepalive fails in the meantime hands over
     within one keepalive period and one enumerate, about 15 s.
2. **The unplug callback does two things and nothing else: it marks that beacon unplugged and
   sends its own handle on the manager's channel.** The manager's own goroutine acts only if that
   handle is still the one bound to the record, and closes it; a callback from a handle that a
   replug or a Take over has since displaced is **ignored**, so a displaced handle's late failure
   (up to 5 s after it lost the record) never tears down its successor. The
   callback runs on the library's keepalive goroutine **while that goroutine holds the
   `FarmBeacon` mutex** (`pkg/fsbeacon/usbhid.go:63-66`: the lock is taken for the worker's whole
   life, and `OnError` is called inside it at `:77-80`). A `Stop()` from inside the callback would
   send on `beacon.stop`, which only that same worker reads (`:83`), and block for ever; any other
   command on the handle would wait on the mutex. T5's fake keepalive holds the fake's mutex while
   it fires the callback, and every command on the fake takes that mutex, so a close or stop from
   inside the callback deadlocks the test, which fails on a two-second timeout rather than hanging.
3. A failed command on a beacon, from any caller (tray, HTTP, watcher), marks that beacon
   unplugged and hands the close to the manager the same way, as each caller does today for the
   one beacon (`main.go:127-128`, `server.go:87`, `watcher.go:566-571`); the others are untouched.
4. Each beacon is its own `FarmBeacon` with its own mutex, worker goroutine and handle
   (`pkg/fsbeacon/usbhid.go:38-45`); nothing is shared between two lights but the `hid` library's
   global init, which is idempotent.

**One beacon failing must not darken the others:** every per-beacon path above touches one map
entry under one lock; the tick's apply step (§8.1, 3) applies per beacon and treats a failed
command as that beacon's unplug, then continues with the next beacon. The device-manager test
(task T5) pins it: three fake beacons, one made to fail its write; the other two keep their state
and their next commands.

**Quit** stops every beacon and closes every handle (today's `OnQuit`, `main.go:163-178`, over
the map).

The **library change** this needs (task T1): `pkg/fsbeacon` gains `Enumerate() ([]DeviceInfo,
error)` and `OpenPath(path string) (*FarmBeacon, error)`, `FarmBeacon` keeps the `DeviceInfo` it
was opened from and exposes it as `Info()`, and on macOS `LocationID()`. `OpenFarmBeacon()` stays
as it is for the upstream CLI (`cmd/fsbeacon`). The tray's `farmHandle` (§14.2) wraps all of it.
The upstream comment at `usbhid.go:3` is corrected once the probe has shown what it meant.

## 13. macOS

What differs from Windows, all of it inside `hidapi` and already exercised by today's one-beacon
code except the identity source:

- **Enumeration** goes through `IOHIDManagerCopyDevices` with vendor and product matching
  (`hid_darwin.c:700-792`); each match gets a `DevSrvsID:` path (§5.2).
- **Open** is `IOHIDDeviceOpen` with seize (`hid_darwin.c:470`, `:1056`), so a second process
  cannot open a light the tray holds. `hid.SetOpenExclusive(false)` exists (`hid_darwin.go:45`);
  not used, because an exclusive hold is what the tray wants.
- **Each open device gets its own thread and run loop** for input reports and the removal
  callback (`hid_darwin.c:1079`, `read_thread`); three lights are three threads. Unplugging sets
  `disconnected` (`hid_darwin.c:861`) and the next write fails, which is the same `OnError` path
  as Windows.
- **Identity** is the location id, read after open (`hid_darwin.go:33`); the enumeration path is
  per-attach, held in memory for the open handle and never persisted (§5.2). The port-key function is therefore platform-specific, the
  only platform-specific line in the identity code.
- **Permissions:** unknown. Whether opening this light trips a macOS permission prompt is one of
  §3's unknowns, settled at the first Mac run; nothing here assumes either way.

Nothing here can be run in this session (`cgo` needs Cocoa and IOKit), and no Mac has ever been
run against a light (`docs/PUNCHLIST.md:30`). Verification stays what it is for every release:
the `build` workflow's macOS jobs green on the pushed branch (`.github/workflows/build.yml`,
`macos` job), the artifact unzipped and listed. The hands-on macOS check waits with the punchlist
item. The probe's `extra_darwin.go` is 27 lines, ten of them code, against a signature read from
the library source; it is marked unverified in the retro.

## 14. Testing

### 14.1 The probe, step by step (the owner, once)

1. Plug in every beacon. **Quit the tray** from its menu first: only one program should hold a
   beacon (§5.1 says why on each platform).
2. Run `beacon-probe.exe`. Double-clicking is fine: it writes its findings to the window and to
   `beacon-probe.txt` beside itself, and waits for Enter. Send back `beacon-probe.txt`.
   **Stop here, after sending the file, if** its `summary` says `DUPLICATED` or `MIXED`, or if two
   `beacon N` entries show the same `port key` **and** the same `usage page / usage`: those lights
   cannot be told apart, and the design is revisited before anything is built (§3, §5.3). Two
   entries with the same `port key` and *different* usage lines are one light with two
   collections, which is fine; step 4 shows it as one light flashing twice.
3. Move one beacon to another USB port and run it again. It appends to the same file, so the
   before and after sit together. (A run after a reboot, same ports, is one more useful pair.)
4. Run `beacon-probe.exe -flash` from a terminal (`cmd` or PowerShell in the exe's folder). Each
   light flashes for two seconds in turn and the window names which one; then all stop. **Write
   down, for each `beacon N` the window names, which physical light flashed** (left, middle,
   right, or a sticker), and send that with the file: it is what tells two paths of one light
   apart from two lights, and which path the build opens (§3).

The exe: `beacon-probe.exe`, sha256
`5302d50d7a434c86346e76fe2d6d4345607ade27a7acf83181fa34078356b0e4`, from commit
`fcc9126b7dae47a307de460265a69de9a0a4ee1a`, built inside `tools/beacon-probe/` with

```
GOOS=windows GOARCH=amd64 CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc \
  go build -trimpath -ldflags='-buildid=' -o beacon-probe.exe .
```

(the release line without `-H windowsgui`: this is a console program, and that flag would hide
its window). It is its own Go module; `go list ./...` from the repository root lists the same four
packages as before, and `go test -count=1 -race ./cmd/blinkybeacon-tray/` passed with it in the
tree.

### 14.2 The fake device layer the suite drives

The device manager (§12) takes an interface, so the suite needs no hardware and no `libudev`
(`go test ./...` is red on Linux for that reason, `docs/PUNCHLIST.md`, "Documented quirks"):

```go
// devices.go — the seam between the device manager (§12) and the USB library.
type deviceSource interface {
    Enumerate() ([]deviceInfo, error)            // every beacon the library can see
    Open(info deviceInfo) (deviceHandle, error)  // open that one
}

type deviceInfo struct {
    Path      string // what Open takes; held in memory for open handles, never persisted
    Serial    string // "" when the light reports none
    UsagePage uint16 // §3: a light with several collections shows one path per collection
    Usage     uint16
}

type deviceHandle interface {
    Beacon            // Flash, Spin, Stop, Close — the local mirror (state.go:19-24)
    Key() string      // the match key, "sn:…" or "port:…" (§5.3), computed by the source;
                      // on macOS from the open handle's location id (hid_darwin.go:33)
    OnUnplug(func())  // registers the callback the keepalive fires on a failed write
                      // (FarmBeacon.OnError, pkg/fsbeacon/usbhid.go:44)
    StartIdleWorker() // pkg/fsbeacon/usbhid.go:155-157
    Live() bool       // not marked unplugged, and the last keepalive attempt succeeded
                      // (FarmBeacon.LastAttemptOK, an atomic the worker sets after each write
                      // and nothing reads under the FarmBeacon mutex; §12, item 1)
}
```

The real source, `farmSource`, and its handle, `farmHandle` (a `*fsbeacon.FarmBeacon` with the
`DeviceInfo` it was opened from), live behind `//go:build windows || darwin` as `main.go` does
today (`main.go:1`); `beacon_check.go` gains `var _ deviceHandle = (*farmHandle)(nil)` beside its
existing assertion (`beacon_check.go:10`). The fake, `fakeBus`, is a test type in the tray package:

- `plug(serial, portKey, usagePage, usage)` adds a device; `unplug(portKey)` removes it from
  enumeration and makes its open handle's next `Spin`/`Flash`/`Stop` return an error, and fires
  the registered unplug callback **from the fake's own keepalive goroutine while holding the
  fake's mutex**, which every command on the fake also takes (§12, 2). **`unplug` models the
  stale window:** the handle's `LastAttemptOK` stays true until the fake keepalive's next write
  fails, so inside that window a plug of the same key at a new path (a macOS replug) is refused
  once and bound at the next enumerate; and **the old handle still fires its callback after the
  replug**, so the test sees the manager ignore a displaced handle's callback. `move(portKey, newPortKey)`
  is an unplug then a plug with the same serial and a new port; `failNext(portKey)` makes one
  command fail without unplugging, for the "one beacon failing" case.
- Each fake handle counts commands, as `countingBeacon` does, and records the order they came in.
- The fake's identity mode is set per test (`sn` or `port`), so both branches of §5.3 are
  exercised whatever the probe decides.

**What changes in the existing tests, honestly.** `AppState.SetBeacon` goes away with the registry
(T3), so every test that plants a beacon changes its setup line: `watch_loop_test.go` (23 calls
across 54 tests), `server_test.go` (3 calls, 6 tests), `watcher_test.go` (3 calls, plus 8 calls of
`applyState`, which becomes per-beacon), `watch_loop_cadence_test.go` (2),
`settings_handler_test.go` (1), `state_test.go` (3, rewritten for the registry) and
`integration_test.go` (3, skipped without a dashboard). The fakes stay: `mockBeacon`
(`server_test.go:10-21`) and `countingBeacon` (`watcher_test.go:508-516`) remain `Beacon`
implementations, planted through one test helper, `plant(app, key, Beacon)`, which wraps them in
a `fakeHandle` with a fixed key, a no-op `StartIdleWorker`, `Live()` true and a stored callback.
The dashboard stubs and every assertion stay. About 35 setup lines change, and no expectation
changes, **except in `state_test.go`**: its four tests are about `AppState`'s single beacon
(`SetBeacon` clearing to idle, and so on) and are rewritten as registry tests in T3.

Scenarios the suite pins: three beacons bound to three records; unplug one and the other two keep
their state and take their next commands; replug and the record re-binds and the wanted state is
re-applied (a pause resumes spinning); a port move yields one unplugged record and one new
unassigned beacon, and Take over gives the old record the new key, moves the open handle, its
state and `identifyUntil` to it, and drops the new one's in-memory record; a replug at a new path
inside the stale window is refused once and bound at the next enumerate, and the displaced
handle's late callback is ignored; a nameless in-memory row is not written by a save, a named one
is; a first-seen beacon follows nothing and is driven by a bodiless `POST /spin`,
and naming it on the settings page saves it; a row for a light that vanished before the save is
skipped while the rest of the save applies; an unmatched v0.8.0 record binds to the first light and the config is
saved with its key, without an HTTP restart; a settings save from a page opened before that bind
keeps the bind (§10.5); a save naming a forgotten public id is refused and nothing is written;
Identify or Forget on a gone id is refused; in `sn` mode a duplicate serial at run time is refused
and reported; a close from inside the unplug callback deadlocks (the test's own guard, §12, 2).

### 14.3 The owner's hands-on plan, three or more real lights

Written for someone at a desk with the built `BB-DASH.exe`, three lights, and a dashboard with at
least two official lines (call them Line A and Line B). Each step says what to look for.

1. **Upgrade in place.** Quit the old tray. Put the new `BB-DASH.exe` beside the old
   `blinkybeacon-config.json`. Plug in **one** light only. Start the tray. *Expect:* the tray behaves
   exactly as before: `● Beacon: Idle`, the bound row names the old line, the light reacts to a
   pause on it. Open Settings: one beacon row, `Beacon 1`, following the old line, with the old
   light settings.
2. **Name it.** Type `Desk left`, Save. *Expect:* the tray's row says `Desk left`; the config file
   now has a `beacons` list with `"id": "b1"` and a `key`.
3. **Plug in the second light.** Within ten seconds, *expect:* a `New beacon` row in the tray
   (`Unbound`) and on the settings page, following nothing. The dashboard pausing Line A spins
   Desk left only.
4. **Identify.** Click Identify on the new row. *Expect:* the second light flashes for two
   seconds; the first does not. Name it `Desk right`, tick Line B, Save.
5. **Two lines, two lights.** Pause Line B on the dashboard. *Expect:* Desk right spins, Desk left
   dark. Pause Line A too. *Expect:* both spin. Unpause A. *Expect:* only Desk right spins.
6. **One light, two lines.** On Desk left tick Line A **and** Line B, and set its Draft-End Flash
   to `60` for this step so the window is long enough to see; Save. Pause Line B. *Expect:* both
   lights spin (Desk left for B, Desk right for B). End a draft on Line A while B is paused.
   *Expect:* Desk left keeps spinning (the pause outranks the flash; §7.2 row 2), Desk right keeps
   spinning. Unpause B **within the minute**. *Expect:* Desk left **flashes** for what is left of
   the 60 s (§7.2 row 7), then goes dark; Desk right dark. Then put the flash back to `15`, Save,
   and repeat the pause and draft end, but wait **more than 15 s** before unpausing. *Expect:* Desk
   left goes dark, the window having run out under the pause.
7. **Different settings on one line.** Set Desk left's draft flash to 5 s and Desk right's to 20 s;
   both follow Line A only. End a draft on A. *Expect:* both flash; Desk left stops after about
   5 s, Desk right after about 20.
8. **The Companion buttons.** With the module's existing Spin button, *expect:* **both** lights
   spin; Stop stops both. From a terminal, `curl -X POST http://127.0.0.1:1337/flash?beacon=b1`,
   then `curl -X POST "http://127.0.0.1:1337/flash?beacon=Desk%20left"` (the same by name).
   *Expect:* only Desk left flashes, both times; `curl http://127.0.0.1:1337/status` says
   `{"state":"flash","connected":true}`; `curl http://127.0.0.1:1337/beacons` lists both with
   their states. (The watcher corrects both on its next tick if they follow lines; untick the lines
   first for a longer look.)
9. **Unplug mid-pause.** Pause Line B; Desk right spins. Pull its USB. *Expect:* within five
   seconds the tray says `Desk right — Unplugged`; Desk left is untouched (still dark, or still
   spinning if it follows B). Plug it back into the **same** port. *Expect:* within ten seconds
   it is `Desk right` again and **spins**, because B is still paused.
10. **Port move.** Quit nothing. Move Desk right to a **different** port. *Expect, if the probe
    found no serial numbers:* `Desk right — Unplugged` stays, and a `New beacon` (`b3`) appears. On
    the settings page the new row offers `This light is: Desk right, moved from its old port`;
    choose it, Save (the control is there only while some configured beacon is unplugged; a new
    light found while none is shows no such control). *Expect:* one row, `Desk right`, **still
    `b2`**, in the new port, following B, and spinning at once if B is still paused (no replug
    needed); `b3` is gone;
    `curl -X POST http://127.0.0.1:1337/flash?beacon=b2` still flashes it. *If the
    probe found serial numbers:* nothing changes but the port; it is `Desk right` as soon as it is
    found.
11. **A third light, never configured.** Plug in the third. *Expect:* `New beacon`, follows
    nothing. The module's Spin button spins all three. Unplug it. *Expect:* it vanishes from the
    tray; nothing was written to the config for it.
12. **Quit** with all lights spinning. *Expect:* all go dark.
13. **Token revoked** (optional, needs a dashboard admin): revoke the token. *Expect:* every bound
    beacon goes dark on the next poll and every tray row says `token rejected`; after re-minting,
    `watching` returns within thirty seconds.

### 14.4 The suite

Every task in §15 names its tests. The whole suite stays `go test -count=1 -race
./cmd/blinkybeacon-tray/`, offline, on Linux, with the fake bus; the library package keeps its
hardware test as it is (`pkg/fsbeacon/usbhid_test.go`, skipped without a light).

## 15. Build order

Each task is one PR-sized unit a reviewer can hold. **Numeric** marks plan-gate rule 1 (an
exact-arithmetic property test sweeping beyond the pinned values, with the pinned values named).
**Parser** marks rule 3 (a read-only smoke on a real-shape input with omitted keys). Every task
touches a third-party API surface, a loader, or the HTTP contract, and the Marks column says
which.

| # | Task | Touches | Tested by | Marks |
|---|---|---|---|---|
| T0 | **Fix the identity mode from the probe's file.** One constant (`sn` or `port`), `matchKey(info, dev)` with the Windows and macOS port-key functions, and the collection rule (§3: the usage pair to open when a light shows two paths). Stops on `MIXED`, `DUPLICATED` or a port-key collision (§5.3). | new `beacon_key.go`, `beacon_key_windows.go`, `beacon_key_darwin.go` | table test over sample paths and location ids, both modes; the probe's own lines as fixtures; two paths with one instance segment yield one key | parser (of the path) |
| T1 | **Library: enumerate and open by path.** `fsbeacon.Enumerate()`, `fsbeacon.OpenPath()`, `FarmBeacon.Info()`, `LastAttemptOK()` (an atomic bool the worker sets after each keepalive write, for `Live()`; never read under the mutex), `LocationID()` on macOS; `OpenFarmBeacon()` unchanged. | `pkg/fsbeacon/usbhid.go` (+ a darwin file) | `go vet` under `GOOS=windows` and `darwin` via CI; `beacon_check.go`'s compile-time assertion; the existing hardware test | third-party API |
| T2 | **Config: the `beacons` list, public ids, migration, mirror, and the one writer.** §10.1–10.2, §10.5, D5. `configStore` with `Get`/`Update`; `beacon_seq`. | `config.go`, new `config_store.go`, `config_test.go`, `main.go` (`getCfg`/`restartCh` move onto the store) | load of a v0.5.0, v0.7.0 and v0.8.0 file each yields one unmatched `b1` with the right values; save-then-load round trip; mirror keys written from `beacons[0]`; `beacon_seq` only ever rises and is raised to the highest id on load; a missing, malformed (`b01` included) or duplicate `id` is re-issued and logged; forgiveness per field; an `Update` that changes neither `addr` nor `port` triggers no restart; **read-only smoke on the real-shape fixtures with omitted keys** | parser |
| T3 | **The beacon registry and the aggregate.** Records, handles, per-beacon state, `identifyUntil`; `aggregateState`, `aggregateConnected` (§9.2). Replaces `AppState`'s single beacon. | `state.go` → `beacons.go`, `state_test.go` | property test: for every multiset of up to six beacon states, the aggregate equals the rank maximum; connected = any; **pinned values: the §9.2 table**; the existing `state_test.go` cases ported | numeric |
| T4 | **Per-pair watchers, the combine, and the 429 backoff.** §7; the loop's fan-out and the per-line throttle (§8.1). `Watcher` unchanged. | `watch_loop.go`, `watcher.go` (`pollFailureStatus` gains 429, `WatchThrottled`, the `Retry-After` reader), new `combine.go`, tests | the §7.2 table, rows 1–10, pinned; property sweep over all 3^6 answer combinations; deadlines (rows 4, 7); **"pause lifts inside the flash window → flash resumes" pinned on one pair** (row 7, today's `Decide`); the 401 backoff shared and the 404 per line; a 429 backs off that line only, honours `Retry-After` in seconds and as a date, caps at 30 s, defaults to 10 s; a 503 stays `lost` at full cadence; one fetch per distinct line (count on the stub) | numeric |
| T5 | **The device manager with the fake bus.** §12, §14.2: `deviceSource`, `deviceHandle`, `farmSource`/`farmHandle`, the manager goroutine and its close channel. | `main.go` (the USB goroutine), new `devices.go`, `devices_farm.go` (windows ‖ darwin), `beacon_check.go`, `fakebus_test.go`, the ~35 setup lines listed in §14.2 | the §14.2 scenarios, the stale-window replug, the ignored displaced-handle callback and the skipped second-collection path included; the "one fails, two go on" case; **the deadlock guard: a close from inside the unplug callback fails the test on its two-second timeout** | third-party API |
| T6 | **HTTP: `?beacon=`, `GET /beacons`, the aggregate `/status`.** §9. | `server.go`, `server_test.go`, the API block in `settings_handler.go:86-106` | **every existing `server_test.go` assertion holds, with its setup line changed as §14.2 says**; every row of the §9.1 reply table, the 400 and the three 503s included; `/beacons` shape golden; `/status` ignores `?beacon=`; 404 or 400 before any command on a mixed call; a percent-encoded name resolves | HTTP contract |
| T7 | **The tray menu and icon.** §11.1 (eight root-level slots before the separator and Quit, the ordering rule), the S14 hook. | `tray.go`, `icons.go`, new `tray_slots.go` with its fake-menu test | the label functions table-tested as today's are; the slot logic unit-tested on a fake menu that **models Windows (an update to a hidden item makes it visible at its id-sorted position; `Show`/`Hide` move it in and out of that position)**, failing on any update to a hidden slot outside the tick that shows it and on any visible slot out of creation order, over plug/unplug/forget/replug sequences; the rest by hand (§14.3, T10) | — |
| T8 | **The settings page.** §11.2, the merge (§10.5). | `settings_handler.go`, `settings_handler_test.go` | form round trip for two beacons; fields keyed by public id; duplicate name refused; a name that reads as a public id refused; names escaped; the merge against the registry: naming a new light saves it, a nameless in-memory row is not written, a record bound after the page loaded survives the save, a row for a light that vanished is skipped and the rest applies, a row for a tombstoned id refuses the save with 409 and writes nothing; Identify and Forget act by public id without saving the form, and refuse a gone id with 409; Take over keeps the old record's public id, moves the handle and state, drops the new light's record, and is offered for unplugged records only; `slowed by the dashboard` as the line list's fourth reason; the line checkboxes from the cached list and the comma-separated fallback; CSRF on every action | — |
| T9 | **Docs.** `BEACON-DASHBOARD.md` §1, §2 (the picker is now per beacon), §4, §5 (the tray rows), §7 (the config sample; **correct lines 351-354 to what the code does: a pause that lifts inside the flash window hands the light back to the flash**), §8 (drop "only one line at a time"); README's API block; the settings page's API text. | docs | a reviewer reads them against the shipped behaviour, step by step with §14.3 | — |
| T10 | **Hands-on and CI.** §14.3 on the owner's lights, plus one check on real Windows that a hidden beacon slot re-shows with its children intact; the `build` workflow green for Windows and both macOS legs. | — | the owner's report; the artifacts listed | — |

Order: T0 and T1 first (T0 waits on the probe's file; T1 does not), then T2 and T3 in parallel,
T4, T5, T6, T7 and T8 in parallel, T9, T10. The upstream vet redness on
`cmd/fsbeacon/commands/root.go` stays as it is: vet the tray package only, as every kickoff does.

## 16. Out of scope, and what the dashboard would need

Out of scope here, by the kickoff: the token (S17/S18), notifications, history and sound (S15),
and anything on the dashboard.

**The dashboard needs nothing from this design.** Load (§8.2) stays under what the read API was
designed for; the line list and the per-line projection are consumed as they are. No issue is
filed for S16.

Two things are noted for later, not requested: S9's dashboard half (`test_flash_seq`, already
DOTA-LIVE-DASHBOARD#90) will arrive on every line reply, and with several beacons the tray should
flash every beacon that follows that line, which the per-pair watcher gives for free; and the
"paused for M:SS" timer (S11, deferred) would be per beacon, taking the longest of its lines'
pauses.

## 17. Evidence appendix

Measured or read in this session, in the worktree `wt/s16-spec` cut from `origin/dashboard`
`6407297`:

- `git log --oneline --all | grep -i 'S16\|several beacons\|multi.beacon'` found only the two
  tracker commits `de5c072` and `eff1c08` (docs only, `git show --stat`); `git ls-remote origin
  'wt/*s16*'` found nothing. S16 had not begun.
- `go test -count=1 -race ./cmd/blinkybeacon-tray/`: `ok … 1.936s` before the probe was added,
  `ok … 1.914s` with it in the tree. `go vet` of the tray package under `GOOS=windows`: clean.
- `go list ./...` from the root, with `tools/beacon-probe/` present: the same four packages
  (`cmd/blinkybeacon-tray`, `cmd/fsbeacon`, `cmd/fsbeacon/commands`, `pkg/fsbeacon`).
- The probe: `go vet` under `GOOS=windows` clean; built twice from `fcc9126` to the same sha256
  `5302d50d…b0e4`; `go version -m` shows `vcs.revision=fcc9126b…ee1a`, `vcs.modified=false`;
  `file` says `PE32+ executable (console) x86-64`. Build time 3.2 s.
- Library sources read: `go-hid@v0.14.1/hid.go`, `hid_windows.go`, `hid_darwin.go`,
  `hid_windows.c`, `hid_darwin.c`, `hidapi.h`; `systray@v1.2.2/systray.go` and, for the
  amendments, `systray_windows.go` (`addOrUpdateMenuItem`, `hideMenuItem`).
- Companion module read at `~/projects/companion-module-blinkybeacon` (v1.1.0): `src/index.ts`,
  `src/actions.ts`, `src/feedbacks.ts`.
- Not measured here, and said so where it is used: Windows instance-id behaviour across reboot
  and port move (§5.2, Windows documentation; probe step 3); macOS location-id stability (§13);
  `hid.Enumerate` cost on Windows (§3); upstream's "hardware limitation" (§3, probe step 4).

## Amendments — red team 2026-10-20

A blind red team attacked this document at `e3760f8` and confirmed fourteen findings; the
master ruled on each the same day. The rulings are applied above; this section says where. The
owner's five decisions (§2) keep their recommendations; D1, D4 and D5 have corrected reasoning.

| # | Finding | Ruling | Where the text changed |
|---|---|---|---|
| F1 | The hardware key (`port:6&2a…&0&0001`) cannot travel unencoded in `?beacon=`: Go splits the query on `&`, and the Companion module builds URLs by template. | The API's beacon id is a short public id the tray assigns (`b1`, `b2`, …), never reused after a Forget; the hardware key stays internal as the match key. `?beacon=` values are percent-encoded like any query value. A name that reads as a public id is refused at save. | §1; §5.3 ("Two ids per beacon"); §5.4; §6; §9.1; §9.3; §10.1 (`beacon_seq`, `key`); §10.2; §10.4; §11.2; §14.3 steps 2, 8, 10; T0, T2 |
| F2 | §14.3 step 6 ("goes dark, not back to flashing") contradicted §7.2 row 7 and today's `Decide`, which keeps `flashUntil` through a pause; the user guide's sentence overstates the code. | Row 7 is the rule: paused > flash window > idle. A flash whose window is still running when the pause lifts resumes for what is left. | §1; §7.2 closing paragraph; §14.3 step 6 (now sees both outcomes); T4 (the pin test); T9 (correct `BEACON-DASHBOARD.md:351-354`) |
| F3 | D1's argument against guessing was wrong: in port mode a swap matches both records and never triggers a guess. | Rewrite the alternative with the real risk (a light in a drawer, a borrowed light in a fresh port inherits its lines); say a swap behaves the same under both options. Recommendation stands. | §2 D1 |
| F4 | A duplicated serial is still a serial to Windows: the instance id is built from it, so the port key stops being a port. The design had no mode for `DUPLICATED`. | `DUPLICATED` is its own outcome and a stop: the build picks no mode; the probe file comes back to the design. Say what to read in the file. | §3 (row 1); §5.3; §4 step 2; §14.1 step 2; T0 |
| F5 | The §14.2 seam `Open(path) (Beacon, error)` could not carry `OnError`, `StartIdleWorker` or the macOS location id; "existing tests pass unchanged" did not hold. | Redesign the seam (`deviceSource`, `deviceInfo`, `deviceHandle` with `Key`, `OnUnplug`, `StartIdleWorker`); budget the test changes honestly. | §12; §14.2 (the types and "What changes in the existing tests, honestly"); §9.1 last paragraph; §7.1; T1, T5, T6 |
| F6 | On Windows any `SetTitle`/`Enable`/`Disable`/`Check` on a hidden item re-inserts it; the eight-slot plan had no ordering rule. | State the ordering rule; T7's fake models Windows; the hidden-parent check on real Windows is a hand step in T10. **Completed by the re-check (N3 below):** revision 1 stated the library's behaviour wrongly ("lands at the bottom, below Quit") and moved the slots under a `Beacons ▸` entry on that false reason; revision 2 returns them to the root, pre-created before the separator and Quit, with the true behaviour cited. | §2 D4; §11.1 (layout and rule); T7; T10 |
| F7 | Two config writers (the settings POST and the bind-save) with no merge rule; a stale tab could undo a bind; a bind through `restartCh` would restart the HTTP server. | One writer. The settings POST merges by public id (a forgotten id refuses the save; an unmentioned record is kept); a bind saves without a restart. | new §10.5; §10.2 step 2; §11.2; T2, T8; §14.2 scenarios |
| F8 | The load argument cited the page-route measurement, not `/api/v0`, and omitted S79's "no rate limiting in v0, Cloudflare can throttle"; the tray has no backoff on 429/5xx. | Correct the claim, keep the arithmetic. Add per-line backoff on 429 only, honouring `Retry-After`, capped at 30 s; 5xx and unreachable unchanged. Name the status. | §8.1 ("A throttled line", status `throttled`); §8.2; §9.3 (`watch`); T4 |
| F9 | §9.1's reply table had holes: all named beacons failing, a mix of unplugged and live, an empty `beacon=`, a name equal to another beacon's id. | Fill every hole with an explicit row; the no-parameter path keeps today's replies exactly. | §9.1 (the table); T6 |
| F10 | The macOS "no Input Monitoring permission" claim was stated as fact on an app never run against a light on a Mac. | Move it to §3's unknowns, settled at the first Mac run. | §3 (new row); §13 ("Permissions") |
| F11 | §3 did not ask whether one light is one HID path (several top-level collections share an instance segment). | Add it, with the reading rule from the probe file and what the design does if a light shows two paths. | §3 (new row); §14.1 step 2; §14.2 (`deviceInfo` carries the usage pair); T0 |
| F12 | `OnError` runs on the keepalive goroutine holding the `FarmBeacon` mutex; closing from inside it deadlocks; the fake had no mutex. | The callback only marks the beacon unplugged and hands the close to the manager's goroutine; say why; T5's fake models the mutex so a close from inside the callback deadlocks the test. | §12 (item 2); §14.2 (`unplug`); T5 |
| F13 | D5's "costs nothing": an older build's Save drops the `beacons` list. | Correct to the real cost; keep the recommendation with the caveat. | §2 D5 |
| F14 | Identify and Forget acted on a form index, not the record id; a stale tab could delete the wrong record. | They carry the record's public id and the server checks it; a gone id is refused, never applied to another record. | §11.2; T8; §14.2 scenarios |

Also corrected while applying the rulings: `extra_darwin.go` is 27 lines, ten of them code
(§13), and the test-reuse sentence in §7.1 now says what it can.

### Re-check of revision 1 (`9692bd2`)

The red team re-checked revision 1: F1–F5 and F8–F14 resolved, F6 partly, and six defects in the
revision itself. The master ruled all in; applied as below.

| # | Finding | Ruling | Where the text changed |
|---|---|---|---|
| N1 | Take over deleted the old record and retired its public id, so a Companion button holding `b2` would 404 after a port move. | The row is the identity; the light moved. Take over keeps the old record, its name and its public id, and gives it the new light's match key; the new light's in-memory record and its never-saved id are dropped. The match key never changes through the form except by Take over. | §2 D1 (wording); §10.5 (the exception); §11.2 (the control reads "This light is …, moved"; the Take over bullet); §14.2 scenarios; §14.3 step 10; T8 |
| N2 | The 409 rule, read against the live config, would refuse naming a first-seen (in-memory) light, which §14.3 step 4 needs. | Merge against the registry (configured plus in-memory records). A row for an in-memory record that has since vanished is ignored and the rest of the save applies; only a configured id that is gone refuses with 409. | §10.5 (the merge bullet); §11.2 (form fields); §14.2 scenarios; T8 |
| N3 | Revision 1's library claim was false: the library re-shows an item at its id-sorted position, not at the end (`systray_windows.go:564-571, 680-690`), so "below Quit" was no reason to move the slots; and "retitle before Show" contradicted "never touch a hidden slot". | Return to the D4 the owner was first shown: a submenu per beacon at the root, eight slots pre-created before the separator and Quit. State the true behaviour. The rule: update only visible slots, and accept that the retitle of a hidden slot is what shows it, so retitle in the tick the slot is meant to appear and never earlier. T7's fake models that. The false reason removed everywhere. | §2 D4; §11.1 (layout and rule); T7; the F6 row above |
| N4 | On a macOS replug (a new path for the same light) the `sn`-mode guard would fire against the stale handle during its 5 s keepalive window; no retry was specified; port mode's second handle was unaddressed. | A key held by a handle marked unplugged, or whose keepalive has not succeeded within the last period, is a replug: close the old handle, bind the new one. The guard fires only against a live handle. A refused path is retried at every enumerate. The same rule covers port mode. The fake's `unplug` models the stale window. | §5.3 (the guard); §12 (item 1, three cases); §14.2 (`Live()` on the handle; `unplug`; scenarios); T1 (`LastWrite()`); T5 |
| N5 | No loader forgiveness for `beacon_seq` or a duplicate or malformed `id`. | On load, `beacon_seq = max(beacon_seq, highest id number)`; a duplicate or malformed `id` is re-issued from the counter and logged. T2 tests both. | §10.1; T2 |
| N6 | (a) "never stored" path; (b) the probe's tiebreak when both writes succeed; (c) "no test's expectation changes" versus `state_test.go` rewritten; (d) no 429 word for the settings page's line-list fetch. | (a) a path is never persisted, held in memory for open handles; (b) step 4 asks which light flashed for each `beacon N`, and the tiebreak uses that answer; (c) the contradiction removed; (d) `slowed by the dashboard` beside the three existing reasons. | (a) §12 item 1, §13, §14.2 (`deviceInfo`); (b) §3 (the HID-path row), §14.1 step 4; (c) §14.2; (d) §11.2, T8 |

### Re-check of revision 2 (`a17e555`)

The red team re-checked revision 2: N1–N6 and F6 resolved, and seven gaps in the new mechanisms.
The master ruled all in; this was the last design round, so each is closed with the smallest
wording.

| # | Finding | Ruling | Where the text changed |
|---|---|---|---|
| R1 | Take over re-keyed the record but left the open handle bound to the dropped record, so step 10 failed until a replug. | In the same `Update`, the open handle, its light state and `identifyUntil` move to the surviving record. | §11.2 (Take over); §14.2 scenarios; §14.3 step 10; T8 |
| R2 | The replug test "no keepalive within 5 s" was phase-locked with the 10 s enumerate and could close a healthy light; the Windows clause was unreachable; `LastWrite` under the mutex was the F12 trap. | "Not live" means marked unplugged, or the last keepalive attempt failed; no age test; recovery is the retry at every enumerate. The Windows clause dropped. The result is read from an atomic, `LastAttemptOK()`, never under the mutex. | §5.3; §12 item 1; §14.2 (`Live()`, `unplug`); T1 |
| R3 | The close channel was keyed by match key, so a displaced handle's late callback could tear down its successor. | The callback carries its own handle; the manager acts only if it is still the one bound; a stale callback is ignored. The fake's old handle fires after a replug so T5 sees it. | §12 item 2; §14.2 (`unplug`, scenarios); T5 |
| R4 | §12 opened second-collection paths that §3 said to skip, a permanent false "cannot be told apart". | Before `Open`, skip any path whose usage pair is not the one T0 prefers; the guard then sees only true twins. | §12 item 1; T5 |
| R5 | Any save promoted every in-memory row, writing nameless records and breaking "a borrowed light leaves no record". | An in-memory row is written only when it carries a name, at least one line, or a Take over. | §10.5; §14.2 scenarios; T8 |
| R6 | Ignore-versus-409 needed to tell a forgotten id from a vanished one, with no data for it. | Tombstones, in memory, of the ids forgotten since start: a tombstone refuses with 409; any other unknown id is a vanished row, ignored. A Forget before a restart reads as vanished afterwards, which is safe. | §10.5; T8 |
| R7 | (a) the Take over list; (b) the vanished-row wording; (c) `b01`. | (a) offered only for unplugged records; (b) "is no longer listed"; (c) a leading-zero id is malformed, re-issued and logged. | §11.2; §10.5; §10.1; T2, T8 |
