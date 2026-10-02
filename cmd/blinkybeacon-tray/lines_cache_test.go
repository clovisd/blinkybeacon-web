package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// fakeClock is a clock that moves only when the test says so.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// countingLineList is the dashboard's GET /api/v0/lines, counting requests per
// bearer token, safe to read while page loads are still in flight.
type countingLineList struct {
	mu     sync.Mutex
	status int
	body   string
	hits   map[string]int // by Authorization header
}

func newCountingLineList(status int, body string) *countingLineList {
	return &countingLineList{status: status, body: body, hits: map[string]int{}}
}

func (s *countingLineList) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.hits[r.Header.Get("Authorization")]++
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(s.status)
	w.Write([]byte(s.body))
}

func (s *countingLineList) total() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, h := range s.hits {
		n += h
	}
	return n
}

func (s *countingLineList) sentWith(token string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits["Bearer "+token]
}

const pickerMarkup = `<select id="line_number" name="line_number">`

func TestSettingsForm_asksForTheLineListAtMostOncePerFiveSeconds(t *testing.T) {
	// GET /settings has no login and no CSRF of its own, so a web page the
	// operator visits can load it in a loop. Each load used to send the
	// dashboard a token-bearing request; now the answer is remembered.
	withTempConfig(t)
	dash := newCountingLineList(http.StatusOK, fourLines)
	srv := httptest.NewServer(dash)
	defer srv.Close()
	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srv.URL, LineNumber: 3, APIToken: testToken})

	clock := newFakeClock()
	h := &settingsHandler{client: srv.Client(), lineList: lineListCache{now: clock.now}}

	for i := 0; i < 3; i++ {
		if body := getSettings(t, h); !strings.Contains(body, pickerMarkup) {
			t.Fatalf("page load %d has no picker:\n%s", i+1, body)
		}
	}
	if n := dash.total(); n != 1 {
		t.Errorf("three page loads in a row sent %d requests, want 1", n)
	}

	clock.advance(5*time.Second - time.Nanosecond)
	if body := getSettings(t, h); !strings.Contains(body, pickerMarkup) {
		t.Fatalf("the remembered list did not render the picker:\n%s", body)
	}
	if n := dash.total(); n != 1 {
		t.Errorf("a page load just inside five seconds sent a request: %d in all, want 1", n)
	}

	clock.advance(time.Nanosecond)
	getSettings(t, h)
	if n := dash.total(); n != 2 {
		t.Errorf("a page load five seconds on sent %d requests in all, want 2 — the list is read again", n)
	}
}

func TestSettingsForm_remembersAFailedLineListLookUpForFiveSecondsToo(t *testing.T) {
	// Whatever the outcome. An unreachable or refusing dashboard must not make
	// every page load wait on it and ask again.
	cases := []struct {
		name, reason string
		status       int
	}{
		{"the token is refused", "token rejected", http.StatusUnauthorized},
		{"the dashboard is failing", "unreachable", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withTempConfig(t)
			dash := newCountingLineList(tc.status, `{"v":0,"error":"no"}`)
			srv := httptest.NewServer(dash)
			defer srv.Close()
			saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
				DashboardURL: srv.URL, LineNumber: 6, APIToken: testToken})

			clock := newFakeClock()
			h := &settingsHandler{client: srv.Client(), lineList: lineListCache{now: clock.now}}

			for i := 0; i < 3; i++ {
				if body := getSettings(t, h); !strings.Contains(body, tc.reason) {
					t.Fatalf("page load %d does not say %q:\n%s", i+1, tc.reason, body)
				}
			}
			if n := dash.total(); n != 1 {
				t.Errorf("three page loads sent %d requests to a failing dashboard, want 1", n)
			}

			clock.advance(5 * time.Second)
			getSettings(t, h)
			if n := dash.total(); n != 2 {
				t.Errorf("%d requests in all five seconds on, want 2 — a failure is retried after that", n)
			}
		})
	}
}

func TestSettingsForm_aNewURLOrTokenReadsTheLineListAfresh(t *testing.T) {
	// The answer belongs to one dashboard and one token. A save that changes
	// either must show the operator the new dashboard's list on the next page
	// load, not the old one's for another five seconds.
	withTempConfig(t)
	first := newCountingLineList(http.StatusOK, fourLines)
	srvA := httptest.NewServer(first)
	defer srvA.Close()
	second := newCountingLineList(http.StatusOK, fourLines)
	srvB := httptest.NewServer(second)
	defer srvB.Close()

	clock := newFakeClock() // never advanced: every load is inside the five seconds
	h := &settingsHandler{client: srvA.Client(), lineList: lineListCache{now: clock.now}}

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srvA.URL, LineNumber: 3, APIToken: testToken})
	getSettings(t, h)

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srvA.URL, LineNumber: 3, APIToken: "a-new-token"})
	getSettings(t, h)
	if n := first.sentWith("a-new-token"); n != 1 {
		t.Errorf("the new token was sent %d times, want 1 — a new token is a new list", n)
	}

	saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
		DashboardURL: srvB.URL, LineNumber: 3, APIToken: "a-new-token"})
	getSettings(t, h)
	if n := second.total(); n != 1 {
		t.Errorf("the new dashboard was asked %d times, want 1 — a new URL is a new list", n)
	}
	if n := first.total(); n != 2 {
		t.Errorf("the first dashboard was asked %d times in all, want 2 — once per token", n)
	}
}

func TestSettingsForm_pageLoadsDuringALineListRequestShareIt(t *testing.T) {
	// Five page loads land while the dashboard is still answering the first.
	// One request goes out, and all five pages render from its answer. On the
	// fake clock of a synctest bubble, so "while" is exact: the dashboard does
	// not answer until every page load is parked.
	synctest.Test(t, func(t *testing.T) {
		withTempConfig(t)
		release := make(chan struct{})
		dash := newCountingLineList(http.StatusOK, fourLines)
		slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
			dash.ServeHTTP(w, r)
		})
		saveConfig(Config{Addr: defaultAddr, Port: defaultPort,
			DashboardURL: "http://dashboard.test", LineNumber: 3, APIToken: testToken})

		h := &settingsHandler{client: &http.Client{Transport: inProcessTransport{slow}}}

		const loads = 5
		bodies := make([]string, loads)
		var wg sync.WaitGroup
		for i := range loads {
			wg.Go(func() {
				w := httptest.NewRecorder()
				h.handleGet(w, httptest.NewRequest(http.MethodGet, "/settings", nil))
				bodies[i] = w.Body.String()
			})
		}
		synctest.Wait() // every page load is now waiting on the dashboard
		close(release)
		wg.Wait()

		if n := dash.total(); n != 1 {
			t.Errorf("%d page loads in flight together sent %d requests, want 1", loads, n)
		}
		for i, body := range bodies {
			if !strings.Contains(body, pickerMarkup) {
				t.Errorf("page load %d did not render the shared answer:\n%s", i+1, body)
			}
		}
	})
}
