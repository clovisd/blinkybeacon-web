# PUNCHLIST — rapid-fire fix queue

The small items for this repository: checks, fixes and questions too small for a stage row in
[ROADMAP-STAGES.md](ROADMAP-STAGES.md). The MASTER-DEV board shows every line under `## Queue`
as `origin/main` carries it.

- **One line per item.** The board counts every non-blank line under `## Queue` as an item, so
  an item never wraps onto a second line, and the Queue has no sub-headings. Keep a line under
  300 characters; the board cuts it there.
- **Closing an item:** move its line to `## Done` in the commit that settles it, with the tag
  or commit. An item decided against moves to `## Deferred` with the reason. Nothing leaves the
  Queue silently.
- **Prefixes:** `owner:` needs the owner's hands or answer · `watch:` depends on a change in
  DOTA-DASHBOARD · `parked:` waits on a named condition. An item with no prefix is ready to build.
- **Line:** `(dashboard line)` marks an item whose code exists only on the `dashboard` branch;
  an item that names no line applies to both.
- A source written as `sessions/…` or "retro" is in MASTER-DEV; the retros are
  `sessions/retros/<date>-blinkybeacon-<stage>.md`.

**Written 2026-09-30** by the same audit as the stage tracker, and **updated the same day** with
the owner's answers to its questions (MASTER-DEV
`sessions/retros/assets/2026-09-30-owner-answers/batch-1.md`, and `batch-3.md` beside it for
the macOS, replug, pause, Cloudflare and token items). An item that says *owner, 2026-09-30*
takes its words from those answers.

## Queue

*(append items below — plain lines, any wording)*

- owner: macOS on a real light — v0.4.0 and dashboard-v0.7.0 ship macOS zips: arm64 built and tested in CI, amd64 built only; nobody has run either against a beacon on a Mac (`sessions/HANDOFF-blinkybeacon.md` §1, 2026-08-22). "Not now" (owner, 2026-09-30); the release promotion waits on it.
- owner: USB replug mid-pause — unbinding mid-pause used to leave the light spinning (fixed in `cebf8ea`); the same on a USB replug was never checked (b1-token retro, 2026-08-20). Needs the light. "Not now" (owner, 2026-09-30); the release promotion waits on it.
- The release workflow at the next `v*` release — both `release.yaml` runs failed on 2026-08-22 (Go 1.19 against go.mod's 1.25); the fix (`c0d1c4d`) is unproven until a `v*` release shows one green "Generate Release Artifacts" run. `dashboard-v*` releases skip it since dashboard-v0.8.0.
- go vet under GOOS=windows is red on upstream `cmd/fsbeacon/commands/root.go` (unbuffered `os.Signal` channels, lines 25 and 32), which is why every kickoff vets only the tray package (b4-settings retro, 2026-08-21).
- watch: a second API token revokes the beacon's — the dashboard keeps one `lines:read` token, so minting one for another consumer darkens every beacon. Until DOTA-LIVE-DASHBOARD #75 is settled, mint no other (dotabot match-telemetry retros, 2026-08-20/21).
- owner: the API token expires — tokens last 180 days (S79a spec), the beacon's from 2026-08-20 or later: the light may go dark from about 2027-02-16. "Don't know — note to mint a fresh one by February" (owner, 2026-09-30). A new `lines:read` token revokes the old: paste it in every tray at once.
- watch: the dashboard's subdomain move — the owner's plan to serve the API from `api.dash.cl6.us` (S79 owner rulings, 2026-08-12), still planned with no date (owner, 2026-09-30): if a beacon's Dashboard URL has to change, the tray clears its saved token and each beacon needs it pasted again.
- parked: CSRF on /spin /flash /stop — the same hole the settings form closed (b1-token retro, 2026-08-20). The owner lifted the Companion freeze for this fix only (2026-09-30): the module sends a token (S17), then the tray checks it (S18).
- owner: approved 2026-09-30, once the macOS and replug checks pass — promote both releases out of prerelease: `gh release edit v0.4.0 --repo clovisd/blinkybeacon-web --prerelease=false --latest` · `gh release edit dashboard-v0.7.0 --repo clovisd/blinkybeacon-web --prerelease=false --latest=false`
- Re-commit `tools/protocol-fuzzer` (the tool and its testing guide) from `origin/protocol-fuzzer` in a pull request under the owner's name; the old commit's author field cannot land on `main`. The branch stays (owner, 2026-09-30).
- Match identity across back-to-back matches (dashboard line) — if the earlier match spent its flash, a next match named only after hero selection never flashes, and one named during it flashes late or twice. Needs the next match seen within 30 s; unmeasured live (dash-fixes retro, 2026-10-02).
- owner: Start with Windows after the upgrade — put `BB-DASH.exe` beside the old program, delete the old one, tick Start with Windows again, and check it starts at login; the registry path has never run on Windows (dash-fixes retro, 2026-10-02).
- Two code comments (`formSeconds` and a settings test) still say a stale settings tab reaches the save handler (dashboard line); the form's token refuses it first, since every save mints a new one (dash-fixes retro, 2026-10-02).
- `BUILD_MACOS.md` on the dashboard line says the build workflow attaches `blinkybeacon-tray-macos-*.zip`; since dashboard-v0.8.0 this line's zips are `BB-DASH-macos-*.zip` (dashboard line; task review, 2026-10-02).

## Documented quirks

Real, known, and deliberately not queued: each one is told to users or recorded as a trap.

- A good token pasted after a rejection can take up to 30 s to show `watching` (dashboard line): the loop is asleep in its backoff and re-reads the config on waking (`BEACON-DASHBOARD.md` §6; b1-token retro).
- A dashboard restarted mid-pause (dashboard line) shows `idle — no game data yet` and the light is dark until Dota next checks in, about 10 s (`BEACON-DASHBOARD.md` §7).
- Losing the feed forgets both once-per-match guards (dashboard line), so a lobby can be announced twice (`BEACON-DASHBOARD.md` §7).
- A hand-edited `"pause_side": "Radiant "` (trailing space) in the config file (dashboard line) falls back to both sides; the settings form trims, the file loader does not (b4-settings retro).
- `go test ./...` is red on Linux: `pkg/fsbeacon` needs libudev. Test `./cmd/blinkybeacon-tray/` only.
- A match abandoned during the draft, then the next first seen with its draft already finished and no match id yet, flashes once for that draft (dashboard line; `BEACON-DASHBOARD.md` §7, dashboard-v0.8.0).
- A hand-typed `poll_interval_ms` must be a bare whole number: a quoted or decimal value makes the config file unreadable, and every setting, the token included, falls back to its default (dashboard line; `BEACON-DASHBOARD.md` §4).

## Deferred

*(item · why · what was learned)*

## Done

*(item · commit or tag · date)*

- `tray.go` is `gofmt`-clean — `7333845` (v0.4.0), 2026-08-21; `gofmt -l` empty on both lines, measured 2026-09-30.
- `BUILD_WINDOWS.md` no longer calls unplug-while-idle undetected — `02957b8` (dashboard-v0.1.0), 2026-08-19, on main `30f0ad5`.
- The reproducible Windows build line in `BUILD_WINDOWS.md` — dashboard-v0.4.0, 2026-08-20, on main `30f0ad5`.
- The side-filter hint on the settings page (needs dashboard v3.101.0) — `19d7010` (dashboard-v0.6.0), 2026-08-21.
- The draft flash needs no cfg reinstall, and says so — `cbfac6b` (dashboard-v0.5.0), 2026-08-21.
- The "undelayed observer client" sentence in the user guide — `dfd1cec` (dashboard-v0.2.0), 2026-08-20.
- A Windows tray job in `release.yaml` — replaced by `.github/workflows/build.yml` — `7333845` (v0.4.0), 2026-08-21.
- A "Test connection" button — replaced by the line picker's reasons on the settings page (`token rejected`, `unreachable`, `no token`) — dashboard-v0.4.0, 2026-08-20.
- `match_id` on non-public lines stays published, so nothing changes in the tray — the owner's answer to DOTA-LIVE-DASHBOARD #72 Q1, 2026-09-30.
- One real pause, measured (how late the spin ends; the 30 s feed-quiet threshold; S79 design §3.3d, Q2; watcher retro, 2026-08-19) — never measured. Owner, 2026-09-30: "It worked, we used this live at an event." Read as closed on that live use, no longer needed; the owner can reopen it.
- The production path (whether Cloudflare caches `/api/v0/lines/<N>`; the round trip; S79 design §8.2–8.3; b1-token retro, 2026-08-20) — never measured. Owner, 2026-09-30: "It worked, we used this live at an event." Read as closed on that live use, no longer needed; the owner can reopen it.
- The settings page asks the dashboard for its line list at most once per 5 s, failures included — `a6f8027` (dashboard-v0.8.0), 2026-10-02.
- Once-per-match draft flash, tested through the real loop: one flash per match, re-armed by the next — `a6f8027` (dashboard-v0.8.0), 2026-10-02.
- A settings save without a light setting — unreachable: every save mints a new form token, so an older tab's form is refused; pinned by a test — `a6f8027` (dashboard-v0.8.0), 2026-10-02.
