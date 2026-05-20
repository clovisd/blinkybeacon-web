package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockBeacon satisfies the local Beacon interface for testing without USB hardware.
type mockBeacon struct {
	spinCalled  bool
	flashCalled bool
	stopCalled  bool
	err         error
}

func (m *mockBeacon) Spin() error  { m.spinCalled = true; return m.err }
func (m *mockBeacon) Flash() error { m.flashCalled = true; return m.err }
func (m *mockBeacon) Stop() error  { m.stopCalled = true; return m.err }
func (m *mockBeacon) Close() error { return nil }

func TestHandleStatus_disconnected(t *testing.T) {
	state := NewAppState()
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["state"] != "idle" {
		t.Errorf("expected state=idle, got %v", resp["state"])
	}
	if resp["connected"] != false {
		t.Errorf("expected connected=false, got %v", resp["connected"])
	}
}

func TestHandleSpin_disconnected_returns503(t *testing.T) {
	state := NewAppState()
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodPost, "/spin", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestHandleSpin_connected_callsBeacon(t *testing.T) {
	state := NewAppState()
	mock := &mockBeacon{}
	state.SetBeacon(mock)
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodPost, "/spin", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !mock.spinCalled {
		t.Error("expected Spin() to be called on beacon")
	}
	sv, _, _ := state.Get()
	if sv != StateSpin {
		t.Errorf("expected state=spin after /spin, got %q", sv)
	}
}

func TestHandleFlash_connected_callsBeacon(t *testing.T) {
	state := NewAppState()
	mock := &mockBeacon{}
	state.SetBeacon(mock)
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodPost, "/flash", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !mock.flashCalled {
		t.Error("expected Flash() to be called on beacon")
	}
	sv, _, _ := state.Get()
	if sv != StateFlash {
		t.Errorf("expected state=flash after /flash, got %q", sv)
	}
}

func TestHandleStop_connected_callsBeacon(t *testing.T) {
	state := NewAppState()
	mock := &mockBeacon{}
	state.SetBeacon(mock)
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodPost, "/stop", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !mock.stopCalled {
		t.Error("expected Stop() to be called on beacon")
	}
	sv, _, _ := state.Get()
	if sv != StateIdle {
		t.Errorf("expected state=idle after /stop, got %q", sv)
	}
}

func TestHandleSpin_wrongMethod_returns405(t *testing.T) {
	state := NewAppState()
	srv := NewHTTPServer(state, "127.0.0.1", 1337, nil)

	req := httptest.NewRequest(http.MethodGet, "/spin", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
