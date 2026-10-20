//go:build windows || darwin

// beacon-probe lists every Farming Simulator beacon light plugged into this
// PC and reports what the HID layer knows about each one: path, serial
// number, manufacturer and product strings, interface and release numbers.
// With -flash it also flashes each light in turn for a couple of seconds,
// naming the one it is flashing, so you can see which physical light is
// which, and then stops them all.
//
// It changes no setting. The only file it writes is beacon-probe.txt beside
// the executable, which it appends to on every run so that several runs
// (before and after moving a light to another USB port, say) end up in one
// file you can send back.
//
// Quit the tray app before running this. The tray re-sends its own command
// to every light it holds every five seconds, which would fight the flash
// here; and on macOS the tray holds each light exclusively, so this program
// could not open them at all.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	hid "github.com/sstallion/go-hid"
)

// The beacon's USB identity and the two reports used here, copied from
// pkg/fsbeacon/usbhid.go so this module stands alone.
const (
	fs22vid uint16 = 0x340d
	fs22pid uint16 = 0x1710
)

var (
	magicFlash = []byte{0x0, 0xFF, 0x07, 0xFF, 0x50, 0xFF, 0x1C, 0x8E, 0xB0, 0xB8}
	magicStop  = []byte{0x0, 0xFF, 0x00, 0x00, 0x64, 0x00, 0x32, 0x9E, 0xD7, 0x0D}
)

const outputFileName = "beacon-probe.txt"

func main() {
	flashMode := flag.Bool("flash", false, "flash each beacon in turn, then stop them all")
	seconds := flag.Int("seconds", 2, "how long each beacon flashes in -flash mode")
	flag.Parse()

	// Double-clicked from Explorer there are no arguments and the console
	// window closes the moment this program exits, so in that case it waits
	// for Enter at the end.
	waitAtEnd := len(os.Args) == 1

	out, closeOut := openOutput()
	defer closeOut()

	exit := run(out, *flashMode, time.Duration(*seconds)*time.Second)

	if waitAtEnd {
		fmt.Fprint(os.Stdout, "\nPress Enter to close this window.")
		bufio.NewReader(os.Stdin).ReadString('\n')
	}
	os.Exit(exit)
}

// openOutput returns a writer that goes to the terminal and to the text file
// beside the executable. If the file cannot be opened the run still goes to
// the terminal, with a warning, rather than not at all.
func openOutput() (io.Writer, func()) {
	path := outputFileName
	if exe, err := os.Executable(); err == nil {
		path = filepath.Join(filepath.Dir(exe), outputFileName)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot write %s: %v (printing to the terminal only)\n", path, err)
		return os.Stdout, func() {}
	}
	fmt.Fprintf(os.Stdout, "Writing a copy of everything below to %s\n\n", path)
	return io.MultiWriter(os.Stdout, f), func() { f.Close() }
}

func run(out io.Writer, flashMode bool, hold time.Duration) int {
	printHeader(out, flashMode)

	if err := hid.Init(); err != nil {
		fmt.Fprintf(out, "HID init failed: %v\n", err)
		return 1
	}
	defer hid.Exit()

	var found []hid.DeviceInfo
	err := hid.Enumerate(fs22vid, fs22pid, func(info *hid.DeviceInfo) error {
		found = append(found, *info)
		return nil
	})
	if err != nil {
		fmt.Fprintf(out, "Enumeration failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(out, "Beacons found (vendor %04x, product %04x): %d\n\n", fs22vid, fs22pid, len(found))
	if len(found) == 0 {
		fmt.Fprintln(out, "No beacon is plugged in, or none could be read. Check the cables and run again.")
		return 1
	}

	for i, info := range found {
		printDevice(out, i+1, len(found), info)
	}
	printSummary(out, found)

	if flashMode {
		fmt.Fprintln(out)
		flashEach(out, found, hold)
	}
	fmt.Fprintln(out, "\nDone.")
	return 0
}

func printHeader(out io.Writer, flashMode bool) {
	mode := "list"
	if flashMode {
		mode = "flash"
	}
	host, _ := os.Hostname()
	fmt.Fprintf(out, "==== beacon-probe · %s · mode: %s ====\n", time.Now().Format("2006-01-02 15:04:05 MST"), mode)
	fmt.Fprintf(out, "PC: %s · %s/%s · HIDAPI %s · probe built from %s\n\n",
		host, runtime.GOOS, runtime.GOARCH, hid.GetVersionStr(), buildRevision())
}

// buildRevision is the git commit this executable was built from, as the Go
// toolchain stamps it, so the text file says which probe produced it.
func buildRevision() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "(unknown)"
	}
	rev, modified := "(unknown)", ""
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 12 {
				rev = s.Value[:12]
			} else {
				rev = s.Value
			}
		case "vcs.modified":
			if s.Value == "true" {
				modified = " (with uncommitted changes)"
			}
		}
	}
	return rev + modified
}

// printDevice reports one beacon: first what enumeration said about it, then
// what an open handle says, which on Windows and macOS can add a little.
func printDevice(out io.Writer, n, of int, info hid.DeviceInfo) {
	fmt.Fprintf(out, "--- beacon %d of %d ---\n", n, of)
	fmt.Fprintf(out, "  path              : %s\n", info.Path)
	fmt.Fprintf(out, "  serial number     : %s\n", orEmpty(info.SerialNbr))
	fmt.Fprintf(out, "  manufacturer      : %s\n", orEmpty(info.MfrStr))
	fmt.Fprintf(out, "  product           : %s\n", orEmpty(info.ProductStr))
	fmt.Fprintf(out, "  interface number  : %d\n", info.InterfaceNbr)
	fmt.Fprintf(out, "  release number    : 0x%04x\n", info.ReleaseNbr)
	fmt.Fprintf(out, "  usage page / usage: 0x%04x / 0x%04x\n", info.UsagePage, info.Usage)
	fmt.Fprintf(out, "  bus               : %s\n", info.BusType)
	if key := platformPortKey(info); key != "" {
		fmt.Fprintf(out, "  port key          : %s\n", key)
	}

	dev, err := hid.OpenPath(info.Path)
	if err != nil {
		fmt.Fprintf(out, "  open              : FAILED: %v\n", err)
		fmt.Fprintln(out)
		return
	}
	defer dev.Close()
	fmt.Fprintf(out, "  open              : ok\n")
	if s, err := dev.GetSerialNbr(); err == nil {
		fmt.Fprintf(out, "  serial (open)     : %s\n", orEmpty(s))
	} else {
		fmt.Fprintf(out, "  serial (open)     : error: %v\n", err)
	}
	if s, err := dev.GetMfrStr(); err == nil {
		fmt.Fprintf(out, "  manufacturer (open): %s\n", orEmpty(s))
	}
	if s, err := dev.GetProductStr(); err == nil {
		fmt.Fprintf(out, "  product (open)    : %s\n", orEmpty(s))
	}
	for _, line := range platformExtra(dev) {
		fmt.Fprintf(out, "  %s\n", line)
	}
	fmt.Fprintln(out)
}

// printSummary answers the two questions the design is waiting on: do these
// lights carry serial numbers, and are they distinct.
func printSummary(out io.Writer, found []hid.DeviceInfo) {
	serials := map[string]int{}
	empty := 0
	for _, info := range found {
		s := strings.TrimSpace(info.SerialNbr)
		if s == "" {
			empty++
			continue
		}
		serials[s]++
	}
	paths := map[string]int{}
	for _, info := range found {
		paths[info.Path]++
	}

	fmt.Fprintln(out, "--- summary ---")
	switch {
	case empty == len(found):
		fmt.Fprintln(out, "  serial numbers: NONE — no beacon reports one; the USB port is the only way to tell them apart")
	case empty > 0:
		fmt.Fprintf(out, "  serial numbers: MIXED — %d of %d beacons report none\n", empty, len(found))
	default:
		dup := 0
		for s, c := range serials {
			if c > 1 {
				dup++
				fmt.Fprintf(out, "  serial numbers: DUPLICATED — %q is reported by %d beacons\n", s, c)
			}
		}
		if dup == 0 {
			fmt.Fprintf(out, "  serial numbers: DISTINCT — every one of the %d beacons reports its own\n", len(found))
		}
	}
	if len(paths) == len(found) {
		fmt.Fprintf(out, "  paths: distinct (%d)\n", len(paths))
	} else {
		fmt.Fprintf(out, "  paths: NOT distinct — %d beacons share %d paths\n", len(found), len(paths))
	}
}

// flashEach opens every beacon first and holds all of them open together,
// which is itself one of the facts being checked, then flashes them one at a
// time and finally stops every one of them.
func flashEach(out io.Writer, found []hid.DeviceInfo, hold time.Duration) {
	type handle struct {
		n    int
		info hid.DeviceInfo
		dev  *hid.Device
	}
	var open []handle
	for i, info := range found {
		dev, err := hid.OpenPath(info.Path)
		if err != nil {
			fmt.Fprintf(out, "beacon %d: open FAILED, skipping it: %v\n", i+1, err)
			continue
		}
		open = append(open, handle{n: i + 1, info: info, dev: dev})
	}
	fmt.Fprintf(out, "Holding %d of %d beacons open at once.\n", len(open), len(found))

	for _, h := range open {
		fmt.Fprintf(out, "Flashing beacon %d of %d for %v — %s\n", h.n, len(found), hold, describe(h.info))
		if _, err := h.dev.Write(magicFlash); err != nil {
			fmt.Fprintf(out, "  write FAILED: %v\n", err)
			continue
		}
		time.Sleep(hold)
		if _, err := h.dev.Write(magicStop); err != nil {
			fmt.Fprintf(out, "  stop FAILED: %v\n", err)
		}
	}

	for _, h := range open {
		h.dev.Write(magicStop)
		h.dev.Close()
	}
	fmt.Fprintln(out, "All beacons stopped.")
}

func describe(info hid.DeviceInfo) string {
	parts := []string{}
	if s := strings.TrimSpace(info.SerialNbr); s != "" {
		parts = append(parts, "serial "+s)
	}
	if key := platformPortKey(info); key != "" {
		parts = append(parts, "port key "+key)
	}
	if len(parts) == 0 {
		return info.Path
	}
	return strings.Join(parts, ", ")
}

func orEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(empty)"
	}
	return s
}
