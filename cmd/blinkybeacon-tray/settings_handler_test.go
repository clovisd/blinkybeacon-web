package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// withTempConfig points the config file at a disposable directory for one test.
func withTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	orig := configDir
	configDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { configDir = orig })
}

// submitSettings does what a browser does: GET the form, return its CSRF token
// with the POST.
func submitSettings(t *testing.T, h *settingsHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	get := httptest.NewRecorder()
	h.handleGet(get, httptest.NewRequest(http.MethodGet, "/settings", nil))
	m := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(get.Body.String())
	if m == nil {
		t.Fatalf("settings form carries no CSRF token:\n%s", get.Body.String())
	}
	form.Set("csrf", m[1])
	return postSettings(t, h, form)
}

// postSettings submits exactly the values given — no CSRF token is added, so a
// test can forge one, omit one, or send a stale one.
func postSettings(t *testing.T, h *settingsHandler, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/settings", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.handlePost(w, req)
	return w
}

// ------------------------------------------------------------- round trip

func TestSettingsPost_savesAddrAndPort(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	w := submitSettings(t, h, url.Values{"addr": {" 0.0.0.0 "}, "port": {"8080"}})

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200:\n%s", w.Code, w.Body.String())
	}
	got := loadConfig()
	if got.Addr != "0.0.0.0" || got.Port != 8080 {
		t.Errorf("saved config = %+v, want addr 0.0.0.0 port 8080", got)
	}
}

func TestSettingsPost_fallsBackToDefaultsOnNonsense(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	submitSettings(t, h, url.Values{"addr": {""}, "port": {"99999"}})

	got := loadConfig()
	if got.Addr != defaultAddr || got.Port != defaultPort {
		t.Errorf("saved config = %+v, want the defaults", got)
	}
}

// ----------------------------------------------------------------- escaping

func TestSettingsForm_escapesTheSavedBindAddress(t *testing.T) {
	// /settings has no auth and can be bound to 0.0.0.0, so the stored value
	// is attacker-reachable. It lands inside an HTML attribute.
	withTempConfig(t)
	const inject = `" onfocus="alert(1)` + `x`
	saveConfig(Config{Addr: inject, Port: defaultPort})

	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))

	if strings.Contains(w.Body.String(), `onfocus="alert(1)`) {
		t.Errorf("settings form reflected an unescaped attribute break:\n%s", w.Body.String())
	}
}

func TestSettingsSavedPage_escapesTheBindAddress(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	w := submitSettings(t, h, url.Values{"addr": {`" onfocus="alert(1)`}, "port": {"1337"}})

	if strings.Contains(w.Body.String(), `onfocus="alert(1)`) {
		t.Errorf("saved page reflected an unescaped attribute break:\n%s", w.Body.String())
	}
}

// ------------------------------------- forging a settings save from a page

func TestSettingsForm_carriesACSRFToken(t *testing.T) {
	withTempConfig(t)
	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))

	m := regexp.MustCompile(`name="csrf" value="([^"]+)"`).FindStringSubmatch(w.Body.String())
	if m == nil {
		t.Fatalf("settings form has no CSRF token:\n%s", w.Body.String())
	}
	if len(m[1]) < 32 {
		t.Errorf("CSRF token %q is too short to be unguessable", m[1])
	}
}

func TestSettingsPost_refusesASubmitWithNoCSRFToken(t *testing.T) {
	// The exploit this blocks: a page the operator happens to visit auto-POSTs
	// a form to http://127.0.0.1:1337/settings that rebinds the server to
	// 0.0.0.0, exposing the beacon's control API to the whole network.
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort})

	h := &settingsHandler{}
	w := postSettings(t, h, url.Values{"addr": {"0.0.0.0"}, "port": {"1337"}})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a submit with no CSRF token", w.Code)
	}
	if got := loadConfig(); got.Addr != defaultAddr {
		t.Errorf("Addr = %q — a forged submit rebound the server", got.Addr)
	}
}

func TestSettingsPost_refusesAGuessedCSRFToken(t *testing.T) {
	withTempConfig(t)
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort})

	h := &settingsHandler{}
	w := postSettings(t, h, url.Values{"addr": {"0.0.0.0"}, "port": {"1337"}, "csrf": {"not-the-real-token"}})

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a wrong CSRF token", w.Code)
	}
	if got := loadConfig(); got.Addr != defaultAddr {
		t.Errorf("Addr = %q — a guessed token was accepted", got.Addr)
	}
}

// ------------------------------------------------ the HTTP API, documented

func TestSettingsForm_documentsTheHTTPAPI(t *testing.T) {
	// The settings page is the one place an operator already looks, so it is
	// where the control API is written down — every route, its method, and
	// what /status answers. The Companion module and any script drive these.
	withTempConfig(t)
	saveConfig(Config{Addr: "192.168.1.20", Port: 4242})

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
		"503",  // what a missing beacon answers
		"curl", // a copy-pasteable example
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
	saveConfig(Config{Addr: "0.0.0.0", Port: 1337})

	h := &settingsHandler{}
	w := httptest.NewRecorder()
	h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
	body := w.Body.String()

	if strings.Contains(body, "http://0.0.0.0:1337") {
		t.Errorf("API examples point at 0.0.0.0, which no client can call")
	}
	if !strings.Contains(body, "http://127.0.0.1:1337") {
		t.Errorf("API examples should fall back to 127.0.0.1 when bound to all interfaces:\n%s", body)
	}
}
