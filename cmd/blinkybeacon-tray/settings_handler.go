package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
button{background:#0078d4;color:#fff;border:none;padding:9px 22px;font-size:1em;border-radius:4px;cursor:pointer;margin-top:8px}
button:hover{background:#106ebe}
</style>
</head>
<body>
<h1>BlinkyBeacon Settings</h1>
<form method="POST" action="/settings">
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
}

func (h *settingsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	cfg := loadConfig()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, settingsFormHTML, cfg.Addr, cfg.Port, cfg.DashboardURL, cfg.LineNumber)
}

func (h *settingsHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
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

	newCfg := Config{Addr: addr, Port: port, DashboardURL: dashboardURL, LineNumber: line}
	if err := saveConfig(newCfg); err != nil {
		http.Error(w, "failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	newListenAddr := fmt.Sprintf("%s:%d", addr, port)
	settingsURL := "http://" + newListenAddr + "/settings"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, settingsSavedHTML, settingsURL, newListenAddr, settingsURL)

	if h.onSave != nil {
		go h.onSave(newCfg)
	}
}
