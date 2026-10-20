//go:build unix

package commands

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

var signalWaits = []struct {
	name string
	wait func()
}{
	{"runUntilInterrupted", runUntilInterrupted},
	{"runWithTimeout", func() { runWithTimeout(time.Hour) }},
}

var waitSignals = []syscall.Signal{syscall.SIGINT, syscall.SIGTERM}

// Each case runs in a fresh copy of the test binary, named by this variable,
// so a signal sent for one case cannot end the next.
const signalCaseEnv = "FSBEACON_SIGNAL_CASE"

func TestSignalEndsWait(t *testing.T) {
	if c := os.Getenv(signalCaseEnv); c != "" {
		runSignalCase(t, c)
		return
	}
	for i, w := range signalWaits {
		for j, sig := range waitSignals {
			t.Run(w.name+"/"+sig.String(), func(t *testing.T) {
				cmd := exec.Command(os.Args[0], "-test.run=^TestSignalEndsWait$")
				cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%d %d", signalCaseEnv, i, j))
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("%v\n%s", err, out)
				}
				if !strings.Contains(string(out), "Caught SIGTERM, exiting.") {
					t.Fatalf("no exit message:\n%s", out)
				}
			})
		}
	}
}

// runSignalCase keeps signalling this process until the wait returns, so the
// outcome does not depend on when the wait starts listening.
func runSignalCase(t *testing.T, c string) {
	var i, j int
	if _, err := fmt.Sscan(c, &i, &j); err != nil {
		t.Fatalf("bad %s %q: %v", signalCaseEnv, c, err)
	}
	wait, sig := signalWaits[i].wait, waitSignals[j]

	// Listening here first keeps a signal sent before the wait listens from
	// ending the process.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, sig)

	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	// Nothing has been signalled yet, so the wait must still be waiting.
	select {
	case <-done:
		t.Fatal("returned before any signal")
	case <-time.After(50 * time.Millisecond):
	}
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	deadline := time.After(5 * time.Second)
	for {
		if err := syscall.Kill(os.Getpid(), sig); err != nil {
			t.Fatalf("sending %v: %v", sig, err)
		}
		select {
		case <-done:
			return
		case <-tick.C:
		case <-deadline:
			t.Fatalf("still waiting 5 s after the first %v", sig)
		}
	}
}
