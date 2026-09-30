# ROADMAP-STAGES.md — stage tracker

The stage queue for this repository: what has shipped, what is waiting, and what is only a
candidate until the owner says it is wanted. The MASTER-DEV board reads the `| S<n> |` rows of
this file as `origin/main` carries it. Smaller items — checks, fixes, questions — live in
[PUNCHLIST.md](PUNCHLIST.md).

**Written 2026-09-30** from an audit of every place this project's pending work had been
recorded: the MASTER-DEV ledger rows, kickoffs and retros for `blinkybeacon`, its handoff and
project database, the DOTA-DASHBOARD S79 design of record, this repository's pull requests,
releases and branches, and the MASTER-DEV intake. Before this file the repository had no
tracker; its history lived only in MASTER-DEV.

**Updated 2026-09-30** with the owner's answers to that audit's questions: S9, S11 and S12
changed, S13–S20 added. A cell that says *owner, 2026-09-30* takes its words from those
answers.

**Keeping it:** a stage flips its own row in the commit that ships it, with the tag and the
merge commit as evidence. A new stage takes the next free number. Nothing is renumbered.

**Statuses** (the circle leads every status cell, as in DOTA-DASHBOARD's tracker):
🟡 `IN PROGRESS` → being built · ⚪ `NEXT`/`QUEUED` → open this one next / sequenced ·
⚫ `DEFERRED` → parked deliberately, trigger named · ⚫ `CANDIDATE` → proposed but never asked
for; the owner decides · 🟢 `SHIPPED <tag>` → closed · 🔴 `BLOCKED` → halted on a decision or a
dependency.

**Line** is the branch a stage ships on: `main` (the generic tray, tags `v*`) or `dashboard`
(the generic tray plus the DOTA-DASHBOARD companion, tags `dashboard-v*`). Generic work lands
on `main` and is merged into `dashboard`, never the reverse. A stage whose work is in the
Bitfocus Companion module names that repository, `clovisd/blinkybeacon-companion`, instead.

**Evidence outside this repository:** "ledger" is a row of MASTER-DEV `sessions/LEDGER.md`;
"retro" is a file in MASTER-DEV `sessions/retros/`; "S79 design" is DOTA-DASHBOARD
`docs/superpowers/specs/2026-08-12-s79-beacon-companion-design.md`, whose sub-stage names
(B0–B4, C, D) the rows below keep; "the owner's answers, 2026-09-30" is MASTER-DEV
`sessions/retros/assets/2026-09-30-owner-answers/batch-1.md`, answering
`sessions/retros/assets/2026-09-30-blinkybeacon-docs-audit/owner-questions.md`.

## Stage queue

| # | Stage | Feature source | Status | Line | Evidence |
|---|---|---|---|---|---|
| S1 | **The generic tray** — tray menu, HTTP API (`/spin` `/flash` `/stop` `/status`), settings page, unplug detection, state icons | The owner's own sessions, May 2026, before the MASTER-DEV program | 🟢 **SHIPPED v0.3.2** (`62caa71`, 2026-05-20) | main | GitHub releases v0.2.0 → v0.3.2 |
| S2 | **Watch a dashboard line** — S79 B0: flash when the draft ends, spin during a pause, dark when the feed is lost | Owner, 2026-08-12 (the S79 plan) and 2026-08-19 ("go all the way to ship") | 🟢 **SHIPPED dashboard-v0.1.0** (`0ade3d9`, 2026-08-19) | dashboard | ledger 2026-08-19 `watcher` · retro `2026-08-19-blinkybeacon-watcher.md` |
| S3 | **Authenticate to the dashboard's read API** — S79 B1: a bearer token that is never shown back, the config written owner-only | Owner, 2026-08-20: "Run it to full ship" | 🟢 **SHIPPED dashboard-v0.2.0** (`17363c8`, 2026-08-20) | dashboard | PR #1 · ledger 2026-08-20 `b1-token` · needs dashboard v3.97.0+ |
| S4 | **Flash at the last final pick; say idle, not FEED LOST** — S79 B2, the tray half | Owner ruling 2026-08-20 (flash 15 s at the last final pick) and the owner's FEED LOST report the same evening | 🟢 **SHIPPED dashboard-v0.3.0** (`d5e0f1e`, 2026-08-20) | dashboard | PR #2 · ledger 2026-08-20 `b2-draft-flash` · `draft_complete` needs dashboard v3.99.0+ |
| S5 | **Pick the line from a list** — S79 C: the settings page fetches `/api/v0/lines`; `Bound: Line C` in the tray; the reproducible Windows build line | S79 design §6 Stage C; sprint item, 2026-08-20 | 🟢 **SHIPPED dashboard-v0.4.0** (`29bf5c4`, 2026-08-20) | dashboard | PR #3 · ledger 2026-08-20 `c-line-picker` · line numbers need dashboard v3.98.0+ |
| S6 | **Flash at the end of the draft on every install** — S79 B3: the game-state trigger restored beside `draft_complete`, once per match, no cfg reinstall | Owner, 2026-08-21: "Please deal with the flashing light situation and config problem." | 🟢 **SHIPPED dashboard-v0.5.0** (`95a88e1`, 2026-08-21) | dashboard | PR #4 · ledger 2026-08-21 `b3-state-fallback` |
| S7 | **Three light settings** — S79 B4: the draft-flash length, a new-lobby flash, a pause-side filter | Owner request, 2026-08-21 | 🟢 **SHIPPED dashboard-v0.6.0** (`5d4c5e2`, 2026-08-21) | dashboard | PR #5 · ledger 2026-08-21 `b4-settings` · the side filter needs dashboard v3.101.0+ |
| S8 | **Two lines, macOS, the API on the settings page** — `main` rebuilt as the generic tray with the hardening the dashboard line found (escaping, CSRF on the settings form, a 0600 config); the HTTP API documented on the settings page; macOS builds in CI | Owner, inline session 2026-08-21 (the branch split and macOS) | 🟢 **SHIPPED v0.4.0** (`7333845`) and **dashboard-v0.7.0** (`099c48a`), 2026-08-22 | both | MASTER-DEV `sessions/HANDOFF-blinkybeacon.md` §1 · build runs 32543413637 and 32543413711 green · `main` became the generic line on 2026-09-30 (`c0d1c4d`, the owner's force-push; build run 36711038063 green) |
| S9 | **Test flash, the tray half** — a test button in the tray's settings that gives a short double flash, and the tray obeying the dashboard's test signal once the dashboard half exists; the dashboard's button flashes every connected beacon at once, so an operator can check the light before a match | Owner, Discord #punchlist-dashboard, 2026-08-21 (`#eb3e50`, filed under dota-dashboard); owner, 2026-09-30: a button on the dashboard and in the tray's settings, *"A short double flash"*, and *"a test flash button in the dashboard to test flash all beacons connected"* | ⚪ **QUEUED** — owner-approved 2026-09-30; the settings button needs nothing from the dashboard; obeying the test signal waits for the dashboard to add it to `/api/v0`, a contract change there first | both — the button on `main`, the test signal on `dashboard` | MASTER-DEV `sessions/logs/discord-inbox/punchlist.jsonl` lines 161–164 · the owner's answers, 2026-09-30 · the dashboard half is DOTA-DASHBOARD's own row |
| S10 | **Follow the dashboard's keyed API** — per-consumer keys (today a second `lines:read` token revokes the beacon's), a line identity that survives a deleted line's number being reused, push instead of polling, `v0` → `v1` | S79 design §4.5 and §6 Stage D; DOTA-DASHBOARD ROADMAP item D; DOTA-LIVE-DASHBOARD #75 | ⚫ **DEFERRED** — trigger: the dashboard builds its item D; the tray follows the API, never leads it | dashboard | retro `2026-08-20-dota-dashboard-line-identity-build.md` ("the `/api/v0/` residual is real and unmitigated at v0") · #75 open |
| S11 | **A "paused for M:SS" timer in the tray** — one of three operator niceties the S79 design left optional; the owner wanted the other two, now S13 (the poll interval) and S14 (the bound-line icon) | S79 design §6 Stage C ("if wanted") | ⚫ **DEFERRED** — owner, 2026-09-30; trigger: the owner asks for it. The timer would also need `pause_started_at` on `/api/v0` | dashboard | the owner's answers, 2026-09-30 |
| S12 | **Team names instead of Radiant/Dire** — the side filter names a side of the map, never a team. The owner wanted the rest of the user guide's list of what this version does not do: now S15 (sound, notifications or a history) and S16 (more than one line at a time) | `BEACON-DASHBOARD.md` §8, on the dashboard line | ⚫ **DEFERRED** — owner, 2026-09-30; trigger: the owner asks for it. Team names would also need team identity on `/api/v0` | dashboard | the owner's answers, 2026-09-30 |
| S13 | **The poll interval in the config file** — how often the tray reads its dashboard line, set in the config file instead of fixed at 2 s (the S79 design proposed 500 ms; at 2 s the light can lag by up to 2 s) | S79 design §3.4 and §6 Stage C ("if wanted"), from S11; the Stage C kickoff kept the poll loop as it was; owner, 2026-09-30: wanted | ⚪ **QUEUED** — owner-wanted, 2026-09-30 | dashboard | `cmd/blinkybeacon-tray/watcher.go` `pollInterval = 2 * time.Second` · the owner's answers, 2026-09-30 |
| S14 | **An icon that shows the tray is bound to a line** — the tray's light icon changes while the tray follows a dashboard line | S79 design §6 Stage C ("if wanted"), from S11; owner, 2026-09-30: *"Just change the light from blue, to blue with a brain or something saying its smart controlled"* | ⚪ **QUEUED** — owner-wanted, 2026-09-30 | dashboard | retro `2026-08-20-blinkybeacon-c-line-picker.md` (no new icon) · the owner's answers, 2026-09-30 |
| S15 | **Sound, notifications or a history** — this version has none: one light, three states | `BEACON-DASHBOARD.md` §8, on the dashboard line, from S12; owner, 2026-09-30: wanted | ⚪ **QUEUED** — owner-wanted, 2026-09-30 | dashboard | the owner's answers, 2026-09-30 |
| S16 | **More than one line at a time** — this version follows only one dashboard line | `BEACON-DASHBOARD.md` §8, on the dashboard line, from S12; owner, 2026-09-30: wanted | ⚪ **QUEUED** — owner-wanted, 2026-09-30 | dashboard | the owner's answers, 2026-09-30 |
| S17 | **The Companion module sends a token** — the Bitfocus Companion module sends a token on its `/spin`, `/flash` and `/stop` calls, so the tray can check it (S18). The owner lifted the module's freeze for S17 and S18 only: until both have run, no other change touches `/spin`, `/flash`, `/stop` or `/status` | Owner, 2026-09-30: lift the freeze and add a token, in two stages, the module first; it answers the CSRF gap on the control routes (PUNCHLIST.md) | ⚪ **QUEUED** — owner-approved 2026-09-30 | `clovisd/blinkybeacon-companion` | b1-token retro, 2026-08-20 · the owner's answers, 2026-09-30 |
| S18 | **The control routes check the token** — `/spin`, `/flash` and `/stop` check the token the Companion module sends (S17), on both lines — the same CSRF gap the settings form's own token fixed for `/settings` on 2026-08-20 | Owner, 2026-09-30, as S17 | ⚪ **QUEUED** — owner-approved 2026-09-30; after S17, the module first | both | PUNCHLIST.md, the CSRF line · the owner's answers, 2026-09-30 |
| S19 | **Each beacon registers with the dashboard** — a beacon "phones home", registers with the dashboard and loads its settings from there. The tray keeps only local settings: the dashboard URL and a fallback URL, the token, a beacon name, start on startup, listen on HTTP too. The dashboard decides when a beacon lights, when it does not, and what triggers it | Owner, 2026-09-30: *"in the future, we want each beacon to be able to phone home to the dashboard, register there, and then load settings from there"*, and *"However this may be way more complicated than it should be, so perhaps just a test flash button in the dashboard to test flash all beacons connected."* | ⚫ **CANDIDATE** — for later; the owner decides. The test flash (S9) comes first | dashboard | the owner's answers, 2026-09-30 · belongs with S20 |
| S20 | **A dashboard page that finds a running tray and sets it up** — a page on the dashboard finds a tray running on the same computer and provisions it | Owner, 2026-09-30: *"Is it possible to have a page on the dashboard that detects a running blinkybeacon, and force provisions it from the prod build? Ie: Check for Blinky and Add to Tool?"* | ⚫ **CANDIDATE** — for later: it hands a beacon its token from a web page, so its design goes to the owner before any build | dashboard | the owner's answers, 2026-09-30 · belongs with S19 |
