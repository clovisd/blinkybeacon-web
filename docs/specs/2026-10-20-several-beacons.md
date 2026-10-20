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
- A beacon that follows **no line** is yours to drive by hand from the tray menu or the HTTP
  routes, exactly like an unbound tray today.
- A light you plug in for the first time shows up as a new beacon that follows nothing, so it never
  lights for a line nobody assigned it to.

The tray menu gets one entry per beacon, with its state, its lines, and its own Spin, Flash, Stop
and Identify. The top-level Spin, Flash and Stop stay and drive every beacon, as the Companion
module's buttons do. The tray icon shows the busiest light.

The HTTP routes the Companion module uses keep their paths and replies. `POST /spin`, `/flash`
and `/stop` with nothing more drive every beacon; with `?beacon=<id>` they drive one. `GET
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
- **Recommended: the port is the seat.** The row keeps its name, lines and settings; the moved
  light appears as a new beacon that follows nothing; on the settings page the new row offers
  "Take over the settings of: Desk left" in one click. Predictable, and never wrong.
- Alternative: the tray guesses. When exactly one row is unplugged and exactly one new light
  appears, it re-binds the row to the new light by itself. Convenient, and silently wrong the
  moment two lights are swapped between two ports.
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
- **Recommended: a submenu per beacon** ("Desk left — Spinning (Line A, Line C) ▸" opening to
  its status, its lines, Spin, Flash, Stop, Identify), under the top-level Spin, Flash and Stop that
  drive every beacon. The menu library supports submenus and can show or hide items, but cannot
  insert one above an existing item, so the tray pre-creates eight hidden beacon submenus at
  start and shows them as beacons appear (§11.1). Eight beacons is the menu's limit, not the
  app's.
- Alternative: a flat list, "Desk left: Spin / Desk left: Flash / …" for every beacon. No submenu
  clicks, but fifteen rows for three beacons, and the same eight-slot pre-creation.

**D5 · Keep the old one-beacon keys in the config file, mirrored from the first beacon.** The new
file holds a `beacons` list (§10.1). The old top-level keys (`line_number`, `flash_seconds`,
`lobby_flash`, `lobby_flash_seconds`, `pause_side`) can either stay, written from the first beacon
on every save, or go.
- **Recommended: keep them, mirrored.** A `dashboard-v0.8.0` build pointed at the new file then
  still runs the first beacon on its line, so going back a release after trying this one costs
  nothing. The cost is five redundant keys in a file few people open.
- Alternative: drop them. A cleaner file; a downgrade reads `line_number` as missing and binds the
  beacon to line 1.

## 3. Unknowns, and the probe step that settles each

| Unknown | Why it matters | Settled by |
|---|---|---|
| **Does this light report a USB serial number, and is it distinct per unit?** | Decides what a beacon's identity is (§5.3): a serial travels with the light; without one the port is the identity, and D1 applies. Some cheap devices report the same serial on every unit, which is as good as none. | Probe step 2: the `summary` block says `DISTINCT`, `NONE`, `MIXED` or `DUPLICATED`. |
| **Does the Windows path stay the same across a reboot, and change across a port move?** | The port-as-seat rule (§5.3) rests on Windows building a serial-less device's instance id from the hub and port. Read from Windows' own documentation, not measured here. | Probe steps 2 and 3: compare `port key` before and after the move; the owner may also run step 2 again after a reboot. |
| **Can several beacons with the same vendor and product id be held open at once, and does writing to one path light only that light?** | Upstream's library header says it *"cannot differentiate between multiple beacons … due to a hardware limitation"* (`pkg/fsbeacon/usbhid.go:3`). Everything in `hidapi` says paths are per device interface and opening by path is exact (§5.1); the comment most likely means "no serial number", but the lights have to show it. | Probe step 4: `-flash` opens every beacon and holds them open together, then flashes each in turn naming it. One light at a time flashing, in order, settles it. If more than one light flashes at once, **stop and report**: that is the kickoff's first halt condition. |
| **macOS** — whether the location id is stable across replug and reboot, and whether several lights open cleanly through IOKit. | §13. No Mac with a light exists (`docs/PUNCHLIST.md:30`, owner: "not now"). | Unmeasurable until a Mac and a light meet. The probe builds on macOS (its `extra_darwin.go` prints the location id); that file compiled nowhere in this session, so it is unverified. |
| **How fast `hid.Enumerate` is on Windows with a dozen HID devices present.** | Sets how often the tray can look for newly plugged lights (§12). `hidapi` opens every HID interface on the system read-only to read its vendor id (`hid_windows.c:806-864`). | The design keeps today's 10-second retry as the look-up cadence, so nothing rests on it. A later tuning can measure it with the probe's timing. |

## 4. The owner's hands-on steps, in short

The full step-by-step is §14.1 (the probe) and §14.3 (the built feature on three lights). The
probe first:

1. Plug in every beacon, and **quit the tray first**: the tray re-sends its own command to every
   light it holds every five seconds, which would fight the probe's flash (and on a Mac it holds
   each light exclusively, so the probe could not open them at all).
2. Run `beacon-probe.exe` (double-click is fine). Send back the `beacon-probe.txt` it writes
   beside itself.
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

A beacon's **id** is a string with a prefix saying what kind it is:

- `sn:<serial>` when the light reports a serial number, **and** the probe showed the owner's
  units report distinct ones. The light keeps its identity in any port, on any PC.
- `port:<key>` otherwise. On Windows the key is the instance-id segment of the path, lower-cased
  (`6&2a1b3c4d&0&0000`); on macOS it is the location id in hex (`0x14200000`). The port is the
  seat: whatever light sits in it is that beacon (D1).

**The build picks one mode from the probe's file and fixes it as a constant** (task T0, §15).
Mixing the two at run time was considered and refused: an id that depends on what else is plugged
in (serial when unique, port when a twin appears) changes under the operator's feet. In `sn` mode,
a second connected light reporting the same serial as a first is reported in the tray as "cannot be
told apart" and is not opened: loud, never wrong.

### 5.4 Naming, and finding out which light is which

- The operator names a beacon on the settings page (§11.2). Names are free text, trimmed, 1–40
  characters, unique among the configured beacons (the save refuses a duplicate by name), and
  shown everywhere the beacon is mentioned: tray, settings, logs, `GET /beacons`. A beacon that
  has not been named shows as `New beacon` with its id beside it.
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
  id        string        "sn:…" or "port:…" (§5.3); "" for a record not yet matched to hardware (§10.2)
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
pair-watcher reuses every test as it is.

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
the light showed spin, exactly as it is today when a pause lands during a flash: "does not go back
to flashing when the pause lifts" (`BEACON-DASHBOARD.md:351-354`). Nothing about the guards
changes.

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
(`lines.go:131`). Compare the dashboard's own measurement when the read API was designed: four
pollers at 2 Hz for 10 s, 84 polls, **0 errors**, p95 8.3 ms; and "ten companions ≈ 20 req/s …
negligible against live GSI ingest" (DOTA-DASHBOARD
`docs/superpowers/specs/2026-08-12-s79-beacon-companion-design.md` §3.2, §4.3). The worst row
here, 16 req/s, sits under what was already judged negligible, and the realistic rows are a
tenth of it. The fan-out does land its requests together at each tick, a burst of up to eight;
the dashboard answers from memory on a threadpool (S79 §4.2 "cache-read only, no disk on the
request path"), so a burst of eight is eight `/ping`-class reads. **No dashboard change is needed
for load**, and §16 asks for none.

## 9. The HTTP API

### 9.1 The three control routes

Paths and replies unchanged (`server.go:25-28`, `:76-131`). What is added is an **optional
`beacon` query parameter**:

```
POST /spin                      drives every connected beacon
POST /spin?beacon=<id>          drives that one
POST /spin?beacon=<id>&beacon=<id>   drives those
```

`<id>` is the beacon's id (`sn:…` / `port:…`) **or its exact name**; the id is tried first. The
handler reads it with `r.ParseForm()` + `r.Form["beacon"]`, which Go fills from the query string
and, for a POST with a form body, from the body too; the documented way is the query string.

Why a query parameter and not a form field: the Companion module posts with **no body and no
content type** (`src/index.ts:44`, `fetch(url, {method: 'POST'})`), so a form field would make
every caller grow a body and a header, while `?beacon=` is one more token on the URL the module
already builds; `curl -X POST "http://127.0.0.1:1337/spin?beacon=Desk%20left"` is a one-liner for
a script (a space in a name travels as `%20`); and the token (S18) will ride a header, leaving the query string to say *which*.

Replies:

| Call | Reply |
|---|---|
| no `beacon`, at least one beacon connected | `200 {"state", "connected"}` for the beacons as a whole, after the command (the §9.2 aggregate). A beacon whose command fails is dropped for reconnection, as today (`server.go:86-89`), and the rest go on; the reply is still 200 because the aggregate is true of what remains. |
| no `beacon`, none connected | `503 {"error": "beacon not connected"}` — today's text (`server.go:83`) |
| `beacon=<id>`, known and connected | `200 {"state", "connected"}` **about that beacon** |
| `beacon=<id>`, known and unplugged | `503 {"error": "beacon not connected"}` |
| `beacon=<id>`, unknown | `404 {"error": "no such beacon"}` |
| several ids, mixed | each resolved as above; one unknown fails the whole call with 404 before anything is driven |
| wrong method | `405`, as today |

The existing tests in `server_test.go` pass unchanged (the freeze, in test form).

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
    {"id": "port:6&2a1b3c4d&0&0001", "name": "Desk left",  "connected": true,  "state": "spin",
     "lines": [1, 3], "watch": "ok"},
    {"id": "port:6&2a1b3c4d&0&0002", "name": "Desk right", "connected": false, "state": "idle",
     "lines": [2],    "watch": "ok"},
    {"id": "port:6&2a1b3c4d&0&0003", "name": "",           "connected": true,  "state": "idle",
     "lines": [],     "watch": "off"}
  ]
}
```

- `v` is the shape's version, `1`. `beacons` is in config order, then newly seen unnamed beacons
  in the order they were found; it includes configured beacons that are unplugged
  (`connected: false`) so a picker can offer a beacon that is temporarily off.
- `id`, `name` as §5.3/§5.4 (`name` is `""` until the operator names it). `state` is the beacon's
  own, one of `idle`/`spin`/`flash`. `lines` is what it follows. `watch` is `ok` when every line it follows
  is `ok`, otherwise the status of its first line (in `lines` order) that is not `ok`, in the
  tray's own words (`watcher.go:95-109`: `off`, `no-token`, `ok`, `lost`, `rejected`, `no-line`,
  `stopped`, `no-game-yet`, `quiet`); `off` for a beacon with no lines.
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
  "beacons": [
    {
      "id": "port:6&2a1b3c4d&0&0001",
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
ignored by this one whenever `beacons` is present.

Per-beacon values load with the same forgiveness as today's top-level ones (`config.go:151-167`):
a missing or out-of-range number lands on the documented default; an unknown `pause_side` loads
as `both`; a line number under 1 is dropped from `lines`; duplicates in `lines` are collapsed. The
settings page stays stricter (`settings_handler.go:260-266`). The file stays owner-only and written
atomically (`config.go:186-227`), unchanged.

### 10.2 Loading a `dashboard-v0.8.0` file

A v0.8.0 file (`BEACON-DASHBOARD.md` §7's sample) has no `beacons` key. Load rule:

1. Parse as today. If `beacons` is absent or empty, **synthesise one record** from the top-level
   keys: `id: ""`, `name: "Beacon 1"`, `lines: [line_number]`, light settings from the four
   top-level keys. Nothing is written yet; loading stays a pure read.
2. A record with `id: ""` is **unmatched**: it binds to the first connected beacon that no other
   record claims, in enumeration order, the moment the device manager (§12) sees one. On binding,
   the record takes that beacon's real id and the config is saved, so the next start matches by
   id. With one light plugged in this is today's behaviour exactly: the first (only) beacon found
   is driven by the configured line with the configured settings, and the HTTP routes, tray and
   settings page read as they do today.
3. With several lights plugged in at that first start, the unmatched record binds to the first one
   enumerated and the others appear as new beacons following nothing (§10.4). The operator sorts
   it on the settings page, with Identify to tell them apart; the settings row shows each
   beacon's id, and `Beacon 1` is whichever light was enumerated first.

The build's loader test (task T2; a parser, plan-gate rule 3) ends with a read-only smoke on a
real-shape v0.8.0 fixture with keys omitted the way old files omit them (a v0.5.0 file has none
of the light keys; a v0.7.0 file has no `poll_interval_ms`), and a v0.8.0 file with every key.

### 10.3 A beacon unplugged, and back

The record stays. The beacon's tray row and `GET /beacons` say `unplugged` (`connected: false`).
Its (beacon, line) watchers keep running: the flash deadline and the guards are the record's, not
the handle's, exactly as the single watcher survives a USB drop today (the loop never looks at the
beacon except in `applyState`, which skips while disconnected, `watcher.go:553-555`). When the
same identity reappears it is re-opened, re-bound to its record, and the wanted state is applied
on the next tick: a replug mid-pause resumes spinning. (`docs/PUNCHLIST.md:31` records that this
has never been checked on hardware for one beacon; §14.3 step 9 checks it for several.)

### 10.4 A beacon seen for the first time

It gets an **in-memory** record: its real id, `name: ""`, `lines: []`, default light settings. It
follows nothing, so no line can light it; the hand controls and a call without a beacon choice do
drive it (the owner: existing buttons "drive every beacon at once"). It is listed in the tray, on
the settings page and in `GET /beacons`. **Nothing is written to the config** until the operator
names it or gives it a line on the settings page; a light borrowed for an afternoon leaves no
record behind. Forgetting a configured beacon is a button on its settings row, enabled while it is
unplugged.

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
New beacon (port:6&2a…&0&0003) — Idle · Unbound ▸ …
─────────────────────────
Start with Windows
Quit
```

- `getlantern/systray` v1.2.2 has `AddSubMenuItem` (`systray.go:142`), `Hide` (`:189`) and `Show`
  (`:194`), and items can be added after the menu is up, but only **appended at the bottom**:
  there is no insert. So `onTrayReady` creates **eight** beacon submenus, hidden, between the
  all-beacon controls and the startup row, and the 500 ms sync loop (`tray.go:69-141`) shows,
  hides and retitles them as beacons come and go. Eight is the menu's limit; a ninth beacon is
  still driven and listed by HTTP and the settings page, and the top row says `9 beacons (8 shown
  here)`.
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
│   Name  [Desk left            ]   id port:6&2a1b3c4d&0&0001                     │
│   Lines [x] Line A · running   [ ] Line B · stopped   [x] Line C · running      │
│   Draft-End Flash [15] s   New-Lobby Flash [ ] on, [10] s   Spin For Pauses From [Both sides ▾] │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ○ Desk right                      unplugged                         [Forget]   │
│   Name  [Desk right           ]   id port:6&2a1b3c4d&0&0002                     │
│   Lines [ ] Line A   [x] Line B   [ ] Line C                                    │
│   Draft-End Flash [15] s   New-Lobby Flash [x] on, [10] s   Spin For Pauses From [Radiant only ▾] │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ● New beacon                      plugged in · idle                 [Identify]  │
│   Name  [                     ]   id port:6&2a1b3c4d&0&0003                     │
│   Take over the settings of  [ — ▾ | Desk right ]                               │
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
  rejected`, `unreachable`, `no token`).
- **Form fields** are indexed, not keyed by id (ids contain `&` and `\`): `b0.id` (hidden),
  `b0.name`, `b0.lines` (multi-value), `b0.flash_seconds`, `b0.lobby_flash`,
  `b0.lobby_flash_seconds`, `b0.pause_side`, `b0.takeover` (the id of the record to absorb).
  Validation per beacon is today's (`formSeconds`, `validPauseSide`), naming the beacon and
  field in the refusal: `Desk right: flash_seconds must be …`. Names are escaped on render
  (`html.EscapeString`, as every operator string is on this page) and refused when duplicated.
- **Identify** and **Forget** are buttons of the same form with `name="action"` values
  `identify:<index>` and `forget:<index>`; the handler acts and redirects back to `/settings`
  without saving the rest of the form, so a half-edited page is not committed by an Identify
  click. Both carry the CSRF token by being in the form.
- **Take over** moves a record's name, lines and light settings onto the new beacon's id and
  deletes the old record, in one save (D1).

## 12. Hot-plug and failure

Today: one goroutine opens the first beacon, retrying every 10 s (`main.go:85-110`); the
library's keepalive worker writes the current command every 5 s and calls `OnError` on a failed
write (`pkg/fsbeacon/usbhid.go:70-80`), which `main.go:95` turns into `SetBeacon(nil)`; the
goroutine then closes the handle and goes back to retrying.

New: a **device manager** goroutine owning a map `id → open handle`:

1. Every 10 s (today's cadence, kept): `Enumerate(vid, pid)`. For each path not already open:
   `OpenPath`, compute the id (§5.3), bind to the record with that id, or to an unmatched record
   (§10.2), or create an in-memory record (§10.4); `StartIdleWorker`; set `OnError` to mark **that
   beacon** unplugged and schedule its close.
2. A failed command on a beacon, from any caller (tray, HTTP, watcher), marks that beacon
   unplugged, as each caller does today for the one beacon (`main.go:127-128`, `server.go:87`,
   `watcher.go:566-571`); the others are untouched.
3. Each beacon is its own `FarmBeacon` with its own mutex, worker goroutine and handle
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
error)` and `OpenPath(path string) (*FarmBeacon, error)`, and `FarmBeacon` keeps the
`DeviceInfo` it was opened from. `OpenFarmBeacon()` stays as it is for the upstream CLI
(`cmd/fsbeacon`). The upstream comment at `usbhid.go:3` is corrected once the probe has shown
what it meant.

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
  per-attach and never stored (§5.2). The port-key function is therefore platform-specific, the
  only platform-specific line in the identity code.
- **Bundle:** no new entitlement. Opening a non-keyboard HID device needs no Input Monitoring
  permission; the existing app already opens one.

Nothing here can be run in this session (`cgo` needs Cocoa and IOKit), and no Mac has ever been
run against a light (`docs/PUNCHLIST.md:30`). Verification stays what it is for every release:
the `build` workflow's macOS jobs green on the pushed branch (`.github/workflows/build.yml`,
`macos` job), the artifact unzipped and listed. The hands-on macOS check waits with the punchlist
item. The probe's `extra_darwin.go` is ten lines against a signature read from the library source;
it is marked unverified in the retro.

## 14. Testing

### 14.1 The probe, step by step (the owner, once)

1. Plug in every beacon. **Quit the tray** from its menu first: only one program should hold a
   beacon (§5.1 says why on each platform).
2. Run `beacon-probe.exe`. Double-clicking is fine: it writes its findings to the window and to
   `beacon-probe.txt` beside itself, and waits for Enter. Send back `beacon-probe.txt`.
3. Move one beacon to another USB port and run it again. It appends to the same file, so the
   before and after sit together. (A run after a reboot, same ports, is one more useful pair.)
4. Run `beacon-probe.exe -flash` from a terminal (`cmd` or PowerShell in the exe's folder). Each
   light flashes for two seconds in turn and the window names which one; then all stop. Note
   which physical light answered to which `port key`, if it is not obvious.

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
type deviceSource interface {
    Enumerate() ([]fsbeacon.DeviceInfo, error)
    Open(path string) (Beacon, error)   // the local Beacon mirror, state.go:19-24
}
```

The real source is the library, behind `//go:build windows || darwin` as `main.go` is today
(`main.go:1`). The fake, `fakeBus`, is a test type in the tray package beside `mockBeacon`
(`server_test.go:10-21`) and `countingBeacon` (`watcher_test.go:508-516`):

- `plug(serial, portKey)` adds a device; `unplug(portKey)` removes it from enumeration and makes
  its open handle's next `Spin`/`Flash`/`Stop` return an error and fire `OnError`;
  `move(portKey, newPortKey)` is an unplug then a plug with the same serial and a new port;
  `failNext(portKey)` makes one command fail without unplugging, for the "one beacon failing"
  case.
- Each fake handle counts commands, as `countingBeacon` does, and records the order they came in.
- The fake's identity mode is set per test (`sn` or `port`), so both branches of §5.3 are
  exercised whatever the probe decides.

Scenarios the suite pins: three beacons bound to three records; unplug one and the other two keep
their state and take their next commands; replug and the record re-binds and the wanted state is
re-applied (a pause resumes spinning); a port move yields one unplugged record and one new
unassigned beacon, and `takeover` merges them; a first-seen beacon follows nothing and is driven
by a bodiless `POST /spin`; an unmatched v0.8.0 record binds to the first light and the config is
saved with its id; in `sn` mode a duplicate serial is refused and reported.

### 14.3 The owner's hands-on plan, three or more real lights

Written for someone at a desk with the built `BB-DASH.exe`, three lights, and a dashboard with at
least two official lines (call them Line A and Line B). Each step says what to look for.

1. **Upgrade in place.** Quit the old tray. Put the new `BB-DASH.exe` beside the old
   `blinkybeacon-config.json`. Plug in **one** light only. Start the tray. *Expect:* the tray behaves
   exactly as before: `● Beacon: Idle`, the bound row names the old line, the light reacts to a
   pause on it. Open Settings: one beacon row, `Beacon 1`, following the old line, with the old
   light settings.
2. **Name it.** Type `Desk left`, Save. *Expect:* the tray's row says `Desk left`; the config file
   now has a `beacons` list with an `id`.
3. **Plug in the second light.** Within ten seconds, *expect:* a `New beacon` row in the tray
   (`Unbound`) and on the settings page, following nothing. The dashboard pausing Line A spins
   Desk left only.
4. **Identify.** Click Identify on the new row. *Expect:* the second light flashes for two
   seconds; the first does not. Name it `Desk right`, tick Line B, Save.
5. **Two lines, two lights.** Pause Line B on the dashboard. *Expect:* Desk right spins, Desk left
   dark. Pause Line A too. *Expect:* both spin. Unpause A. *Expect:* only Desk right spins.
6. **One light, two lines.** On Desk left tick Line A **and** Line B, Save. Pause Line B.
   *Expect:* both lights spin (Desk left for B, Desk right for B). End a draft on Line A while B is
   paused. *Expect:* Desk left keeps spinning (the pause outranks the flash; §7.2 row 2), Desk
   right keeps spinning. Unpause B. *Expect:* Desk left goes dark, not back to flashing (the flash
   was spent under the pause); Desk right dark.
7. **Different settings on one line.** Set Desk left's draft flash to 5 s and Desk right's to 20 s;
   both follow Line A only. End a draft on A. *Expect:* both flash; Desk left stops after about
   5 s, Desk right after about 20.
8. **The Companion buttons.** With the module's existing Spin button, *expect:* **both** lights
   spin; Stop stops both. From a terminal, `curl -X POST "http://127.0.0.1:1337/flash?beacon=Desk%20left"`. *Expect:* only Desk left flashes; `curl http://127.0.0.1:1337/status` says
   `{"state":"flash","connected":true}`; `curl http://127.0.0.1:1337/beacons` lists both with
   their states. (The watcher corrects both on its next tick if they follow lines; untick the lines
   first for a longer look.)
9. **Unplug mid-pause.** Pause Line B; Desk right spins. Pull its USB. *Expect:* within five
   seconds the tray says `Desk right — Unplugged`; Desk left is untouched (still dark, or still
   spinning if it follows B). Plug it back into the **same** port. *Expect:* within ten seconds
   it is `Desk right` again and **spins**, because B is still paused.
10. **Port move.** Quit nothing. Move Desk right to a **different** port. *Expect, if the probe
    found no serial numbers:* `Desk right — Unplugged` stays, and a `New beacon` appears. On the
    settings page the new row offers `Take over the settings of: Desk right`; choose it, Save.
    *Expect:* one row, `Desk right`, in the new port, following B. *If the probe found serial
    numbers:* nothing changes but the port; it is `Desk right` as soon as it is found.
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
| T0 | **Fix the identity mode from the probe's file.** One constant (`sn` or `port`) and `beaconID(info, dev)`, with the Windows and macOS port-key functions. | new `beacon_id.go`, `beacon_id_windows.go`, `beacon_id_darwin.go` | table test over sample paths and location ids, both modes; the probe's own lines as fixtures | parser (of the path) |
| T1 | **Library: enumerate and open by path.** `fsbeacon.Enumerate()`, `fsbeacon.OpenPath()`, `FarmBeacon.Info`; `OpenFarmBeacon()` unchanged. | `pkg/fsbeacon/usbhid.go` | `go vet` under `GOOS=windows` and `darwin` via CI; `beacon_check.go`'s compile-time assertion; the existing hardware test | third-party API |
| T2 | **Config: the `beacons` list, migration, mirror.** §10.1–10.2, D5. | `config.go`, `config_test.go` | load of a v0.5.0, v0.7.0 and v0.8.0 file each yields one unmatched record with the right values; save-then-load round trip; mirror keys written from `beacons[0]`; forgiveness per field; **read-only smoke on the real-shape fixtures with omitted keys** | parser |
| T3 | **The beacon registry and the aggregate.** Records, handles, per-beacon state, `identifyUntil`; `aggregateState`, `aggregateConnected` (§9.2). Replaces `AppState`'s single beacon. | `state.go` → `beacons.go`, `state_test.go` | property test: for every multiset of up to six beacon states, the aggregate equals the rank maximum; connected = any; **pinned values: the §9.2 table**; the existing `state_test.go` cases ported | numeric |
| T4 | **Per-pair watchers and the combine.** §7; the loop's fan-out (§8.1). `Watcher` unchanged. | `watch_loop.go`, new `combine.go`, tests | the §7.2 table, rows 1–10, pinned; property sweep over all 3^6 answer combinations; deadlines (rows 4, 7); the 401 backoff shared and the 404 per line; one fetch per distinct line (count on the stub) | numeric |
| T5 | **The device manager with the fake bus.** §12, §14.2. | `main.go` (the USB goroutine), new `devices.go`, `fakebus_test.go` | the §14.2 scenarios; the "one fails, two go on" case | third-party API |
| T6 | **HTTP: `?beacon=`, `GET /beacons`, the aggregate `/status`.** §9. | `server.go`, `server_test.go`, the API block in `settings_handler.go:86-106` | **every existing `server_test.go` test passes unchanged**; the §9.1 reply table; `/beacons` shape golden; `/status` ignores `?beacon=`; 404 before any command on a mixed call | HTTP contract |
| T7 | **The tray menu and icon.** §11.1, the S14 hook. | `tray.go`, `icons.go` | the label functions table-tested as today's are; the eight-slot show/hide logic unit-tested on a fake menu; the rest by hand (§14.3) | — |
| T8 | **The settings page.** §11.2. | `settings_handler.go`, `settings_handler_test.go` | form round trip for two beacons; indexed fields; duplicate name refused; names escaped; Identify and Forget act without saving the form; takeover; the line checkboxes from the cached list and the comma-separated fallback; CSRF on every action | — |
| T9 | **Docs.** `BEACON-DASHBOARD.md` §1, §2 (the picker is now per beacon), §4, §5 (the tray rows), §7 (the config sample), §8 (drop "only one line at a time"); README's API block; the settings page's API text. | docs | a reviewer reads them against the shipped behaviour, step by step with §14.3 | — |
| T10 | **Hands-on and CI.** §14.3 on the owner's lights; the `build` workflow green for Windows and both macOS legs. | — | the owner's report; the artifacts listed | — |

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
  `hid_windows.c`, `hid_darwin.c`, `hidapi.h`; `systray@v1.2.2/systray.go`.
- Companion module read at `~/projects/companion-module-blinkybeacon` (v1.1.0): `src/index.ts`,
  `src/actions.ts`, `src/feedbacks.ts`.
- Not measured here, and said so where it is used: Windows instance-id behaviour across reboot
  and port move (§5.2, Windows documentation; probe step 3); macOS location-id stability (§13);
  `hid.Enumerate` cost on Windows (§3); upstream's "hardware limitation" (§3, probe step 4).
