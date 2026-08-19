# Beacon ↔ Dashboard — user guide

The tray app can watch a LIVE Dashboard line and drive the beacon by itself:

- **The draft ends → the beacon flashes red for 5 seconds.**
- **The game is paused → the beacon spins amber, for as long as the pause lasts.**
- **Anything else → the beacon is dark.**

It polls the dashboard every 2 seconds over the dashboard's normal web page
route. Nothing needs to be installed, enabled or changed on the dashboard.

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
4. Click **Save & Apply**.

That's it. The watcher picks the change up on its next poll — no restart.

To turn the watcher off again, blank the Dashboard URL and save. The beacon
then does nothing until you drive it by hand from the tray menu (Spin / Flash /
Stop), which stays available at all times.

## 3. What each light means

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
| `● Dashboard: watching` | Polling fine, feed is fresh. Trust the light. |
| `▲ Dashboard: FEED LOST` | Can't reach the dashboard, or it has gone quiet. **The beacon is dark and is telling you nothing.** |
| `○ Dashboard: line not running` | Dashboard reached, but that line isn't accepting game data. |

The rule the watcher follows: **never a confident wrong light.** If it cannot
see the game, it goes dark rather than leaving the beacon spinning on a pause
that may have ended minutes ago.

## 4. If it isn't working

**`▲ FEED LOST` immediately after setting it up**

- Wrong URL or port. Open the same URL in a browser on this PC — you should see
  the dashboard. If the browser can't reach it, nor can the beacon.
- Wrong line number. A line that doesn't exist reads as a lost feed. Re-read the
  number from the dashboard URL.
- The dashboard is refusing this PC. The watcher uses the dashboard's ordinary
  viewer-level access, which a machine on your LAN normally gets automatically.
  If the dashboard has been locked down (`public_view` / IP-view turned off, or
  the line set to specific users), it will answer "no" and the watcher reports a
  lost feed. Fix it on the dashboard side.

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

## 5. Details worth knowing

- **Polls every 2 seconds**, so the light can lag reality by up to ~2 seconds.
- **The feed counts as lost** if the dashboard hasn't heard from Dota for more
  than 30 seconds (or can't be reached at all). During a legitimate pause Dota
  still checks in about every 10 seconds, so a real pause is never mistaken for
  a dead feed.
- **A pause outranks the flash.** If a pause begins during the 5-second flash,
  the beacon switches straight to spinning, and does not go back to flashing
  when the pause lifts.
- **Settings live in `blinkybeacon-config.json`**, next to the `.exe`:

  ```json
  {
    "addr": "127.0.0.1",
    "port": 1337,
    "dashboard_url": "http://192.168.1.50:8080",
    "line_number": 2
  }
  ```

  You can edit it by hand while the app is closed if you prefer.

- **The manual controls still work** while the watcher is running, but the
  watcher will correct the light on its next poll — it decides from the
  dashboard, every time.

## 6. What this version does not do

- No pick-the-line-from-a-list — you type the number.
- No sound, no notification, no history. One light, three states.
- Only one line at a time.
