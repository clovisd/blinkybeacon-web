package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type HTTPServer struct {
	state *AppState
	srv   *http.Server
	addr  string
	port  int
}

type statusResponse struct {
	State     StateValue `json:"state"`
	Connected bool       `json:"connected"`
}

func NewHTTPServer(state *AppState, addr string, port int, onConfigSave func(Config)) *HTTPServer {
	s := &HTTPServer{state: state, addr: addr, port: port}
	mux := http.NewServeMux()
	mux.HandleFunc("/spin", s.handleSpin)
	mux.HandleFunc("/flash", s.handleFlash)
	mux.HandleFunc("/stop", s.handleStop)
	mux.HandleFunc("/status", s.handleStatus)

	sh := &settingsHandler{onSave: onConfigSave}
	mux.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			sh.handleGet(w, r)
		case http.MethodPost:
			sh.handlePost(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	s.srv = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", addr, port),
		Handler: mux,
	}
	return s
}

func (s *HTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.srv.Handler.ServeHTTP(w, r)
}

func (s *HTTPServer) Start() error {
	return s.srv.ListenAndServe()
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func (s *HTTPServer) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func (s *HTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sv, connected, _ := s.state.Get()
	s.writeJSON(w, http.StatusOK, statusResponse{State: sv, Connected: connected})
}

func (s *HTTPServer) handleSpin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, connected, beacon := s.state.Get()
	if !connected {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon not connected"})
		return
	}
	if err := beacon.Spin(); err != nil {
		s.state.SetBeacon(nil)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon disconnected"})
		return
	}
	s.state.SetState(StateSpin)
	s.writeJSON(w, http.StatusOK, statusResponse{State: StateSpin, Connected: true})
}

func (s *HTTPServer) handleFlash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, connected, beacon := s.state.Get()
	if !connected {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon not connected"})
		return
	}
	if err := beacon.Flash(); err != nil {
		s.state.SetBeacon(nil)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon disconnected"})
		return
	}
	s.state.SetState(StateFlash)
	s.writeJSON(w, http.StatusOK, statusResponse{State: StateFlash, Connected: true})
}

func (s *HTTPServer) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	_, connected, beacon := s.state.Get()
	if !connected {
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon not connected"})
		return
	}
	if err := beacon.Stop(); err != nil {
		s.state.SetBeacon(nil)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "beacon disconnected"})
		return
	}
	s.state.SetState(StateIdle)
	s.writeJSON(w, http.StatusOK, statusResponse{State: StateIdle, Connected: true})
}
