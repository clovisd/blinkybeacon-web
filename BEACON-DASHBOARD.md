# Beacon ↔ Dashboard — user guide

The tray app can watch a LIVE Dashboard line and drive the beacon by itself:

- **The draft ends → the beacon flashes red for 5 seconds.**
- **The game is paused → the beacon spins amber, for as long as the pause lasts.**
- **Anything else → the beacon is dark.**

It polls the dashboard every 2 seconds over the dashboard's read API,
`/api/v0/lines/<N>`, using an **API token** that a dashboard admin mints for
you. Without that token the watcher does not poll at all — see
[section 3](#3-get-a-token-and-paste-it-in).

---

## 1. Get the app running

1. Download `blinkybeacon-tray.exe` from the release, or build it yourself —
   see [BUILD_WINDOWS.md](BUILD_WINDOWS.md).
2. Put it in a folder of its own. It writes `blinkybeacon-config.json` next to
   itself, so don't leave it in `Downloads`.
3. Plug the beacon in over USB.
4. Double-click the `.exe`. A tray icon appears (bottom-right, possibly under
   the `^` arrow). There is no window — that's normal.

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
3. Fill in **Line Number** — the `N` in the dashboard's own URL for that line.
   Open the line in your browser and read it off the address bar:
   `http://192.168.1.50:8080/line/`**`2`**`/` means Line Number is `2`.
   > The number is the line's own id, **not** its position in the list — the
   > third line on the page is not necessarily line 3. Always read it from the
   > URL.
4. Leave **Dashboard API Token** for the next section, and click
   **Save & Apply**.

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
  `.exe`, because the app has to send it on every poll. Treat that file the way
  you would treat a password: don't put the folder on a shared drive, and don't
  paste the file into a bug report.
- **It never appears in the log**, and never travels in a URL — only in an
  `Authorization` header, which proxies do not log by default.
- **Revoking it on the dashboard is instant.** The beacon goes dark on the very
  next poll, within about 2 seconds.

## 4. What each light means

| Beacon | Means |
|---|---|
| **Flashing red, ~5 seconds** | The draft just ended — the game is starting. |
| **Spinning amber** | The game is paused, right now. It stops when play resumes. |
| **Dark** | Nothing is happening — **or** the dashboard feed is gone. Check the tray. |

Dark is deliberately ambiguous at the beacon, so the tray menu always spells
out which one it is:

| Tray line | Means |
|---|---|
| `○ Dashboard: not configured` | No dashboard URL set. The watcher is off. |
| `○ Dashboard: no token set` | URL set, but no API token. **Nothing is being polled at all** — see [section 3](#3-get-a-token-and-paste-it-in). |
| `● Dashboard: watching` | Polling fine, feed is fresh. Trust the light. |
| `▲ Dashboard: token rejected` | The dashboard refused the token: it was revoked, expired, or mistyped. Mint a new one and paste it in. |
| `▲ Dashboard: line 2 not found` | Token accepted, but that line number doesn't exist (or isn't an official line). Re-read the number from the dashboard URL. |
| `▲ Dashboard: FEED LOST` | Can't reach the dashboard, or it has gone quiet. **The beacon is dark and is telling you nothing.** |
| `○ Dashboard: line not running` | Dashboard reached, but that line isn't accepting game data. |

All six of the non-`watching` lines mean the same thing at the beacon: **dark**.
The tray is the only place that tells you which one you are looking at.

The rule the watcher follows: **never a confident wrong light.** If it cannot
see the game, it goes dark rather than leaving the beacon spinning on a pause
that may have ended minutes ago.

## 5. If it isn't working

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

**`▲ FEED LOST` immediately after setting it up**

- Wrong URL or port. Open the same URL in a browser on this PC — you should see
  the dashboard. If the browser can't reach it, nor can the beacon.
- The dashboard doesn't have the read API. `FEED LOST` (rather than
  `token rejected`) on a URL you can open in a browser usually means the
  dashboard is older than the `/api/v0/lines/` route. Update the dashboard.

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

## 6. Details worth knowing

- **Polls every 2 seconds**, so the light can lag reality by up to ~2 seconds.
- **The feed counts as lost** if the dashboard hasn't heard from Dota for more
  than 30 seconds (or can't be reached at all). During a legitimate pause Dota
  still checks in about every 10 seconds, so a real pause is never mistaken for
  a dead feed.
- **A pause outranks the flash.** If a pause begins during the 5-second flash,
  the beacon switches straight to spinning, and does not go back to flashing
  when the pause lifts.
- **A rejected token backs the polling off to 30 seconds.** Every other state
  keeps the normal 2-second cadence.
- **Settings live in `blinkybeacon-config.json`**, next to the `.exe`:

  ```json
  {
    "addr": "127.0.0.1",
    "port": 1337,
    "dashboard_url": "https://dashboard.example.com",
    "line_number": 2,
    "token": "the-token-you-were-given"
  }
  ```

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

## 7. What this version does not do

- No pick-the-line-from-a-list — you type the number.
- No sound, no notification, no history. One light, three states.
- Only one line at a time.
