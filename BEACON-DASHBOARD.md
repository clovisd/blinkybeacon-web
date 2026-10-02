# Beacon ↔ Dashboard — user guide

The tray app can watch a LIVE Dashboard line and drive the beacon by itself:

- **The draft ends → the beacon flashes red for 15 seconds.**
- **The game is paused → the beacon spins amber, for as long as the pause lasts.**
- **Anything else → the beacon is dark.**

Since **v0.6.0** all three of those are yours to change: how long the draft
flash runs, whether a **new lobby** flashes too and for how long, and whether a
pause spins the light for **both sides or only one**. See
[section 4](#4-tuning-the-light). The defaults are exactly the behaviour above,
so upgrading and never opening Settings changes nothing.

**"The draft ends" is read off the dashboard, whichever way it can see it —
and no cfg needs reinstalling for that to work.**

The beacon flashes at the **end of the pick/ban phase as the dashboard sees
it**: the moment the game state leaves hero selection, which is the same moment
the dashboard's own action log records as **"Draft → Strategy"**. That is
derived from data every line already sends, so **every existing install flashes,
with nothing to reinstall and nothing to configure.**

Where a line *does* publish draft detail, the beacon uses the sharper moment
instead — the **last final pick** of the pick/ban phase, the owner's ruling:

> "'Draft ended' means when all picks and bans have completed, not when players
> have picked their assigned hero from the drafted bunch. We want to flash for
> 15 seconds right when the last final pick is done in the pick/ban phase."

**Whichever of the two comes first flashes, once per match.** The tray says
which one is in play on that line — `draft timing: picks` or
`draft timing: game state`. Both flash; neither is broken.

> Optional, not required: reinstalling the tracked PC's cfg from its install
> link on a dashboard running v3.99.0 or later makes the flash land at the last
> final pick itself rather than at the end of hero selection. In Captains Mode
> that is the difference between the pick and the choose-your-hero stretch that
> follows it. Everything works without doing this.

It polls the dashboard every 2 seconds (you can change that — see
[section 4](#4-tuning-the-light)) over the dashboard's read API,
`/api/v0/lines/<N>`, using an **API token** that a dashboard admin mints for
you. Without that token the watcher does not poll at all — see
[section 3](#3-get-a-token-and-paste-it-in).

---

## 1. Get the app running

1. Download `BB-DASH.exe` from the release, or build it yourself —
   see [BUILD_WINDOWS.md](BUILD_WINDOWS.md). On a Mac it is
   `BB-DASH-macos-arm64.zip` (Apple Silicon) or `BB-DASH-macos-amd64.zip`
   (Intel), each holding `BlinkyBeacon.app` — see the [README](README.md#download).
   Every file has a `.sha256` beside it.
2. Put it in a folder of its own. It writes `blinkybeacon-config.json` next to
   itself, so don't leave it in `Downloads`.
3. Plug the beacon in over USB.
4. Double-click the `.exe`. A tray icon appears (bottom-right, possibly under
   the `^` arrow). There is no window — that's normal.

> **Upgrading on Windows from a release that shipped `blinkybeacon-tray.exe`?**
> Only the file name has changed.
>
> 1. Put `BB-DASH.exe` in the **same folder** as the old program. Your settings
>    live in `blinkybeacon-config.json` beside it, and that is where the new
>    program looks for them.
> 2. Quit the old program from its tray menu, and delete `blinkybeacon-tray.exe`.
> 3. Start `BB-DASH.exe`. If **Start with Windows** was on, tick it again: it
>    shows unticked until you do, because Windows is still set to start the
>    old file.

The tray menu's top line tells you whether the beacon was found:

```
● Beacon: Idle          <- found, nothing happening
● Beacon: Disconnected  <- not found; check the cable
```

## 2. Point it at your dashboard

1. Right-click the tray icon → **Settings…**. Your browser opens on the
   settings page.
2. Fill in **Dashboard URL** — the address you use to open the dashboard,
   e.g. `http://192.168.1.50:8080`. Host and port only; no path.
3. **Pick the line from the list.** Once a token is saved (next section), the
   settings page asks your dashboard which lines it has and offers them by
   name — `Line C · running`, `Line D · stopped`. Choose yours; there is no
   number to look up any more.
   > **No list?** The page falls back to a **Line Number** box and says why in
   > one line: `token rejected`, `unreachable`, or `no token`. Type the `N`
   > from the dashboard's own URL for that line —
   > `http://192.168.1.50:8080/line/`**`2`**`/` means `2`. The number is the
   > line's own id, **not** its position in the list.
   >
   > A line shown greyed out as `no number yet (dashboard < v3.98.0)` cannot be
   > picked: that dashboard has not given its lines numbers yet, and there is
   > nothing for the beacon to bind to. Upgrade the dashboard.
4. Leave **Dashboard API Token** for the next section, and click
   **Save & Apply**.

The list is read by the app itself, not by your browser, so it works across the
internet with no dashboard-side setup — and your token never reaches the
settings page.

The watcher picks changes up on its next poll — no restart. Until a token is
saved the tray will say `○ Dashboard: no token set`, and nothing is polled.

To turn the watcher off again, blank the Dashboard URL and save (or forget the
token — see below). **The beacon goes dark as it lets go**, so it can never be
left spinning on a pause nobody is watching any more. After that it does
nothing until you drive it by hand from the tray menu (Spin / Flash / Stop),
which stays available at all times.

## 3. Get a token, and paste it in

The dashboard's read API is not open to the public, so the beacon needs a
credential of its own. It is a single token, and it only ever grants **read
access to the official lines' game state** — no chat, no rosters, no ability to
change anything.

**On the dashboard** (this part needs a dashboard **admin**):

1. Open the dashboard → **Settings** → **Integrations**.
2. Mint a token for the beacon.
3. **Copy it immediately.** It is shown once and never again. If you lose it,
   mint a new one — that is cheaper than trying to recover it.

**In the tray app:**

4. Right-click the tray icon → **Settings…**
5. Paste it into **Dashboard API Token** and click **Save & Apply**.

Within a couple of seconds the tray should read `● Dashboard: watching`.

Some things worth knowing about the token:

- **It is never shown back to you.** Re-open the settings page and the field is
  empty, with the hint "a token is saved". That is deliberate: the settings
  page has no password on it and can be bound to `0.0.0.0`, so anything printed
  on it is readable by anyone who can reach the app. Leave the field blank when
  you save and the saved token is kept; paste a new one to replace it; tick
  **Forget the saved token** to remove it.
- **It is stored in plain text** in `blinkybeacon-config.json` next to the
  `.exe`, because the app has to send it on every poll. The file is written
  owner-only (`0600` on Linux/macOS; on Windows the folder's own permissions
  apply). Treat it the way you would treat a password: don't put the folder on
  a shared drive, and don't paste the file into a bug report.
- **It never appears in the log**, and never travels in a URL — only in an
  `Authorization` header, which proxies do not log by default.
- **Changing the Dashboard URL clears it.** A token minted by one dashboard is
  not a credential for another, so pointing the app at a different address
  starts it unbound; paste the token for the new dashboard. Changing the *line
  number* keeps it — that's the same dashboard.
- **Saving settings requires the page the app served you.** If you get
  "this settings form is stale", reopen **Settings…** from the tray menu and
  save again. That check is what stops a web page you happened to visit from
  quietly repointing the app — and your token — at somebody else's server.
- **Revoking it on the dashboard is instant.** The beacon goes dark on the very
  next poll, within about 2 seconds at the default cadence.

## 4. Tuning the light

Right-click the tray icon → **Settings…** → **Beacon Light**. Three controls,
all optional; **Save & Apply** and the watcher picks them up on its next poll.

| Setting | Default | What it does |
|---|---|---|
| **Draft-End Flash** | `15` seconds | How long the beacon flashes when the draft ends. 1–600. |
| **New-Lobby Flash** | off, `10` seconds | Flash again, separately, when a **new lobby** appears on the line. 1–600. |
| **Spin For Pauses From** | Both sides | Whether a pause spins the light for everyone, or only for **Radiant** or only for **Dire**. |

**What "a new lobby" means.** The tray sees a **fresh match id** on the line
while the game is still **before or in the draft** — Dota's `INIT`,
`WAIT_FOR_PLAYERS_TO_LOAD`, `WAIT_FOR_MAP_TO_LOAD` or `HERO_SELECTION`. That is
the moment a game has been made and the players are filing in. It fires **once
per match**; the next match arms it again.

It deliberately does **not** fire when the tray joins a game already in
progress — starting the app, or rebinding it to another line, mid-match is not
a lobby opening, and announcing one would be a light for a moment that passed
before anyone was watching.

A lobby flash and a draft-end flash in the same match are **both allowed** —
they are separate announcements of separate moments. If they ever overlap, the
beacon simply flashes until the later of the two finishes.

> **The side filter needs a dashboard that can attribute a pause.** Picking
> Radiant or Dire compares against the dashboard's `pause_party`, published
> from **v3.101.0** onwards. On an older dashboard nothing is attributed, so
> **Radiant and Dire never spin for a pause at all** — leave it on *Both sides*
> until the dashboard is upgraded. The settings page says so on the control.
>
> Even on a new enough dashboard, an **admin pause** and one the dashboard
> could not attribute count as neither side: they spin only under *Both sides*.

**How often it asks the dashboard** is set in the config file only — there is
no control for it on the settings page:

| Key | Default | Range |
|---|---|---|
| `poll_interval_ms` | `2000` (2 seconds) | `500`–`10000` milliseconds |

Lower means the light reacts sooner — a pause or the end of the draft shows on
the beacon within about one interval — and the dashboard gets more requests:
at `500` the beacon asks four times as often as at the default. Higher is
gentler on the dashboard and slower to react. A missing key, or a value outside
the range, runs at the default. However low it is set, the watcher never has
more than one poll out at a time.

Type the value as a bare whole number — `"poll_interval_ms": 750`, no quotes.
A file the app cannot read — a quoted number such as `"750"`, a decimal such as
`750.0`, a missing comma — makes **every** setting fall back to its default,
the dashboard URL and token included. Fix the file before you save from the
settings page, or the save writes those defaults over it.

The file is read when the app starts, and again whenever the settings page
saves; a save keeps whatever value the file holds. See
[section 7](#7-details-worth-knowing) for where the file is.

## 5. What each light means

| Beacon | Means |
|---|---|
| **Flashing red** | The pick/ban phase just ended — the draft is done. 15 seconds unless you changed it. |
| **Flashing red, at the top of a game** | A new lobby appeared on the line — only if you turned that on; see [section 4](#4-tuning-the-light). |
| **Spinning amber** | The game is paused, right now. It stops when play resumes. If you picked a side, only that side's pauses. |
| **Dark** | Nothing is happening — **or** the dashboard feed is gone. Check the tray. |

Dark is deliberately ambiguous at the beacon, so the tray menu always spells
out which one it is:

The tray's top rows also name **which line** the beacon is following, so you
never have to open Settings to check:

| Tray line | Means |
|---|---|
| `Bound: Line C` | Following the line the dashboard calls Line C. |
| `Bound: line 3 (unnamed)` | Bound to line 3, but no poll has come back yet, so we only know the number. |
| `Unbound` | No dashboard URL set — the watcher is off and the beacon is yours to drive by hand. |

And below it, what the feed is doing on that line:

| Tray line | Means |
|---|---|
| `○ Dashboard: not configured` | No dashboard URL set. The watcher is off. |
| `○ Dashboard: no token set` | URL set, but no API token. **Nothing is being polled at all** — see [section 3](#3-get-a-token-and-paste-it-in). |
| `● Dashboard: watching · draft timing: picks` | Polling fine, feed is fresh. This line publishes draft detail, so the flash lands at the **last final pick**. Trust the light. |
| `● Dashboard: watching · draft timing: game state` | Polling fine, feed is fresh. This line publishes no draft detail, so the flash lands when the game state **leaves hero selection**. **Nothing is wrong and nothing needs reinstalling** — see [section 6](#6-if-it-isnt-working). |
| `▲ Dashboard: token rejected` | The dashboard refused the token: it was revoked, expired, or mistyped. Mint a new one and paste it in. |
| `▲ Dashboard: line 2 not found` | Token accepted, but that line number doesn't exist (or isn't an official line). Reopen Settings and pick the line from the list. |
| `○ Dashboard: idle — no game data yet` | Dashboard reached and the line is running, but it has never heard from Dota since it started. **Nothing is wrong** — there is just no game on this line yet. |
| `▲ Dashboard: feed went quiet (45s)` | The line WAS receiving game data and has stopped. This is the real "lost feed": the dashboard is fine, Dota has gone silent. Check the tracked PC. |
| `▲ Dashboard: unreachable` | Can't reach the dashboard at all, or it answered with something we can't read. **The beacon is dark and is telling you nothing.** |
| `○ Dashboard: line not running` | Dashboard reached, but that line isn't accepting game data. |

All eight of the non-`watching` lines mean the same thing at the beacon:
**dark**. The tray is the only place that tells you which one you are looking
at.

Those last three used to be one line that said `FEED LOST` for all of them. An
idle line between games is not a failure, and calling it one taught everybody to
ignore the words. They are three separate sentences now: **idle** is nothing to
do; **went quiet** is the tracked PC; **unreachable** is the dashboard or the
network.

The rule the watcher follows: **never a confident wrong light.** If it cannot
see the game, it goes dark rather than leaving the beacon spinning on a pause
that may have ended minutes ago.

## 6. If it isn't working

**`▲ token rejected`**

The dashboard got the request and said no. The token is wrong, was revoked, or
has expired. There is no way to tell which apart — the dashboard deliberately
gives the same answer to all three, so that guessing at tokens tells an attacker
nothing.

- Mint a fresh one (dashboard → Settings → Integrations) and paste it in.
- Check you pasted the whole thing, with no leading or trailing characters.
- After a rejection the watcher **slows down to one poll every 30 seconds**, so
  it isn't hammering the site with a dead credential. That means it can take up
  to half a minute after pasting a good token before the tray flips back to
  `watching`. It will get there on its own; no restart needed.

**`▲ line N not found`**

The token worked and the dashboard answered — there is just no such line. It
must be an **official** line, and the number is the one in the dashboard's own
URL for it, not its position in the list.

**`▲ unreachable` immediately after setting it up**

- Wrong URL or port. Open the same URL in a browser on this PC — you should see
  the dashboard. If the browser can't reach it, nor can the beacon.
- The dashboard doesn't have the read API. `unreachable` (rather than
  `token rejected`) on a URL you can open in a browser usually means the
  dashboard is older than the `/api/v0/lines/` route. Update the dashboard.

**`● watching · draft timing: game state`**

Not a fault, and **not something to fix**. It means this line publishes no
per-pick draft detail, so the beacon flashes when the game state leaves hero
selection instead — the same moment the dashboard's action log calls
"Draft → Strategy". The flash, the pause light and everything else work
normally.

If you would rather have the flash at the last final pick itself, and the
dashboard is on v3.99.0 or later, re-run the **tracked PC's** install link from
the dashboard so its Dota cfg is rewritten. That is a refinement, not a repair.

**The beacon didn't flash at the draft**

- Check the tray said `● watching` at the time — any other line means it wasn't
  watching that line when the draft ended.
- The tray only flashes on the **change**: "picks outstanding" → "all picks in",
  or hero selection → whatever comes after it. If the tray was started (or
  retargeted, or lost the feed) after the draft had already ended, it never saw
  the change and deliberately stays dark — announcing a moment that has already
  passed is worse than saying nothing.
- **One flash per match**, so if it already flashed at the last pick it will not
  flash again when hero selection ends a moment later. That is the same flash,
  not a missed one.

**Beacon dark, tray says `watching`, and a game is clearly on**

That's correct behaviour — nothing is happening. It only lights for a draft
ending and for a pause.

**Nothing lights at all, even by hand**

Tray says `● Beacon: Disconnected` → it's the USB side, not the dashboard.
Re-seat the cable; it reconnects on its own within a few seconds.

**The flash was missed**

If the dashboard feed drops out across the moment the draft ends, the watcher
does **not** fire the flash late when it comes back. A flash 90 seconds after
the fact is worse than no flash. The pause light, by contrast, always
self-corrects — it reflects the current state on every poll, so restarting the
app mid-pause immediately spins.

## 7. Details worth knowing

- **Polls every 2 seconds** unless `poll_interval_ms` says otherwise, so the
  light can lag reality by up to one interval — ~2 seconds at the default.
- **The feed counts as quiet** if the dashboard hasn't heard from Dota for more
  than 30 seconds. During a legitimate pause Dota still checks in about every 10
  seconds, so a real pause is never mistaken for a dead feed.
- **A pause outranks the flash.** If a pause begins during a flash — either
  kind — the beacon switches straight to spinning, and does not go back to
  flashing when the pause lifts. A pause your **side filter excludes** is not a
  pause as far as the light is concerned, so it outranks nothing.
- **The draft flash fires once per match, whichever trigger comes first.** The
  last pick and the end of hero selection are two views of the same moment; the
  first one seen spends the match's flash and the second is ignored. "All picks
  in" also stays true for the rest of the game, so later polls don't re-flash.
  A new match starts the detector over.
- **One extra flash is possible between matches.** When a match is abandoned
  during the draft and the next one is first seen with its draft already
  finished, the light may flash once for that draft.
- **The two flashes have separate once-per-match guards.** A match can have its
  lobby flash and its draft-end flash; it cannot have two of either.
- **Losing the feed forgets both.** If the dashboard goes away and comes back,
  the watcher re-syncs from what it can see — which means a lobby it already
  announced can be announced once more. That is the same "never a confident
  wrong light" trade the draft flash has always made.
- **A null game state is never treated as the draft ending.** If the dashboard
  briefly can't see Dota's state, the watcher does not flash on the way into or
  out of that gap — same rule as a dropped feed.
- **A rejected token backs the polling off to 30 seconds.** Every other state
  keeps the configured cadence.
- **A dashboard restart shows `idle — no game data yet` until Dota next checks
  in.** The
  dashboard reports "never heard from Dota" rather than a number it inherited
  from before the restart, so the beacon goes dark instead of trusting a
  freshness figure nobody actually measured. During a live game Dota checks in
  within a second or two, and during a pause about every 10 seconds, so this
  clears itself quickly — but if you restart the dashboard mid-pause, expect the
  beacon to be dark for a moment before it resumes spinning.
- **Settings live in `blinkybeacon-config.json`**, next to the `.exe`:

  ```json
  {
    "addr": "127.0.0.1",
    "port": 1337,
    "dashboard_url": "https://dashboard.example.com",
    "line_number": 2,
    "token": "the-token-you-were-given",
    "flash_seconds": 15,
    "lobby_flash": false,
    "lobby_flash_seconds": 10,
    "pause_side": "both",
    "poll_interval_ms": 2000
  }
  ```

  The last five are the [section 4](#4-tuning-the-light) settings.
  `pause_side` is `both`, `radiant` or `dire`; both durations are 1–600
  seconds; `poll_interval_ms` is 500–10000 and is set here only.
  **A missing key, or one out of range, loads as its default** rather
  than stopping the app — so a file written by v0.5.0 keeps working untouched.
  (The settings page is stricter: type 900 there and it says no, instead of
  quietly saving 15.)

  You can edit it by hand while the app is closed if you prefer. **This file
  holds the token in plain text** — see [section 3](#3-get-a-token-and-paste-it-in).
- **The dashboard's clock, not yours.** "Paused" and "the draft ended" are read
  from the game data the dashboard receives, and the watcher assumes the line is
  bound to the venue's own **undelayed observer client**. If the bound Dota
  client is on a broadcast delay, the beacon will flash and spin on delayed
  time — correctly, but late by however long that delay is.

- **The manual controls still work** while the watcher is running, but the
  watcher will correct the light on its next poll — it decides from the
  dashboard, every time. Unbind it (blank the URL, or forget the token) and it
  hands the beacon back dark, once, and then leaves it to you.

## 8. What this version does not do

- No sound, no notification, no history. One light, three states.
- Only one line at a time.
- The side filter is Radiant or Dire, not a team name — the beacon knows which
  side of the map paused, not who is sitting on it.
