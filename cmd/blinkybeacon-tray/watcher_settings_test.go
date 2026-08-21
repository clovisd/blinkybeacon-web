package main

import (
	"testing"
	"time"
)

// defaultSettings is the watcher configured exactly as a fresh install is: the
// fifteen-second draft flash, no new-lobby flash, and every pause spins. It is
// what every pre-v0.6.0 test now builds its watcher with, which is the whole
// point — those tests describe v0.5.0's behaviour, and v0.5.0's behaviour is
// what the defaults have to keep producing.
func defaultSettings() WatcherSettings { return watcherSettings(defaultConfig()) }

// lobbyFeed is a fresh, live, unpaused line carrying one match id and one game
// state — the two things the new-lobby trigger reads.
func lobbyFeed(matchID, gameState string) *LineState {
	ls := liveFeed()
	ls.N = 1
	ls.GameState = gameState
	if matchID == "" {
		ls.MatchID = nil
	} else {
		ls.MatchID = strPtr(matchID)
	}
	return ls
}

// pausedBy is a paused line the dashboard has attributed to a party — or, for a
// nil party, one it has not (or an older dashboard that cannot).
func pausedBy(party *string) *LineState {
	ls := liveFeed()
	ls.MatchID = strPtr("7891234567")
	ls.Paused = true
	ls.PauseParty = party
	return ls
}

// settingsWith builds the watcher's settings from a Config the test edits, so
// what is being exercised is the same path the settings page drives.
func settingsWith(edit func(*Config)) WatcherSettings {
	cfg := defaultConfig()
	edit(&cfg)
	return watcherSettings(cfg)
}

// ------------------------------------------ (i) the draft flash's duration

func TestDecide_theDraftFlashLastsTheConfiguredSeconds(t *testing.T) {
	// (i) flash_seconds = 5, so the light is out at six seconds — where the
	// hard-coded fifteen would still have it flashing.
	w := NewWatcher(settingsWith(func(c *Config) { c.FlashSeconds = 5 }))
	now := time.Now()
	w.Decide(now, draftingFeed())
	if state, _ := w.Decide(now, draftDoneFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash at the last final pick", state)
	}

	if state, _ := w.Decide(now.Add(5*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("at t+5s exactly: state = %q, want flash — five seconds means five", state)
	}
	if state, _ := w.Decide(now.Add(6*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("at t+6s: state = %q, want idle — the duration came from the config, not a constant", state)
	}
}

func TestDecide_theDraftFlashStillLastsFifteenSecondsByDefault(t *testing.T) {
	// The upgrade promise, stated as a test: nobody who does not open Settings
	// sees the flash change length.
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	w.Decide(now, draftDoneFeed())

	if state, _ := w.Decide(now.Add(15*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("at t+15s: state = %q, want flash", state)
	}
	if state, _ := w.Decide(now.Add(15100*time.Millisecond), draftDoneFeed()); state != StateIdle {
		t.Errorf("after t+15s: state = %q, want idle", state)
	}
}

// ------------------------------------------------- (ii)-(v) the new lobby

func TestDecide_flashesWhenANewLobbyAppears(t *testing.T) {
	// (ii) The line had no match on it; now it has one, and the game has not
	// started. That is a lobby, and the light says so for lobby_flash_seconds.
	w := NewWatcher(settingsWith(func(c *Config) {
		c.LobbyFlash = true
		c.LobbyFlashSeconds = 8
	}))
	now := time.Now()

	if state, _ := w.Decide(now, lobbyFeed("", gameStateHeroSelection)); state != StateIdle {
		t.Fatalf("state = %q, want idle while the line carries no match at all", state)
	}

	armed := now.Add(2 * time.Second)
	if state, _ := w.Decide(armed, lobbyFeed("123", gameStateHeroSelection)); state != StateFlash {
		t.Fatalf("state = %q, want flash on the first sight of match 123", state)
	}
	if state, _ := w.Decide(armed.Add(8*time.Second), lobbyFeed("123", gameStateHeroSelection)); state != StateFlash {
		t.Errorf("at t+8s exactly: state = %q, want flash", state)
	}
	if state, _ := w.Decide(armed.Add(8100*time.Millisecond), lobbyFeed("123", gameStateHeroSelection)); state != StateIdle {
		t.Errorf("after the 8s lobby flash: state = %q, want idle", state)
	}
}

func TestDecide_flashesForEveryPreGameStateANewLobbyCanBeIn(t *testing.T) {
	// A fresh lobby is not always caught in hero selection: the tray polls
	// every two seconds and the match id can appear while Dota is still
	// loading. All four of the dashboard's pre-game states count.
	for _, state := range []string{
		"DOTA_GAMERULES_STATE_INIT",
		"DOTA_GAMERULES_STATE_WAIT_FOR_PLAYERS_TO_LOAD",
		"DOTA_GAMERULES_STATE_WAIT_FOR_MAP_TO_LOAD",
		gameStateHeroSelection,
	} {
		w := NewWatcher(settingsWith(func(c *Config) { c.LobbyFlash = true }))
		if got, _ := w.Decide(time.Now(), lobbyFeed("123", state)); got != StateFlash {
			t.Errorf("game_state %s: state = %q, want flash", state, got)
		}
	}
}

func TestDecide_noLobbyFlashByDefault(t *testing.T) {
	// (iii) The same sequence that flashes above, on a fresh install: dark.
	w := NewWatcher(defaultSettings())
	now := time.Now()

	w.Decide(now, lobbyFeed("", gameStateHeroSelection))
	if state, _ := w.Decide(now.Add(2*time.Second), lobbyFeed("123", gameStateHeroSelection)); state != StateIdle {
		t.Errorf("state = %q, want idle — the new-lobby flash is off unless it is asked for", state)
	}
}

func TestDecide_aLateStartOnARunningGameIsNotANewLobby(t *testing.T) {
	// (iv) The tray was started, or retargeted, onto a match already in
	// progress. There is no lobby to announce; the moment passed before anyone
	// was watching, which is the same rule the draft flash has always had.
	w := NewWatcher(settingsWith(func(c *Config) { c.LobbyFlash = true }))
	now := time.Now()

	if state, _ := w.Decide(now, lobbyFeed("123", "DOTA_GAMERULES_STATE_GAME_IN_PROGRESS")); state != StateIdle {
		t.Errorf("state = %q, want idle — a match already in progress is not a new lobby", state)
	}
	// And it does not become one when the state moves on afterwards.
	if state, _ := w.Decide(now.Add(2*time.Second), lobbyFeed("123", "DOTA_GAMERULES_STATE_POST_GAME")); state != StateIdle {
		t.Errorf("state = %q, want idle", state)
	}
}

func TestDecide_theLobbyFlashFiresOncePerMatchAndANewMatchReArmsIt(t *testing.T) {
	// (v) The match id sits on the line for the whole game, so every later poll
	// is the same lobby. The next match is a different one.
	w := NewWatcher(settingsWith(func(c *Config) {
		c.LobbyFlash = true
		c.LobbyFlashSeconds = 3
	}))
	now := time.Now()

	if state, _ := w.Decide(now, lobbyFeed("123", gameStateHeroSelection)); state != StateFlash {
		t.Fatalf("state = %q, want flash for the new lobby", state)
	}
	for i := 1; i <= 3; i++ {
		w.Decide(now.Add(time.Duration(i)*time.Second), lobbyFeed("123", gameStateHeroSelection))
	}
	if state, _ := w.Decide(now.Add(4*time.Second), lobbyFeed("123", gameStateHeroSelection)); state != StateIdle {
		t.Fatalf("state = %q, want idle — the same match must not re-arm the lobby flash", state)
	}

	if state, _ := w.Decide(now.Add(5*time.Second), lobbyFeed("456", gameStateHeroSelection)); state != StateFlash {
		t.Errorf("state = %q, want flash — a different match id is a different lobby", state)
	}
}

func TestDecide_theLobbyFlashIsForgottenWhenTheWatcherGoesBlind(t *testing.T) {
	// forget() clears the lobby guard with everything else, exactly as it does
	// the draft guard: a guard kept without the match identity that releases it
	// would disarm the flash for good.
	w := NewWatcher(settingsWith(func(c *Config) { c.LobbyFlash = true }))
	now := time.Now()
	w.Decide(now, lobbyFeed("123", gameStateHeroSelection))

	if state, _ := w.Decide(now.Add(time.Second), nil); state != StateIdle {
		t.Fatalf("state = %q, want idle while blind", state)
	}
	if state, _ := w.Decide(now.Add(2*time.Second), lobbyFeed("123", gameStateHeroSelection)); state != StateFlash {
		t.Errorf("state = %q, want flash — going blind forgets the lobby guard too", state)
	}
}

// ------------------------------- (vi) the two flashes are separate guards

func TestDecide_aLobbyFlashAndADraftFlashBothFireInOneMatch(t *testing.T) {
	// (vi) One flash per match PER TRIGGER KIND. The lobby flash must not spend
	// the draft flash's guard — that is the whole reason there are two.
	w := NewWatcher(settingsWith(func(c *Config) {
		c.LobbyFlash = true
		c.LobbyFlashSeconds = 4
		c.FlashSeconds = 6
	}))
	now := time.Now()

	drafting := draftingFeed() // match 7891234567, hero selection, picks outstanding
	if state, _ := w.Decide(now, drafting); state != StateFlash {
		t.Fatalf("state = %q, want the lobby flash on the first sight of the match", state)
	}
	if state, _ := w.Decide(now.Add(5*time.Second), drafting); state != StateIdle {
		t.Fatalf("state = %q, want idle once the 4s lobby flash is done", state)
	}

	lastPick := now.Add(30 * time.Second)
	if state, _ := w.Decide(lastPick, draftDoneFeed()); state != StateFlash {
		t.Fatalf("state = %q, want the draft-end flash — the lobby flash did not spend its guard", state)
	}
	if state, _ := w.Decide(lastPick.Add(6*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("at t+6s: state = %q, want flash for the configured six seconds", state)
	}
	if state, _ := w.Decide(lastPick.Add(6100*time.Millisecond), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle after the draft flash", state)
	}
}

func TestDecide_overlappingFlashesRunUntilTheLaterDeadline(t *testing.T) {
	// Both flashes live at once is one light, and one light cannot be cut short
	// by the earlier of the two deadlines.
	w := NewWatcher(settingsWith(func(c *Config) {
		c.LobbyFlash = true
		c.LobbyFlashSeconds = 30
		c.FlashSeconds = 2
	}))
	now := time.Now()
	w.Decide(now, draftingFeed()) // lobby flash: until t+30s
	w.Decide(now.Add(time.Second), draftDoneFeed())

	// The draft flash's own 2s ended at t+3s. The lobby flash has not.
	if state, _ := w.Decide(now.Add(10*time.Second), draftDoneFeed()); state != StateFlash {
		t.Errorf("state = %q, want flash — the shorter draft flash must not end the longer lobby one", state)
	}
	if state, _ := w.Decide(now.Add(31*time.Second), draftDoneFeed()); state != StateIdle {
		t.Errorf("state = %q, want idle once the later deadline passes", state)
	}
}

// --------------------------------------------- (vii)-(viii) the pause side

func TestDecide_theSideFilterSpinsOnlyForThatSidesPauses(t *testing.T) {
	// (vii) The operator asked for radiant's pauses. Everything else — the
	// other team, an admin pause, an attribution the dashboard could not make,
	// and a dashboard too old to attribute at all — leaves the light alone.
	cases := []struct {
		name  string
		party *string
		want  StateValue
	}{
		{"the side asked for", strPtr(pauseSideRadiant), StateSpin},
		{"the other side", strPtr(pauseSideDire), StateIdle},
		{"an admin pause", strPtr("admin"), StateIdle},
		{"an attribution the dashboard could not make", strPtr("unassigned"), StateIdle},
		{"a dashboard that does not publish pause_party", nil, StateIdle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWatcher(settingsWith(func(c *Config) { c.PauseSide = pauseSideRadiant }))
			if got, _ := w.Decide(time.Now(), pausedBy(tc.party)); got != tc.want {
				t.Errorf("state = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecide_theOtherSideFilterIsTheMirrorImage(t *testing.T) {
	w := NewWatcher(settingsWith(func(c *Config) { c.PauseSide = pauseSideDire }))
	if got, _ := w.Decide(time.Now(), pausedBy(strPtr(pauseSideDire))); got != StateSpin {
		t.Errorf("state = %q, want spin for dire's own pause", got)
	}
	w = NewWatcher(settingsWith(func(c *Config) { c.PauseSide = pauseSideDire }))
	if got, _ := w.Decide(time.Now(), pausedBy(strPtr(pauseSideRadiant))); got != StateIdle {
		t.Errorf("state = %q, want idle for the other side's pause", got)
	}
}

func TestDecide_bothSpinsForEveryAttributionIncludingNone(t *testing.T) {
	// (viii) The default, and v0.5.0's behaviour exactly — including the null
	// that an absent pause_party decodes to, which is every pause an older
	// dashboard reports.
	for _, party := range []*string{
		strPtr(pauseSideRadiant), strPtr(pauseSideDire),
		strPtr("admin"), strPtr("unassigned"), nil,
	} {
		w := NewWatcher(defaultSettings())
		if got, _ := w.Decide(time.Now(), pausedBy(party)); got != StateSpin {
			t.Errorf("pause_party %v: state = %q, want spin — \"both\" filters nothing", party, got)
		}
	}
}

func TestDecide_aFilteredOutPauseStillEndsWithTheLightOut(t *testing.T) {
	// The pause the operator filtered out is not a reason to keep the beacon
	// dark forever either: the line is still live, so the light is simply idle
	// and the next flash still works.
	w := NewWatcher(settingsWith(func(c *Config) { c.PauseSide = pauseSideRadiant }))
	now := time.Now()
	if got, _ := w.Decide(now, pausedBy(strPtr(pauseSideDire))); got != StateIdle {
		t.Fatalf("state = %q, want idle", got)
	}

	w.Decide(now.Add(time.Second), draftingFeed())
	if got, _ := w.Decide(now.Add(2*time.Second), draftDoneFeed()); got != StateFlash {
		t.Errorf("state = %q, want flash — a filtered pause must not disarm the watcher", got)
	}
}

// ------------------------------------------------------ (ix) precedence

func TestDecide_aCountedPauseOutranksBothFlashes(t *testing.T) {
	// (ix) Unchanged from v0.5.0, now with two flashes to outrank.
	w := NewWatcher(settingsWith(func(c *Config) {
		c.LobbyFlash = true
		c.LobbyFlashSeconds = 60
		c.PauseSide = pauseSideRadiant
	}))
	now := time.Now()

	if state, _ := w.Decide(now, lobbyFeed("7891234567", gameStateHeroSelection)); state != StateFlash {
		t.Fatalf("state = %q, want the lobby flash", state)
	}
	if state, _ := w.Decide(now.Add(time.Second), pausedBy(strPtr(pauseSideRadiant))); state != StateSpin {
		t.Errorf("state = %q, want spin — a pause outranks a flash that is still running", state)
	}
}

func TestDecide_aPauseOutranksTheDraftFlashUnderTheDefaults(t *testing.T) {
	w := NewWatcher(defaultSettings())
	now := time.Now()
	w.Decide(now, draftingFeed())
	if state, _ := w.Decide(now.Add(time.Second), draftDoneFeed()); state != StateFlash {
		t.Fatalf("state = %q, want flash", state)
	}

	paused := draftDoneFeed()
	paused.Paused = true
	if state, _ := w.Decide(now.Add(2*time.Second), paused); state != StateSpin {
		t.Errorf("state = %q, want spin", state)
	}
}

// -------------------------------------------------- decoding pause_party

func TestFetchLineState_decodesPauseParty(t *testing.T) {
	srv := servePayload(t, `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567",`+
		`"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":true,"seconds_since_gsi":0.6,`+
		`"ts":1765500000,"draft_complete":true,"pause_party":"dire"}`)

	ls, err := fetchLineState(t.Context(), srv.Client(), srv.URL, 1, testToken)
	if err != nil {
		t.Fatalf("fetchLineState: %v", err)
	}
	if ls.PauseParty == nil {
		t.Fatal("pause_party decoded as nil, want a pointer to \"dire\"")
	}
	if *ls.PauseParty != pauseSideDire {
		t.Errorf("pause_party = %q, want %q", *ls.PauseParty, pauseSideDire)
	}
}

func TestFetchLineState_anAbsentPausePartyDecodesTheSameAsNull(t *testing.T) {
	// The contract's absent == null, which is the only thing that lets a tray
	// built against v3.101.0 poll a dashboard that predates it. Both mean "not
	// attributed", and both leave the "both" filter spinning as it always has.
	absent := servePayload(t, `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567",`+
		`"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":true,"seconds_since_gsi":0.6,"ts":1765500000}`)
	null := servePayload(t, `{"v":0,"n":1,"label":"Line A","running":true,"match_id":"7891234567",`+
		`"game_state":"DOTA_GAMERULES_STATE_GAME_IN_PROGRESS","paused":true,"seconds_since_gsi":0.6,`+
		`"ts":1765500000,"pause_party":null}`)

	for name, srv := range map[string]string{"absent": absent.URL, "null": null.URL} {
		ls, err := fetchLineState(t.Context(), absent.Client(), srv, 1, testToken)
		if err != nil {
			t.Fatalf("%s: fetchLineState: %v", name, err)
		}
		if ls.PauseParty != nil {
			t.Errorf("%s: pause_party = %q, want nil", name, *ls.PauseParty)
		}
	}
}

// ------------------------------------------- config to watcher settings

func TestWatcherSettings_carriesTheThreeSettingsAcross(t *testing.T) {
	cfg := Config{FlashSeconds: 20, LobbyFlash: true, LobbyFlashSeconds: 7, PauseSide: pauseSideDire}

	got := watcherSettings(cfg)

	if got.FlashDuration != 20*time.Second {
		t.Errorf("FlashDuration = %v, want 20s", got.FlashDuration)
	}
	if !got.LobbyFlash {
		t.Error("LobbyFlash did not carry across")
	}
	if got.LobbyFlashDuration != 7*time.Second {
		t.Errorf("LobbyFlashDuration = %v, want 7s", got.LobbyFlashDuration)
	}
	if got.PauseSide != pauseSideDire {
		t.Errorf("PauseSide = %q, want %q", got.PauseSide, pauseSideDire)
	}
}

func TestWatcherSettings_substitutesDefaultsForNonsense(t *testing.T) {
	// The config file is not the only way into this: main.go's flags and a
	// half-built Config in a test both reach it. A zero duration would mean a
	// flash that ends before it starts, which is not a light anyone asked for.
	got := watcherSettings(Config{})

	if got.FlashDuration != defaultFlashSeconds*time.Second {
		t.Errorf("FlashDuration = %v, want the default %ds", got.FlashDuration, defaultFlashSeconds)
	}
	if got.LobbyFlashDuration != defaultLobbyFlashSeconds*time.Second {
		t.Errorf("LobbyFlashDuration = %v, want the default %ds", got.LobbyFlashDuration, defaultLobbyFlashSeconds)
	}
	if got.PauseSide != pauseSideBoth {
		t.Errorf("PauseSide = %q, want %q", got.PauseSide, pauseSideBoth)
	}
}
