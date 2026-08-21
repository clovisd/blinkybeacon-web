package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const settingsFormHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>BlinkyBeacon Settings</title>
<style>
*{box-sizing:border-box}
body{font-family:system-ui,sans-serif;max-width:560px;margin:48px auto;padding:0 24px;color:#1a1a1a}
h1{font-size:1.25em;margin-bottom:24px}
h2{font-size:1.05em;margin:26px 0 14px;padding-top:18px;border-top:1px solid #e5e5e5}
.field{margin-bottom:18px}
label{display:block;font-size:.875em;font-weight:600;margin-bottom:6px}
input,select{width:100%%;padding:8px 10px;border:1px solid #ccc;border-radius:4px;font-size:1em;background:#fff}
input:focus,select:focus{outline:none;border-color:#0078d4;box-shadow:0 0 0 2px #cce4f7}
select option:disabled{color:#999}
.hint{font-size:.8em;color:#666;margin-top:5px}
.clear{font-size:.8em;color:#666;margin-top:7px;font-weight:400;display:block}
.clear input{width:auto;margin-right:6px;vertical-align:-1px}
button{background:#0078d4;color:#fff;border:none;padding:9px 22px;font-size:1em;border-radius:4px;cursor:pointer;margin-top:8px}
button:hover{background:#106ebe}
table{width:100%%;border-collapse:collapse;font-size:.875em;margin:12px 0}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid #eee;vertical-align:top}
th{font-weight:600;color:#444}
code,pre{font-family:ui-monospace,Consolas,monospace;font-size:.9em}
code{background:#f3f3f3;padding:1px 4px;border-radius:3px}
pre{background:#f3f3f3;padding:10px 12px;border-radius:4px;overflow-x:auto}
.api p{font-size:.875em;color:#444;margin:8px 0}
</style>
</head>
<body>
<h1>BlinkyBeacon Settings</h1>
<form method="POST" action="/settings">
<input type="hidden" name="csrf" value="%s">
<div class="field">
  <label for="addr">Bind Address</label>
  <input id="addr" name="addr" type="text" value="%s" placeholder="127.0.0.1">
  <div class="hint">127.0.0.1 = local only &nbsp;&#124;&nbsp; 0.0.0.0 = all network interfaces</div>
</div>
<div class="field">
  <label for="port">Port</label>
  <input id="port" name="port" type="number" value="%d" min="1" max="65535">
</div>
<h2>Dashboard Watcher</h2>
<div class="field">
  <label for="dashboard_url">Dashboard URL</label>
  <input id="dashboard_url" name="dashboard_url" type="text" value="%s" placeholder="http://192.168.1.50:8080">
  <div class="hint">The LIVE Dashboard address. Leave blank to turn the watcher off and drive the beacon by hand.</div>
</div>
%s
<div class="field">
  <label for="token">Dashboard API Token</label>
  <input id="token" name="token" type="password" value="" autocomplete="off" spellcheck="false" placeholder="%s">
  <div class="hint">%s</div>
</div>
<h2>Beacon Light</h2>
%s
<button type="submit">Save &amp; Apply</button>
</form>
%s
</body>
</html>`

// apiDocsHTMLTmpl documents the control API. It is rendered with the live base
// URL so the examples are copy-pasteable, not placeholders. Every route here
// is a contract: the Bitfocus Companion module (clovisd/blinkybeacon-companion)
// drives /spin, /flash and /stop and polls /status, all unauthenticated, so
// none of them may change shape, grow a login, or move — on this branch as
// much as on the generic one.
const apiDocsHTMLTmpl = `<section class="api">
<h2>HTTP API</h2>
<p>Anything that can make an HTTP request can drive the beacon. The server is listening on <code>%[1]s</code>. No authentication; bind to <code>127.0.0.1</code> unless other machines need control.</p>
<table>
<tr><th>Method</th><th>Path</th><th>What it does</th></tr>
<tr><td><code>POST</code></td><td><code>/spin</code></td><td>Start the rotating light.</td></tr>
<tr><td><code>POST</code></td><td><code>/flash</code></td><td>Start the strobe.</td></tr>
<tr><td><code>POST</code></td><td><code>/stop</code></td><td>Turn the light off.</td></tr>
<tr><td><code>GET</code></td><td><code>/status</code></td><td>Current state and whether a beacon is plugged in.</td></tr>
<tr><td><code>GET</code></td><td><code>/settings</code></td><td>This page.</td></tr>
</table>
<p>Every route answers JSON. The three control routes and <code>/status</code> all return the same shape:</p>
<pre>{"state": "idle" | "spin" | "flash", "connected": true | false}</pre>
<p>A control request while no beacon is connected answers <code>503</code> with <code>{"error": "beacon not connected"}</code>; if the beacon drops mid-command it answers <code>503</code> with <code>{"error": "beacon disconnected"}</code> and the app starts looking for it again. A wrong method answers <code>405</code>.</p>
<pre>curl -X POST %[1]s/spin
curl -X POST %[1]s/flash
curl -X POST %[1]s/stop
curl %[1]s/status</pre>
<p><strong>While the dashboard watcher is bound to a line it drives the light itself.</strong> These routes still work, but the watcher corrects the light on its next poll — it decides from the dashboard, every time. Blank the dashboard URL (or forget the token) to hand the beacon back to manual control.</p>
<p>The <a href="https://github.com/clovisd/blinkybeacon-companion">Bitfocus Companion module</a> uses exactly these routes, polling <code>/status</code> every two seconds.</p>
</section>`

const settingsSavedHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>BlinkyBeacon Settings</title>
<meta http-equiv="refresh" content="2;url=%s">
<style>body{font-family:system-ui,sans-serif;max-width:420px;margin:48px auto;padding:0 24px}</style>
</head>
<body>
<h1>Settings Saved</h1>
<p>HTTP server restarting on <strong>%s</strong>&hellip;</p>
<p><a href="%s">Click here if not redirected automatically</a></p>
</body>
</html>`

type settingsHandler struct {
	onSave func(Config)

	// client is who asks the dashboard for its line list. A field so tests can
	// point it at a fixture; nil means the default, which is every real build.
	client *http.Client

	csrfOnce sync.Once
	csrfTok  string
}

// csrf is this process's settings-form token, minted on first use.
//
// /settings has no login, so the only thing separating "the operator clicked
// Save" from "a web page the operator happened to visit auto-submitted a form
// at 127.0.0.1:1337" is proof that whoever is posting could also READ the form.
// A cross-origin page cannot: the browser will happily send it a form POST, but
// will not let it see the GET response. So a random value planted in the form
// and required back on submit is exactly the missing proof.
//
// This matters far more since the config gained a token. A forged save could
// otherwise repoint dashboard_url at an attacker's host while leaving the token
// field blank — and the watcher would put the dashboard credential in an
// Authorization header to that host on its very next poll.
func (h *settingsHandler) csrf() string {
	h.csrfOnce.Do(func() {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return // leave it empty: csrfOK then refuses everything
		}
		h.csrfTok = hex.EncodeToString(b)
	})
	return h.csrfTok
}

// csrfOK reports whether a submitted token matches. It fails closed: if we
// never got randomness, nothing is accepted, because a settings form that
// cannot be defended is worse than one that cannot be saved.
func (h *settingsHandler) csrfOK(got string) bool {
	want := h.csrf()
	if want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// apiBaseURL is the address a CLIENT calls. 0.0.0.0 is what you bind, not
// what you connect to, so the examples fall back to loopback for it.
func apiBaseURL(cfg Config) string {
	host := cfg.Addr
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = defaultAddr
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]" // a bare IPv6 literal needs brackets in a URL
	}
	return fmt.Sprintf("http://%s:%d", host, cfg.Port)
}

// apiDocsHTML renders the API section for the saved config. The base URL is
// derived from the user-supplied bind address, so it is escaped like the rest.
func apiDocsHTML(cfg Config) string {
	return fmt.Sprintf(apiDocsHTMLTmpl, html.EscapeString(apiBaseURL(cfg)))
}

func (h *settingsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	cfg := loadConfig()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// The token is NEVER rendered — not even into a password input, whose
	// masking is purely visual. /settings has no auth and can be bound to
	// 0.0.0.0, so anything on this page is readable by anyone who can reach it,
	// and the dashboard credential is not going to be one of those things.
	placeholder, hint := tokenFieldText(cfg.APIToken)
	// Everything else here is user-supplied and lands inside an HTML attribute,
	// so treat it as hostile.
	fmt.Fprintf(w, settingsFormHTML,
		h.csrf(),
		html.EscapeString(cfg.Addr), cfg.Port,
		html.EscapeString(cfg.DashboardURL), h.lineField(cfg),
		html.EscapeString(placeholder), hint,
		beaconLightFields(cfg),
		apiDocsHTML(cfg))
}

// tokenFieldText is what the token field says about the stored token without
// disclosing any of it: whether one is saved, and what a blank submit means.
//
// It takes the stored token and returns none of it. Both return values are
// fixed strings chosen by this function; the argument only ever picks between
// them. The hint deliberately carries markup and is rendered as such, so it
// must stay that way — the moment any part of a stored value reaches these
// strings, this becomes an injection point.
func tokenFieldText(saved string) (placeholder, hint string) {
	if strings.TrimSpace(saved) == "" {
		return "paste the token from the dashboard",
			"Minted on the dashboard under Settings &rarr; Integrations (admin only). " +
				"Without it the watcher does not poll at all."
	}
	return "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (saved)",
		"A token is <strong>saved</strong>. Leave this blank to keep it, or paste a new one to replace it." +
			`<label class="clear"><input type="checkbox" name="token_clear" value="1"> Forget the saved token</label>`
}

func (h *settingsHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !h.csrfOK(r.FormValue("csrf")) {
		http.Error(w, "this settings form is stale or was not served by this app — "+
			"reopen Settings from the tray menu and try again", http.StatusForbidden)
		return
	}

	addr := strings.TrimSpace(r.FormValue("addr"))
	if addr == "" {
		addr = defaultAddr
	}

	port, err := strconv.Atoi(strings.TrimSpace(r.FormValue("port")))
	if err != nil || port < 1 || port > 65535 {
		port = defaultPort
	}

	// A blank dashboard URL is meaningful: it turns the watcher off.
	dashboardURL := strings.TrimSpace(r.FormValue("dashboard_url"))

	line, err := strconv.Atoi(strings.TrimSpace(r.FormValue("line_number")))
	if err != nil || line < 1 || line > 99 {
		line = defaultLineNumber
	}

	// The three beacon-light settings. Unlike addr, port and line_number above,
	// a value that IS there and is wrong is refused rather than quietly
	// replaced with the default: the operator is watching this page, and
	// telling them their 900 became 15 is the difference between a setting and
	// a suggestion. An ABSENT field is not a mistake — it is a form that
	// predates v0.6.0 — and lands on the documented default like everything
	// else here.
	flashSeconds, err := formSeconds(r.FormValue("flash_seconds"), "flash_seconds", defaultFlashSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	lobbyFlashSeconds, err := formSeconds(r.FormValue("lobby_flash_seconds"), "lobby_flash_seconds", defaultLobbyFlashSeconds)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// A browser sends nothing at all for an unticked checkbox, so absent is the
	// only way "off" is ever expressed.
	lobbyFlash := r.FormValue("lobby_flash") != ""

	pauseSide := strings.TrimSpace(r.FormValue("pause_side"))
	if pauseSide == "" {
		pauseSide = defaultPauseSide
	}
	if !validPauseSide(pauseSide) {
		http.Error(w, fmt.Sprintf("pause_side must be %q, %q or %q",
			pauseSideBoth, pauseSideRadiant, pauseSideDire), http.StatusBadRequest)
		return
	}

	// The token field renders empty every time, because it is never echoed
	// back. So a blank submit means "leave it alone" — otherwise changing the
	// port would silently unbind the watcher. Clearing it is a deliberate act.
	saved := loadConfig()
	token := strings.TrimSpace(r.FormValue("token"))
	switch {
	case token != "":
		// A typed value wins, even alongside the checkbox: the operator is
		// replacing the token, not forgetting it.
	case r.FormValue("token_clear") != "":
		token = ""
	case dashboardURL != saved.DashboardURL:
		// A token minted by one dashboard is not a credential for another.
		// Carrying it across a URL change would be the whole exfiltration
		// primitive in one line, so a new host starts unbound and the operator
		// pastes the token for it deliberately.
		token = ""
	default:
		token = saved.APIToken
	}

	newCfg := Config{
		Addr: addr, Port: port, DashboardURL: dashboardURL, LineNumber: line, APIToken: token,
		FlashSeconds:      flashSeconds,
		LobbyFlash:        lobbyFlash,
		LobbyFlashSeconds: lobbyFlashSeconds,
		PauseSide:         pauseSide,
	}
	if err := saveConfig(newCfg); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newListenAddr := fmt.Sprintf("%s:%d", addr, port)
	settingsURL := "http://" + newListenAddr + "/settings"

	// Escaped for the same reason as the form above; the CONFIG keeps the raw
	// value, only this rendering is escaped.
	safeAddr := html.EscapeString(newListenAddr)
	safeURL := html.EscapeString(settingsURL)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, settingsSavedHTML, safeURL, safeAddr, safeURL)

	if h.onSave != nil {
		go h.onSave(newCfg)
	}
}

// beaconLightFields renders the three v0.6.0 controls: how long the draft-end
// flash runs, whether and how long a new lobby flashes, and whose pauses spin
// the light.
//
// Every value here is an int or one of three fixed words by the time it lands —
// loadConfig has already defaulted anything else away — so unlike the fields
// above there is no operator string to escape. The moment that stops being true
// this needs html.EscapeString like everything else on the page.
func beaconLightFields(cfg Config) string {
	return fmt.Sprintf(`<div class="field">
  <label for="flash_seconds">Draft-End Flash</label>
  <input id="flash_seconds" name="flash_seconds" type="number" value="%d" min="%d" max="%d">
  <div class="hint">Seconds the beacon flashes when the draft ends. %d&ndash;%d.</div>
</div>
<div class="field">
  <label for="lobby_flash_seconds">New-Lobby Flash</label>
  <label class="clear"><input id="lobby_flash" name="lobby_flash" type="checkbox" value="1"%s> Flash when a new lobby is detected</label>
  <input id="lobby_flash_seconds" name="lobby_flash_seconds" type="number" value="%d" min="%d" max="%d">
  <div class="hint">Seconds to flash when a fresh match appears on the line while the game is still before or in the draft. %d&ndash;%d.</div>
</div>
<div class="field">
  <label for="pause_side">Spin For Pauses From</label>
  <select id="pause_side" name="pause_side">
%s  </select>
  <div class="hint">Picking a side needs a dashboard that publishes <code>pause_party</code> (<strong>v3.101.0</strong> or newer). On an older one, Radiant and Dire never spin for a pause at all.</div>
</div>`,
		cfg.FlashSeconds, minFlashSeconds, maxFlashSeconds, minFlashSeconds, maxFlashSeconds,
		checkedAttr(cfg.LobbyFlash),
		cfg.LobbyFlashSeconds, minFlashSeconds, maxFlashSeconds, minFlashSeconds, maxFlashSeconds,
		pauseSideOptions(cfg.PauseSide))
}

// checkedAttr is the checkbox's state, as the attribute HTML spells it.
func checkedAttr(on bool) string {
	if on {
		return " checked"
	}
	return ""
}

// pauseSideOptions renders the three sides with the saved one selected. The
// words are this function's, not the config's — the value attribute is what
// round-trips, and it is one of three constants.
func pauseSideOptions(saved string) string {
	sides := []struct{ value, label string }{
		{pauseSideBoth, "Both sides &mdash; every pause"},
		{pauseSideRadiant, "Radiant only"},
		{pauseSideDire, "Dire only"},
	}
	var b strings.Builder
	for _, s := range sides {
		selected := ""
		if s.value == saved {
			selected = " selected"
		}
		fmt.Fprintf(&b, "    <option value=\"%s\"%s>%s</option>\n", s.value, selected, s.label)
	}
	return b.String()
}

// formSeconds reads one flash duration off the form.
//
// Blank or absent means "the default": the form always renders both of these
// with a value, so a submit without one is not the operator's browser doing
// normal work — it is something older, and the documented default is the
// predictable place for it to land. Anything else out of range is refused, by
// name, so the page can say which field it was.
func formSeconds(raw, field string, def int) (int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || !validFlashSeconds(n) {
		return 0, fmt.Errorf("%s must be a whole number of seconds between %d and %d, got %q",
			field, minFlashSeconds, maxFlashSeconds, v)
	}
	return n, nil
}

// linesFetchTimeout bounds the settings page's look-up of the dashboard's line
// list. Short on purpose: the page must render whatever happens, so a dashboard
// that is slow to answer costs the operator a picker, never a settings window
// that hangs on them.
const linesFetchTimeout = 3 * time.Second

// lineField renders the form's line control: the picker when the dashboard
// answered with its lines, and the typed number exactly as v0.3.0 had it when
// it did not. The page renders either way — this is a settings window, and it
// must open even when the thing it configures is unreachable.
func (h *settingsHandler) lineField(cfg Config) string {
	client := h.client
	if client == nil {
		client = &http.Client{Timeout: linesFetchTimeout}
	}
	ctx, cancel := context.WithTimeout(context.Background(), linesFetchTimeout)
	defer cancel()

	lines, err := fetchLines(ctx, client, cfg.DashboardURL, cfg.APIToken)
	switch {
	case err != nil:
		return lineNumberFieldHTML(cfg.LineNumber, pickerUnavailableReason(err))
	case len(lines) == 0:
		// The look-up worked and there was nothing in it. An empty dropdown is
		// a control the operator can neither use nor explain.
		return lineNumberFieldHTML(cfg.LineNumber, "no lines published")
	}
	return linePickerHTML(lines, cfg.LineNumber)
}

// pickerUnavailableReason names, in three words or so, why there is no picker.
// The operator is looking at a page missing a control they were told to expect,
// and "token rejected" and "unreachable" want completely different next moves.
//
// A dashboard URL that has not been filled in yet is not a failure and gets no
// reason: the field above is empty and says so. Everything that is neither a
// refused token nor a missing one is called unreachable, including a dashboard
// too old to serve the endpoint at all — from this page they are the same fact,
// which is that no list came back.
func pickerUnavailableReason(err error) string {
	switch {
	case errors.Is(err, errNoDashboardURL):
		return ""
	case errors.Is(err, errNoToken):
		return "no token"
	}
	var pe *pollError
	if errors.As(err, &pe) && pe.Status == http.StatusUnauthorized {
		return "token rejected"
	}
	return "unreachable"
}

// lineNumberFieldHTML is the v0.3.0 control, kept whole: type the number. The
// reason, when there is one, rides above the hint that was always here.
func lineNumberFieldHTML(saved int, reason string) string {
	note := ""
	if reason != "" {
		note = fmt.Sprintf("<strong>Line list unavailable (%s)</strong> &mdash; type the number instead. ",
			html.EscapeString(reason))
	}
	return fmt.Sprintf(`<div class="field">
  <label for="line_number">Line Number</label>
  <input id="line_number" name="line_number" type="number" value="%d" min="1" max="99">
  <div class="hint">%sThe N in /line/N/ &mdash; Line A is 1, Line B is 2, and so on.</div>
</div>`, saved, note)
}

// linePickerHTML renders the dashboard's own lines as a picker.
//
// Every label here came off the wire from a host the operator typed in, and
// lands as HTML text on a page with no auth — so every one of them is escaped.
func linePickerHTML(lines []lineSummary, saved int) string {
	var b strings.Builder
	b.WriteString(`<div class="field">
  <label for="line_number">Line</label>
<select id="line_number" name="line_number">
`)
	listed := false
	for _, l := range lines {
		if l.N == nil {
			// Shown, and unselectable. A dashboard below v3.98.0 has not
			// allocated line numbers, so there is nothing to bind to — but
			// dropping the line would leave the operator hunting for a line
			// that is plainly there on their dashboard, with no clue that the
			// dashboard, not the beacon, is what needs upgrading.
			fmt.Fprintf(&b, "<option disabled>%s \u00b7 no number yet (dashboard &lt; v3.98.0)</option>\n",
				html.EscapeString(l.Label))
			continue
		}
		selected := ""
		if *l.N == saved {
			selected = " selected"
			listed = true
		}
		// The page declares charset=utf-8, so the separator goes out as itself
		// rather than as an entity — the option text is generated, and reading
		// it back should look like what the operator sees.
		fmt.Fprintf(&b, "<option value=\"%d\"%s>%s \u00b7 %s</option>\n",
			*l.N, selected, html.EscapeString(l.Label), runningWord(l.Running))
	}
	if !listed && saved >= 1 {
		// The line this beacon is bound to is not in the list — renamed,
		// retired, or invisible to this token. Keep it, selected: a picker that
		// quietly dropped it would rebind the beacon to whichever line happened
		// to be first the moment the operator pressed Save, having come to the
		// page to change something else entirely.
		fmt.Fprintf(&b, "<option value=\"%d\" selected>Line %d (not in the dashboard's list)</option>\n",
			saved, saved)
	}
	b.WriteString(`</select>
  <div class="hint">Your dashboard's own list, read just now. Change the URL or token above and save to re-read it.</div>
</div>`)
	return b.String()
}

// runningWord is what the picker says a line is doing right now.
func runningWord(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}
