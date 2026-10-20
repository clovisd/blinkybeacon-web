//go:build windows

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	hid "github.com/sstallion/go-hid"
)

const (
	fs22vid uint16 = 0x340d
	fs22pid uint16 = 0x1710
)

// Known sequences captured from USB traffic analysis.
var knownSequences = map[string][]byte{
	"stop":  {0x00, 0xFF, 0x00, 0x00, 0x64, 0x00, 0x32, 0x9E, 0xD7, 0x0D},
	"spin":  {0x00, 0xFF, 0x01, 0x66, 0xC8, 0xFF, 0xAD, 0x52, 0x81, 0xD6},
	"flash": {0x00, 0xFF, 0x07, 0xFF, 0x50, 0xFF, 0x1C, 0x8E, 0xB0, 0xB8},
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	if err := hid.Init(); err != nil {
		log.Fatalf("HID init failed: %v", err)
	}
	defer hid.Exit()

	switch os.Args[1] {
	case "list":
		cmdList()
	case "send":
		if len(os.Args) < 12 {
			fmt.Println("send requires exactly 10 hex bytes, e.g.:")
			fmt.Println("  send 00 FF 07 FF 50 FF 1C 8E B0 B8")
			os.Exit(1)
		}
		cmdSend(os.Args[2:12])
	case "replay":
		if len(os.Args) < 3 {
			fmt.Println("Usage: replay <stop|spin|flash>")
			os.Exit(1)
		}
		cmdReplay(os.Args[2])
	case "sweep":
		if len(os.Args) < 5 {
			fmt.Println("Usage: sweep <byte_index> <start_hex> <end_hex> [hold_ms]")
			fmt.Println("  sweep 2 00 0F 500")
			os.Exit(1)
		}
		hold := 800
		if len(os.Args) >= 6 {
			hold, _ = strconv.Atoi(os.Args[5])
		}
		cmdSweep(os.Args[2], os.Args[3], os.Args[4], hold)
	case "dump":
		cmdDump()
	case "interactive":
		cmdInteractive()
	default:
		fmt.Printf("Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`BlinkyBeacon Protocol Fuzzer — internal testing tool

Commands:
  list
      Confirm beacon is connected, show device info.

  send <10 hex bytes>
      Send a raw HID report. Use 0x prefix or bare hex.
      e.g.  send 00 FF 07 FF 50 FF 1C 8E B0 B8

  replay <stop|spin|flash>
      Send one of the three known sequences.

  sweep <byte> <start> <end> [hold_ms]
      Step through values for one byte position, holding each for
      hold_ms (default 800ms) so you can observe the change.
      Uses the 'spin' packet as the base.
      e.g.  sweep 2 00 0F
            sweep 3 00 FF 1000

  dump
      Print the three known packets annotated byte-by-byte.

  interactive
      REPL — send packets interactively, replay sequences, quit.

`)
}

// ---------- commands ----------

func cmdList() {
	dev, err := openBeacon()
	if err != nil {
		fmt.Println("No beacon found:", err)
		return
	}
	defer dev.Close()

	mfg, _ := dev.GetMfrStr()
	product, _ := dev.GetProductStr()
	serial, _ := dev.GetSerialNbr()

	fmt.Println("Beacon found:")
	fmt.Printf("  Manufacturer : %s\n", mfg)
	fmt.Printf("  Product      : %s\n", product)
	fmt.Printf("  Serial       : %s\n", serial)
	fmt.Printf("  VID:PID      : %04X:%04X\n", fs22vid, fs22pid)
}

func cmdSend(args []string) {
	payload := parseHexBytes(args)
	dev, err := openBeacon()
	if err != nil {
		log.Fatal("Beacon not found:", err)
	}
	defer dev.Close()

	n, err := dev.Write(payload)
	if err != nil {
		fmt.Printf("FAIL  % 02X  (%v)\n", payload, err)
		os.Exit(1)
	}
	fmt.Printf("SENT %d bytes  % 02X\n", n, payload)
}

func cmdReplay(name string) {
	payload, ok := knownSequences[name]
	if !ok {
		log.Fatalf("Unknown sequence %q — use stop, spin, or flash", name)
	}
	dev, err := openBeacon()
	if err != nil {
		log.Fatal("Beacon not found:", err)
	}
	defer dev.Close()

	if _, err := dev.Write(payload); err != nil {
		log.Fatalf("Write failed: %v", err)
	}
	fmt.Printf("Replayed %-5s  % 02X\n", name, payload)
}

func cmdSweep(byteStr, startStr, endStr string, holdMs int) {
	byteIdx, err := strconv.Atoi(byteStr)
	if err != nil || byteIdx < 0 || byteIdx > 9 {
		log.Fatalf("byte index must be 0-9")
	}
	startVal := parseHexByte(startStr)
	endVal := parseHexByte(endStr)
	if endVal < startVal {
		log.Fatal("end must be >= start")
	}

	dev, err := openBeacon()
	if err != nil {
		log.Fatal("Beacon not found:", err)
	}
	defer dev.Close()

	base := make([]byte, 10)
	copy(base, knownSequences["spin"])

	fmt.Printf("Sweeping byte[%d]: 0x%02X → 0x%02X  (%.1fs per step)\n",
		byteIdx, startVal, endVal, float64(holdMs)/1000)
	fmt.Println("Press Ctrl+C to stop early.")
	fmt.Println()

	for v := startVal; v <= endVal; v++ {
		payload := make([]byte, 10)
		copy(payload, base)
		payload[byteIdx] = v

		_, writeErr := dev.Write(payload)
		status := "OK  "
		if writeErr != nil {
			status = "FAIL"
		}
		fmt.Printf("  byte[%d]=0x%02X  %s  % 02X\n", byteIdx, v, status, payload)

		if v < endVal {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		}
	}

	fmt.Println("\nDone. Sending stop...")
	dev.Write(knownSequences["stop"])
}

func cmdDump() {
	names := []string{"stop", "spin", "flash"}
	labels := []string{
		"[0] report_id  [1] ?const_ff  [2] mode  [3] ?speed  [4] ?brightness  [5-9] unknown",
	}
	fmt.Println("Known sequences (annotated):")
	fmt.Printf("%-6s  bytes\n", "name")
	fmt.Printf("%-6s  %s\n", "------", strings.Repeat("-", 29))
	for _, n := range names {
		fmt.Printf("%-6s  % 02X\n", n, knownSequences[n])
	}
	fmt.Println()
	fmt.Println("Byte map (hypothesis — requires testing to confirm):")
	for _, l := range labels {
		fmt.Println(" ", l)
	}
	fmt.Println()
	fmt.Println("Byte-by-byte comparison:")
	fmt.Printf("  %-6s", "")
	for i := 0; i < 10; i++ {
		fmt.Printf("  [%d] ", i)
	}
	fmt.Println()
	for _, n := range names {
		fmt.Printf("  %-6s", n)
		for _, b := range knownSequences[n] {
			fmt.Printf(" 0x%02X", b)
		}
		fmt.Println()
	}
}

func cmdInteractive() {
	dev, err := openBeacon()
	if err != nil {
		log.Fatal("Beacon not found:", err)
	}
	defer dev.Close()

	fmt.Println("Interactive mode. Commands: send, replay, dump, quit")
	fmt.Println("  send 00 FF 07 FF 50 FF 1C 8E B0 B8")
	fmt.Println("  replay stop|spin|flash")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		parts := strings.Fields(scanner.Text())
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "send":
			if len(parts) < 11 {
				fmt.Println("Need 10 hex bytes")
				continue
			}
			payload := parseHexBytes(parts[1:11])
			if _, err := dev.Write(payload); err != nil {
				fmt.Printf("FAIL: %v\n", err)
			} else {
				fmt.Printf("SENT  % 02X\n", payload)
			}
		case "replay":
			if len(parts) < 2 {
				fmt.Println("Usage: replay <stop|spin|flash>")
				continue
			}
			if seq, ok := knownSequences[parts[1]]; ok {
				dev.Write(seq)
				fmt.Printf("Replayed %s\n", parts[1])
			} else {
				fmt.Println("Unknown sequence")
			}
		case "dump":
			cmdDump()
		case "quit", "exit", "q":
			dev.Write(knownSequences["stop"])
			fmt.Println("Stopped beacon. Bye.")
			return
		default:
			fmt.Println("Unknown command")
		}
	}
}

// ---------- helpers ----------

func openBeacon() (*hid.Device, error) {
	return hid.OpenFirst(fs22vid, fs22pid)
}

func parseHexByte(s string) byte {
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		log.Fatalf("Invalid hex byte: %q", s)
	}
	return byte(v)
}

func parseHexBytes(args []string) []byte {
	out := make([]byte, len(args))
	for i, a := range args {
		out[i] = parseHexByte(a)
	}
	return out
}
