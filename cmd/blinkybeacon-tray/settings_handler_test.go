package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
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

			if strings.Contains(body, "<select") {
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

	if strings.Contains(body, "<select") {
		t.Errorf("an empty picker was rendered:\n%s", body)
	}
	if !strings.Contains(body, "no lines published") {
		t.Errorf("the page does not say the dashboard listed nothing:\n%s", body)
	}
	if !strings.Contains(body, `type="number" value="2"`) {
		t.Errorf("the typed number input did not come back:\n%s", body)
	}
}
