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
.field{margin-bottom:18px}
label{display:block;font-size:.875em;font-weight:600;margin-bottom:6px}
input{width:100%%;padding:8px 10px;border:1px solid #ccc;border-radius:4px;font-size:1em}
input:focus{outline:none;border-color:#0078d4;box-shadow:0 0 0 2px #cce4f7}
.hint{font-size:.8em;color:#666;margin-top:5px}
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
// and required back on submit is exactly the missing proof. Without it a
// forged save could rebind the control API to 0.0.0.0 for the whole network.
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
	// The bind address is user-supplied and lands inside an HTML attribute.
	// /settings has no auth and can be bound to 0.0.0.0, so treat it as hostile.
	fmt.Fprintf(w, settingsFormHTML, h.csrf(), html.EscapeString(cfg.Addr), cfg.Port)
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

	newCfg := Config{Addr: addr, Port: port}
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
