package main

import "testing"

func TestSameWindowsPath_isThisProgramWhateverTheSpelling(t *testing.T) {
	const exe = `C:\Tools\BlinkyBeacon\BB-DASH.exe`
	for _, stored := range []string{
		exe,
		`c:\tools\blinkybeacon\bb-dash.EXE`,        // Windows paths ignore case
		`C:/Tools/BlinkyBeacon/BB-DASH.exe`,        // either separator
		`C:\Tools\\BlinkyBeacon\.\BB-DASH.exe`,     // doubled separator, "."
		`C:\Tools\old\..\BlinkyBeacon\BB-DASH.exe`, // ".."
		`C:\..\Tools\BlinkyBeacon\BB-DASH.exe`,     // ".." stops at the root
		`"C:\Tools\BlinkyBeacon\BB-DASH.exe"`,      // quoted, as Run values often are
		`  C:\Tools\BlinkyBeacon\BB-DASH.exe `,     // stray whitespace
		`\\?\C:\Tools\BlinkyBeacon\BB-DASH.exe`,    // the long-path prefix
	} {
		if !sameWindowsPath(stored, exe) {
			t.Errorf("sameWindowsPath(%q, %q) = false, want true", stored, exe)
		}
	}
}

func TestSameWindowsPath_aStaleEntryIsNotThisProgram(t *testing.T) {
	// The case that matters: the operator swapped blinkybeacon-tray.exe for
	// BB-DASH.exe and deleted the old file. The Run value still names it, and
	// nothing starts at login — so the menu must not say it will.
	const exe = `C:\Tools\BlinkyBeacon\BB-DASH.exe`
	for _, stored := range []string{
		`C:\Tools\BlinkyBeacon\blinkybeacon-tray.exe`, // the old name, same folder
		`C:\Tools\Other\BB-DASH.exe`,                  // the same name, moved
		`D:\Tools\BlinkyBeacon\BB-DASH.exe`,           // another drive
		`C:\Tools\BlinkyBeacon\BB-DASH.exe.old`,       // a longer name, not a prefix match
		`C:Tools\BlinkyBeacon\BB-DASH.exe`,            // drive-relative is not rooted
		`"C:\Tools\BlinkyBeacon\BB-DASH.exe" --quiet`, // a command line, not this path
		``,
	} {
		if sameWindowsPath(stored, exe) {
			t.Errorf("sameWindowsPath(%q, %q) = true, want false", stored, exe)
		}
	}
}

func TestSameWindowsPath_networkShares(t *testing.T) {
	const exe = `\\server\share\BlinkyBeacon\BB-DASH.exe`
	if !sameWindowsPath(`\\SERVER\Share\blinkybeacon\bb-dash.exe`, exe) {
		t.Error("the same share spelled in another case reads as a different program")
	}
	if !sameWindowsPath(`\\?\UNC\server\share\BlinkyBeacon\BB-DASH.exe`, exe) {
		t.Error("the long-path spelling of the share reads as a different program")
	}
	// ".." stops at the share, as it stops at a drive's root.
	if sameWindowsPath(`\\server\share\..\other\BlinkyBeacon\BB-DASH.exe`, `\\server\other\BlinkyBeacon\BB-DASH.exe`) {
		t.Error(`".." climbed out of the share into another one`)
	}
}
