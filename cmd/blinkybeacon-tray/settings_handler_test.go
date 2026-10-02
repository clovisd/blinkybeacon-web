package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// ------------------------------------------------- the dashboard's line list

// stubLineList stands in for the dashboard's GET /api/v0/lines. It records what
// the settings page sent, so a test can prove where the token went.
type stubLineList struct {
	status   int
	body     string
	lastPath string
	lastAuth string
	hits     int
}

func (s *stubLineList) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.hits++
	s.lastPath = r.URL.Path
	s.lastAuth = r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.status)
	w.Write([]byte(s.body))
}

func newStubLineList(body string) *stubLineList {
	return &stubLineList{status: http.StatusOK, body: body}
}

// fourLines is the shape measured live on 2026-08-20: four official lines, all
// of them with a null n because that dashboard has not allocated numbers yet —
// plus, here, two that have.
const fourLines = `{"v":0,"lines":[
  {"v":0,"n":3,"label":"Line C","running":true,"match_id":"789","game_state":null,"paused":false,"seconds_since_gsi":0.5,"ts":1765500000,"draft_complete":null},
  {"v":0,"n":4,"label":"Line D","running":false,"match_id":null,"game_state":null,"paused":false,"seconds_since_gsi":null,"ts":1765500000,"draft_complete":null},
  {"v":0,"n":null,"label":"Line A","running":true,"match_id":null,"game_state":null,"paused":false,"seconds_since_gsi":null,"ts":1765500000,"draft_complete":null}
]}`

func TestFetchLines_decodesTheListIncludingANullNumber(t *testing.T) {
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()

	lines, err := fetchLines(context.Background(), srv.Client(), srv.URL, testToken)
	if err != nil {
		t.Fatalf("fetchLines: %v", err)
	}
	if len(lines) != 3 {
		t.Fatalf("decoded %d lines, want 3", len(lines))
	}
	if lines[0].N == nil || *lines[0].N != 3 {
		t.Errorf("first line's n = %v, want 3", lines[0].N)
	}
	if lines[0].Label != "Line C" || !lines[0].Running {
		t.Errorf("first line = %+v, want a running Line C", lines[0])
	}
	// The whole point of the pointer: a dashboard that has not allocated
	// numbers publishes null, and null must not arrive as line 0.
	if lines[2].N != nil {
		t.Errorf("third line's n = %v, want nil for a null number", *lines[2].N)
	}
}

// ------------------------------------------------------------- the picker

func TestSettingsForm_picksTheLineFromTheDashboardList(t *testing.T) {
	withTempConfig(t)
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 4, APIToken: testToken})

	body := getSettings(t, &settingsHandler{client: srv.Client()})

	if !strings.Contains(body, `<select id="line_number" name="line_number">`) {
		t.Errorf("settings form has no line picker:\n%s", body)
	}
	if !strings.Contains(body, `<option value="3">Line C · running</option>`) {
		t.Errorf("the running line is not offered as it is named:\n%s", body)
	}
	// Saved line 4, so that is the one the browser must come up showing —
	// otherwise opening Settings and pressing Save would rebind the beacon.
	if !strings.Contains(body, `<option value="4" selected>Line D · stopped</option>`) {
		t.Errorf("the saved line is not pre-selected:\n%s", body)
	}
	if strings.Contains(body, `id="line_number" name="line_number" type="number"`) {
		t.Errorf("the typed number input is still on the page beside the picker:\n%s", body)
	}
}

// getSettings renders the settings page once and hands back its body.
func getSettings(t *testing.T, h *settingsHandler) string {
	t.Helper()
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	if body := w.Body.String(); !strings.Contains(body, "%!") {
		return body
	}
	t.Fatalf("settings form has a broken format verb:\n%s", w.Body.String())
	return ""
}

func TestSettingsForm_showsALineWithNoNumberButRefusesToBindIt(t *testing.T) {
	// Measured live on 2026-08-20: a dashboard below v3.98.0 publishes every
	// line with n: null. Dropping those silently would leave an operator
	// hunting a line that is plainly on their dashboard; offering them would
	// bind the beacon to a number that does not exist.
	withTempConfig(t)
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 3, APIToken: testToken})

	body := getSettings(t, &settingsHandler{client: srv.Client()})

	const want = `<option disabled>Line A · no number yet (dashboard &lt; v3.98.0)</option>`
	if !strings.Contains(body, want) {
		t.Errorf("the numberless line is not offered as a disabled option:\nwant %s\ngot:\n%s", want, body)
	}
	if strings.Contains(body, `value="0"`) {
		t.Errorf("a null line number reached the form as line 0:\n%s", body)
	}
}

func TestSettingsForm_keepsASavedLineTheDashboardDidNotList(t *testing.T) {
	// The beacon is bound to line 7; the dashboard's list does not have it —
	// renamed, retired, or a token that no longer sees it. Rendering the picker
	// without line 7 would make pressing Save silently rebind the beacon to
	// whatever happened to be first.
	withTempConfig(t)
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 7, APIToken: testToken})

	body := getSettings(t, &settingsHandler{client: srv.Client()})

	const want = `<option value="7" selected>Line 7 (not in the dashboard's list)</option>`
	if !strings.Contains(body, want) {
		t.Errorf("the saved line was dropped from the picker:\nwant %s\ngot:\n%s", want, body)
	}
	if strings.Contains(body, `<option value="3" selected>`) {
		t.Errorf("the picker pre-selected a line the beacon is not bound to:\n%s", body)
	}
}

// --------------------------------------------- when the list cannot be had

func TestSettingsForm_fallsBackToTheTypedNumberAndSaysWhy(t *testing.T) {
	// v0.3.0's control, unchanged, whenever the picker cannot be built — plus
	// one line naming which of the three things went wrong, because "the picker
	// is missing" and "your token was revoked" are the same page otherwise.
	cases := []struct {
		name   string
		reason string
		setup  func(t *testing.T) (dashboardURL, token string, client *http.Client)
	}{
		{
			name:   "the token was refused",
			reason: "token rejected",
			setup: func(t *testing.T) (string, string, *http.Client) {
				stub := newStubLineList(`{"error":"unauthorized"}`)
				stub.status = http.StatusUnauthorized
				srv := httptest.NewServer(stub)
				t.Cleanup(srv.Close)
				return srv.URL, testToken, srv.Client()
			},
		},
		{
			name:   "nothing is listening",
			reason: "unreachable",
			setup: func(t *testing.T) (string, string, *http.Client) {
				srv := httptest.NewServer(newStubLineList(fourLines))
				url, client := srv.URL, srv.Client()
				srv.Close() // the dashboard is down, and the port with it
				return url, testToken, client
			},
		},
		{
			name:   "no token has been pasted yet",
			reason: "no token",
			setup: func(t *testing.T) (string, string, *http.Client) {
				srv := httptest.NewServer(newStubLineList(fourLines))
				t.Cleanup(srv.Close)
				return srv.URL, "", srv.Client()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempConfig(t)
			dashboardURL, token, client := tc.setup(t)
			saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
				DashboardURL: dashboardURL, LineNumber: 6, APIToken: token})

			body := getSettings(t, &settingsHandler{client: client})

			if strings.Contains(body, `<select id="line_number"`) {
				t.Errorf("a picker was rendered from a list we never got:\n%s", body)
			}
			if !strings.Contains(body, `<input id="line_number" name="line_number" type="number" value="6"`) {
				t.Errorf("the typed number input did not come back:\n%s", body)
			}
			if !strings.Contains(body, tc.reason) {
				t.Errorf("the page does not say %q:\n%s", tc.reason, body)
			}
		})
	}
}

func TestSettingsPost_savesTheLineWhetherOrNotThePickerRendered(t *testing.T) {
	// The picker is a rendering convenience; the POST contract is v0.3.0's.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: "http://127.0.0.1:1", LineNumber: 1, APIToken: testToken})

	h := &settingsHandler{}
	saved := make(chan Config, 1)
	h.onSave = func(c Config) { saved <- c }

	w := submitSettings(t, h, url.Values{
		"addr":          {defaultAddr},
		"port":          {"1337"},
		"dashboard_url": {"http://127.0.0.1:1"},
		"line_number":   {"5"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if got := loadConfig(); got.LineNumber != 5 {
		t.Errorf("LineNumber = %d, want 5", got.LineNumber)
	}
	// The same rebind v0.3.0 did: onSave is what main.go turns into restartCh.
	select {
	case c := <-saved:
		if c.LineNumber != 5 {
			t.Errorf("the rebind carried line %d, want 5", c.LineNumber)
		}
	case <-time.After(2 * time.Second):
		t.Error("saving the form did not trigger the watcher rebind")
	}
}

// -------------------------------------- where the token goes, and where not

func TestSettingsForm_sendsTheTokenOnlyToTheConfiguredDashboard(t *testing.T) {
	// The operator types the dashboard URL, and a typed URL can redirect. If
	// the picker's fetch followed one, it would hand the dashboard credential
	// to whatever host the redirect names — an exfiltration primitive reached
	// through the settings page, with the beacon doing the sending.
	withTempConfig(t)
	const secret = "sk-live-do-not-leak-me"

	elsewhere := newStubLineList(fourLines)
	other := httptest.NewServer(elsewhere)
	defer other.Close()

	dashboard := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+linesListPath, http.StatusFound)
	}))
	defer dashboard.Close()

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: dashboard.URL, LineNumber: 2, APIToken: secret})

	body := getSettings(t, &settingsHandler{client: dashboard.Client()})

	if elsewhere.lastAuth != "" {
		t.Errorf("the token was sent to a redirect target: %q", elsewhere.lastAuth)
	}
	if strings.Contains(body, secret) {
		t.Errorf("the settings page rendered the token:\n%s", body)
	}
}

func TestSettingsForm_neverWritesTheTokenToTheLog(t *testing.T) {
	// Same rule as the watch loop's: a failed look-up is worth logging, the
	// credential it was made with is not — operators paste logs into bug
	// reports.
	withTempConfig(t)
	const secret = "sk-live-do-not-log-me"

	var logs strings.Builder
	origOut, origFlags := log.Writer(), log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(origOut); log.SetFlags(origFlags) })

	stub := newStubLineList(`{"error":"unauthorized"}`)
	stub.status = http.StatusUnauthorized
	srv := httptest.NewServer(stub)
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 2, APIToken: secret})

	getSettings(t, &settingsHandler{client: srv.Client()})

	if strings.Contains(logs.String(), secret) {
		t.Errorf("the token reached the log:\n%s", logs.String())
	}
}

func TestSettingsForm_asksTheConfiguredDashboardForItsLines(t *testing.T) {
	withTempConfig(t)
	stub := newStubLineList(fourLines)
	srv := httptest.NewServer(stub)
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 3, APIToken: testToken})

	getSettings(t, &settingsHandler{client: srv.Client()})

	if stub.lastPath != linesListPath {
		t.Errorf("asked for %q, want %q", stub.lastPath, linesListPath)
	}
	// A bearer header, never a query string: bearer URLs land in reverse-proxy
	// access logs and headers do not.
	if stub.lastAuth != "Bearer "+testToken {
		t.Errorf("Authorization = %q, want the bearer token", stub.lastAuth)
	}
}

// ---------------------------------------------------- the tray's bound row

func TestBoundLineLabel_namesTheLineTheDashboardNamesIt(t *testing.T) {
	if got, want := boundLineLabel(WatchOK, 3, "Line C"), "Bound: Line C"; got != want {
		t.Errorf("boundLineLabel = %q, want %q", got, want)
	}
}

func TestBoundLineLabel_fallsBackToTheNumberUntilAPollAnswers(t *testing.T) {
	// The label only exists once a poll has come back. Until then the row says
	// what it actually knows — the number out of the config — rather than
	// inventing a name or going blank.
	if got, want := boundLineLabel(WatchOK, 3, ""), "Bound: line 3 (unnamed)"; got != want {
		t.Errorf("boundLineLabel = %q, want %q", got, want)
	}
}

func TestBoundLineLabel_saysUnboundWhenNoLineIsConfigured(t *testing.T) {
	if got, want := boundLineLabel(WatchOff, 1, "Line A"), "Unbound"; got != want {
		t.Errorf("with the watcher off, boundLineLabel = %q, want %q", got, want)
	}
	if got, want := boundLineLabel(WatchOK, 0, ""), "Unbound"; got != want {
		t.Errorf("before the first tick, boundLineLabel = %q, want %q", got, want)
	}
}

func TestAppState_watchLabelDoesNotOutliveItsLine(t *testing.T) {
	// Retarget to another line and the old line's name must go quiet at once.
	// A row reading "Bound: Line C" while the beacon follows line 4 is the
	// confident wrong answer, in words instead of light.
	app := NewAppState()
	app.SetWatchLine(3)
	app.SetWatchLabel(3, "Line C")
	if got := app.WatchLabel(); got != "Line C" {
		t.Errorf("WatchLabel = %q, want %q", got, "Line C")
	}

	app.SetWatchLine(4)
	if got := app.WatchLabel(); got != "" {
		t.Errorf("WatchLabel = %q after retargeting, want it to go quiet", got)
	}
}

func TestWatchLoop_recordsTheDashboardsNameForTheBoundLine(t *testing.T) {
	// The tray's bound row is only as good as the label behind it, and the
	// only place that label exists is the poll the watcher already makes.
	stub := newStubDashboard(livePayload) // labelled "Line A"
	srv := httptest.NewServer(stub)
	defer srv.Close()

	app := NewAppState()
	app.SetBeacon(&countingBeacon{})

	cfg := Config{DashboardURL: srv.URL, LineNumber: 1, APIToken: testToken}
	startWatchLoop(t, app, srv.Client(), func() Config { return cfg }, 5*time.Millisecond)

	waitFor(t, "the watcher to learn the line's name", func() bool {
		return app.WatchLabel() == "Line A"
	})
	if got := boundLineLabel(app.WatchStatus(), app.WatchLine(), app.WatchLabel()); got != "Bound: Line A" {
		t.Errorf("the tray's bound row reads %q, want %q", got, "Bound: Line A")
	}
}

func TestSettingsForm_fallsBackWhenTheDashboardListsNoLines(t *testing.T) {
	// The look-up worked and there was nothing in it — a dashboard with no
	// official lines yet. An empty dropdown is a control the operator cannot
	// use and cannot explain, so this is the typed number too, with its own
	// reason.
	withTempConfig(t)
	srv := httptest.NewServer(newStubLineList(`{"v":0,"lines":[]}`))
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 2, APIToken: testToken})

	body := getSettings(t, &settingsHandler{client: srv.Client()})

	if strings.Contains(body, `<select id="line_number"`) {
		t.Errorf("an empty picker was rendered:\n%s", body)
	}
	if !strings.Contains(body, "no lines published") {
		t.Errorf("the page does not say the dashboard listed nothing:\n%s", body)
	}
	if !strings.Contains(body, `type="number" value="2"`) {
		t.Errorf("the typed number input did not come back:\n%s", body)
	}
}

// -------------------------------------------- the three beacon-light settings

// beaconLightForm is a complete, valid submit of the whole form — the values a
// browser actually sends. Individual tests override the one field under test.
func beaconLightForm() url.Values {
	return url.Values{
		"addr":                {defaultAddr},
		"port":                {"1337"},
		"dashboard_url":       {""},
		"line_number":         {"1"},
		"flash_seconds":       {"15"},
		"lobby_flash_seconds": {"10"},
		"pause_side":          {pauseSideBoth},
	}
}

func TestSettingsForm_rendersTheSavedBeaconLightSettings(t *testing.T) {
	// (xi) Opening Settings must show what is actually saved — otherwise
	// pressing Save to change the port would quietly reset the light.
	withTempConfig(t)
	cfg := defaultConfig()
	cfg.FlashSeconds = 45
	cfg.LobbyFlash = true
	cfg.LobbyFlashSeconds = 7
	cfg.PauseSide = pauseSideDire
	saveConfig(cfg)

	body := getSettings(t, &settingsHandler{})

	for _, want := range []string{
		`id="flash_seconds" name="flash_seconds" type="number" value="45"`,
		`id="lobby_flash" name="lobby_flash" type="checkbox" value="1" checked`,
		`id="lobby_flash_seconds" name="lobby_flash_seconds" type="number" value="7"`,
		`<option value="dire" selected>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the settings form does not carry %s:\n%s", want, body)
		}
	}
	// The saved side is the selected one, and only it.
	if strings.Contains(body, `<option value="both" selected>`) {
		t.Errorf("the pause-side picker pre-selected a side the operator did not save:\n%s", body)
	}
}

func TestSettingsForm_rendersTheDefaultsOnAFreshInstall(t *testing.T) {
	withTempConfig(t)

	body := getSettings(t, &settingsHandler{})

	for _, want := range []string{
		`name="flash_seconds" type="number" value="15"`,
		`name="lobby_flash_seconds" type="number" value="10"`,
		`<option value="both" selected>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the settings form does not carry %s:\n%s", want, body)
		}
	}
	if strings.Contains(body, `type="checkbox" value="1" checked`) {
		t.Errorf("the new-lobby flash came up ticked on a fresh install:\n%s", body)
	}
}

func TestSettingsForm_saysTheSideFilterNeedsANewerDashboard(t *testing.T) {
	// An operator who picks Radiant on a dashboard that does not publish
	// pause_party gets a light that never spins for a pause again, with nothing
	// on screen to explain it. One line on the page is the whole fix.
	withTempConfig(t)

	body := getSettings(t, &settingsHandler{})

	if !strings.Contains(body, "pause_party") || !strings.Contains(body, "v3.101.0") {
		t.Errorf("the pause-side control does not say which dashboard it needs:\n%s", body)
	}
}

func TestSettingsPost_roundTripsAllThreeSettings(t *testing.T) {
	// (xi) Into the file, and out to the watcher, in one submit.
	withTempConfig(t)
	h := &settingsHandler{}
	saved := make(chan Config, 1)
	h.onSave = func(c Config) { saved <- c }

	form := beaconLightForm()
	form.Set("flash_seconds", "45")
	form.Set("lobby_flash", "1")
	form.Set("lobby_flash_seconds", "7")
	form.Set("pause_side", pauseSideRadiant)

	if w := submitSettings(t, h, form); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	got := loadConfig()
	if got.FlashSeconds != 45 || got.LobbyFlashSeconds != 7 {
		t.Errorf("saved durations = %d/%d, want 45/7", got.FlashSeconds, got.LobbyFlashSeconds)
	}
	if !got.LobbyFlash {
		t.Error("the new-lobby flash was not saved as on")
	}
	if got.PauseSide != pauseSideRadiant {
		t.Errorf("saved PauseSide = %q, want %q", got.PauseSide, pauseSideRadiant)
	}

	select {
	case c := <-saved:
		// The same rebind the line picker uses: onSave is what main.go turns
		// into restartCh, and restartCh is what rebuilds the watcher.
		if c.FlashSeconds != 45 || !c.LobbyFlash || c.LobbyFlashSeconds != 7 || c.PauseSide != pauseSideRadiant {
			t.Errorf("the rebind carried %+v, want the three settings just saved", c)
		}
	case <-time.After(2 * time.Second):
		t.Error("saving the form did not trigger the watcher rebind")
	}
}

func TestSettingsPost_anUntickedCheckboxTurnsTheLobbyFlashOff(t *testing.T) {
	// A browser sends nothing at all for an unticked checkbox, so "absent" is
	// the ONLY way off is ever expressed.
	withTempConfig(t)
	cfg := defaultConfig()
	cfg.LobbyFlash = true
	saveConfig(cfg)

	form := beaconLightForm() // no lobby_flash key
	if w := submitSettings(t, &settingsHandler{}, form); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	if loadConfig().LobbyFlash {
		t.Error("LobbyFlash is still on — an unticked checkbox never turned it off")
	}
}

func TestSettingsPost_rejectsAnOutOfRangeDuration(t *testing.T) {
	// Rejected, not silently defaulted: the operator is watching, and telling
	// them their 900 became 15 is the whole difference between a setting and a
	// suggestion.
	cases := []struct{ name, field, value string }{
		{"a draft flash past the bound", "flash_seconds", "601"},
		{"a draft flash of zero", "flash_seconds", "0"},
		{"a negative draft flash", "flash_seconds", "-5"},
		{"a lobby flash past the bound", "lobby_flash_seconds", "100000"},
		{"a lobby flash that is not a number", "lobby_flash_seconds", "ten"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempConfig(t)
			before := defaultConfig()
			before.FlashSeconds = 20
			before.LobbyFlashSeconds = 20
			saveConfig(before)

			form := beaconLightForm()
			form.Set(tc.field, tc.value)
			w := submitSettings(t, &settingsHandler{}, form)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 for %s=%q", w.Code, tc.field, tc.value)
			}
			if !strings.Contains(w.Body.String(), tc.field) {
				t.Errorf("the refusal does not name the field:\n%s", w.Body.String())
			}
			// And nothing was written: a refused save must not half-apply.
			if got := loadConfig(); got.FlashSeconds != 20 || got.LobbyFlashSeconds != 20 {
				t.Errorf("a refused POST still changed the config: %d/%d, want 20/20",
					got.FlashSeconds, got.LobbyFlashSeconds)
			}
		})
	}
}

func TestSettingsPost_rejectsAnUnknownPauseSide(t *testing.T) {
	for _, side := range []string{"Radiant", "team1", "none", "radiant;dire"} {
		withTempConfig(t)
		before := defaultConfig()
		before.PauseSide = pauseSideDire
		saveConfig(before)

		form := beaconLightForm()
		form.Set("pause_side", side)
		w := submitSettings(t, &settingsHandler{}, form)

		if w.Code != http.StatusBadRequest {
			t.Errorf("pause_side=%q: status = %d, want 400", side, w.Code)
		}
		if got := loadConfig().PauseSide; got != pauseSideDire {
			t.Errorf("pause_side=%q: a refused POST changed the config to %q", side, got)
		}
	}
}

func TestSettingsPost_treatsABlankPauseSideAsAbsent(t *testing.T) {
	// Whitespace is not an unknown side, it is a field that was not filled in —
	// the same reading addr, dashboard_url and both durations already get here.
	withTempConfig(t)
	form := beaconLightForm()
	form.Set("pause_side", "   ")

	if w := submitSettings(t, &settingsHandler{}, form); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if got := loadConfig().PauseSide; got != pauseSideBoth {
		t.Errorf("PauseSide = %q, want the default %q", got, pauseSideBoth)
	}
}

func TestSettingsPost_acceptsTheEndsOfTheRange(t *testing.T) {
	withTempConfig(t)
	form := beaconLightForm()
	form.Set("flash_seconds", "1")
	form.Set("lobby_flash_seconds", "600")

	if w := submitSettings(t, &settingsHandler{}, form); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if got := loadConfig(); got.FlashSeconds != 1 || got.LobbyFlashSeconds != 600 {
		t.Errorf("durations = %d/%d, want 1/600", got.FlashSeconds, got.LobbyFlashSeconds)
	}
}

func TestSettingsPost_aFormWithoutTheNewFieldsSavesTheDefaults(t *testing.T) {
	// A form from before v0.6.0 — a stale tab, a bookmarked POST. Absent is not
	// a refusable mistake, it is a form that predates the field, and the same
	// place addr, port and line_number already land is the documented default.
	withTempConfig(t)

	w := submitSettings(t, &settingsHandler{}, url.Values{
		"addr": {defaultAddr}, "port": {"1337"},
		"dashboard_url": {""}, "line_number": {"1"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	got := loadConfig()
	if got.FlashSeconds != defaultFlashSeconds || got.LobbyFlashSeconds != defaultLobbyFlashSeconds {
		t.Errorf("durations = %d/%d, want the defaults", got.FlashSeconds, got.LobbyFlashSeconds)
	}
	if got.PauseSide != pauseSideBoth || got.LobbyFlash {
		t.Errorf("PauseSide = %q, LobbyFlash = %v, want %q and off", got.PauseSide, got.LobbyFlash, pauseSideBoth)
	}
}

func TestSettingsPost_stillNeverEchoesTheToken(t *testing.T) {
	// The B1 rule, re-checked because this change adds fields to the same form
	// and the same POST: a beacon-light save must not disturb the credential.
	withTempConfig(t)
	const secret = "sk-live-do-not-leak-me"
	cfg := defaultConfig()
	cfg.DashboardURL = "http://127.0.0.1:1"
	cfg.APIToken = secret
	saveConfig(cfg)

	form := beaconLightForm()
	form.Set("dashboard_url", "http://127.0.0.1:1")
	form.Set("flash_seconds", "30")
	w := submitSettings(t, &settingsHandler{}, form)

	if strings.Contains(w.Body.String(), secret) {
		t.Errorf("the saved page rendered the token:\n%s", w.Body.String())
	}
	if got := loadConfig(); got.APIToken != secret {
		t.Errorf("APIToken = %q — saving the light settings disturbed the credential", got.APIToken)
	}
	if got := loadConfig().FlashSeconds; got != 30 {
		t.Errorf("FlashSeconds = %d, want 30", got)
	}
}

// ------------------------------------------------ the HTTP API, documented

func TestSettingsForm_documentsTheHTTPAPI(t *testing.T) {
	// The settings page is the one place an operator already looks, so it is
	// where the control API is written down — every route, its method, and
	// what /status answers. The Companion module and any script drive these.
	withTempConfig(t)
	cfg := defaultConfig()
	cfg.Addr, cfg.Port = "192.168.1.20", 4242
	saveConfig(cfg)

	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	body := w.Body.String()

	for _, want := range []string{
		"HTTP API",
		"http://192.168.1.20:4242",         // the live base URL, not a placeholder
		"POST", "/spin", "/flash", "/stop", // the control routes
		"GET", "/status", // the state route
		`"state"`, `"connected"`, // the /status JSON shape
		"idle", "spin", "flash", // the state vocabulary
		"503",     // what a missing beacon answers
		"curl",    // a copy-pasteable example
		"watcher", // what happens to a manual command while a line is bound
	} {
		if !strings.Contains(body, want) {
			t.Errorf("settings page does not document %q", want)
		}
	}
}

func TestSettingsForm_apiDocsFollowTheSavedBindAddress(t *testing.T) {
	// 0.0.0.0 is what you bind, not what you call; the example must name an
	// address a client can actually reach.
	withTempConfig(t)
	cfg := defaultConfig()
	cfg.Addr, cfg.Port = "0.0.0.0", 1337
	saveConfig(cfg)

	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	body := w.Body.String()

	if strings.Contains(body, "http://0.0.0.0:1337") {
		t.Errorf("API examples point at 0.0.0.0, which no client can call")
	}
	if !strings.Contains(body, "http://127.0.0.1:1337") {
		t.Errorf("API examples should fall back to 127.0.0.1 when bound to all interfaces")
	}
}

// ----------------------------------------- what the form does not carry

func TestSettingsPost_keepsAHandEditedPollInterval(t *testing.T) {
	// poll_interval_ms is a config-file-only key: the form has no field for it.
	// The POST builds its Config from the form, so unless the key is carried
	// over from the file, every save — of anything — quietly puts the
	// operator's hand edit back to the default.
	withTempConfig(t)
	srv := httptest.NewServer(newStubLineList(fourLines))
	defer srv.Close()
	writeRawConfig(t, strings.NewReplacer(
		`"https://dashboard.example.com"`, `"`+srv.URL+`"`,
		`"pause_side": "dire"`, `"pause_side": "dire",
  "poll_interval_ms": 750`,
	).Replace(v070ConfigFile))

	h := &settingsHandler{client: srv.Client()}
	saved := make(chan Config, 1)
	h.onSave = func(c Config) { saved <- c }

	form := beaconLightForm()
	form.Set("dashboard_url", srv.URL)
	form.Set("flash_seconds", "30") // the operator came to change something else
	if w := submitSettings(t, h, form); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}

	if got := loadConfig().PollIntervalMs; got != 750 {
		t.Errorf("PollIntervalMs = %d after a save, want the hand-edited 750", got)
	}
	select {
	case c := <-saved:
		if c.PollIntervalMs != 750 {
			t.Errorf("the rebind carried PollIntervalMs %d, want 750", c.PollIntervalMs)
		}
	case <-time.After(2 * time.Second):
		t.Error("saving the form did not trigger the watcher rebind")
	}
}

func TestSettingsPost_aFormFromAnEarlierProcessWithoutTheLightFieldsChangesNothing(t *testing.T) {
	// Every form this process renders carries all four light fields, and a
	// save restarts the server with a new handler and a new csrf token. So the
	// only form that can arrive without them is one an earlier process served
	// — a tab left open across an upgrade or a restart — and its token is one
	// this process never minted. Refused, and the file is not touched.
	withTempConfig(t)
	writeRawConfig(t, v070ConfigFile)
	path, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	earlier := &settingsHandler{} // the process that served the stale tab
	h := &settingsHandler{}
	saved := make(chan Config, 1)
	h.onSave = func(c Config) { saved <- c }

	w := postSettings(t, h, url.Values{
		"csrf":          {earlier.csrf()},
		"addr":          {"127.0.0.1"},
		"port":          {"1337"},
		"dashboard_url": {"https://dashboard.example.com"},
		"line_number":   {"2"},
	})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a form this process did not serve", w.Code)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("the config file changed:\nbefore %s\nafter  %s", before, after)
	}
	select {
	case c := <-saved:
		t.Errorf("a refused form still rebound the watcher with %+v", c)
	case <-time.After(50 * time.Millisecond):
	}
}
