package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

const settingsFormHTML = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>BlinkyBeacon Settings</title>
<style>
*{box-sizing:border-box}
body{font-family:system-ui,sans-serif;max-width:420px;margin:48px auto;padding:0 24px;color:#1a1a1a}
h1{font-size:1.25em;margin-bottom:24px}
h2{font-size:1.05em;margin:26px 0 14px;padding-top:18px;border-top:1px solid #e5e5e5}
.field{margin-bottom:18px}
label{display:block;font-size:.875em;font-weight:600;margin-bottom:6px}
input{width:100%%;padding:8px 10px;border:1px solid #ccc;border-radius:4px;font-size:1em}
input:focus{outline:none;border-color:#0078d4;box-shadow:0 0 0 2px #cce4f7}
.hint{font-size:.8em;color:#666;margin-top:5px}
.clear{font-size:.8em;color:#666;margin-top:7px;font-weight:400;display:block}
.clear input{width:auto;margin-right:6px;vertical-align:-1px}
button{background:#0078d4;color:#fff;border:none;padding:9px 22px;font-size:1em;border-radius:4px;cursor:pointer;margin-top:8px}
button:hover{background:#106ebe}
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
<div class="field">
  <label for="line_number">Line Number</label>
  <input id="line_number" name="line_number" type="number" value="%d" min="1" max="99">
  <div class="hint">The N in /line/N/ &mdash; Line A is 1, Line B is 2, and so on.</div>
</div>
<div class="field">
  <label for="token">Dashboard API Token</label>
  <input id="token" name="token" type="password" value="" autocomplete="off" spellcheck="false" placeholder="%s">
  <div class="hint">%s</div>
</div>
<button type="submit">Save &amp; Apply</button>
</form>
</body>
</html>`

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
		html.EscapeString(cfg.DashboardURL), cfg.LineNumber,
		html.EscapeString(placeholder), hint)
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

	newCfg := Config{Addr: addr, Port: port, DashboardURL: dashboardURL, LineNumber: line, APIToken: token}
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
