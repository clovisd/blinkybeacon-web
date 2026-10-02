package main

import (
	"path"
	"strings"
)

// sameWindowsPath reports whether the program a "Start with Windows" entry
// names is exe, as far as the two spellings can tell. Windows paths ignore
// case and take either separator, so both are cleaned before comparing:
// quotes and stray whitespace around the value dropped, the long-path prefix
// removed, doubled separators collapsed, "." and ".." resolved.
//
// Plain string work rather than path/filepath, whose rules are the host's: it
// has to give Windows answers in the tests that run on Linux and macOS too.
func sameWindowsPath(stored, exe string) bool {
	a, b := cleanWindowsPath(stored), cleanWindowsPath(exe)
	return a != "" && strings.EqualFold(a, b)
}

// cleanWindowsPath reduces a Windows path to one spelling: the volume (a
// drive such as C:, or a share such as \\server\share) followed by the
// cleaned remainder, with backslashes throughout.
func cleanWindowsPath(p string) string {
	p = strings.TrimSpace(p)
	if len(p) >= 2 && p[0] == '"' && p[len(p)-1] == '"' {
		p = p[1 : len(p)-1]
	}
	switch {
	case strings.HasPrefix(p, `\\?\UNC\`):
		p = `\\` + p[len(`\\?\UNC\`):]
	case strings.HasPrefix(p, `\\?\`):
		p = p[len(`\\?\`):]
	}
	p = strings.ReplaceAll(p, "/", `\`)

	vol := windowsVolume(p)
	rest := p[len(vol):]
	if rest == "" {
		return vol
	}
	// path.Clean is the same on every host, and works in forward slashes. A
	// rooted remainder cannot climb above its root, so ".." stops at the
	// drive or the share, as it does on Windows.
	cleaned := path.Clean(strings.ReplaceAll(rest, `\`, "/"))
	return vol + strings.ReplaceAll(cleaned, "/", `\`)
}

// windowsVolume is the drive letter and colon, or the \\server\share, that a
// path starts with — "" for a path with neither.
func windowsVolume(p string) string {
	if len(p) >= 2 && p[1] == ':' {
		return p[:2]
	}
	if !strings.HasPrefix(p, `\\`) {
		return ""
	}
	server := strings.IndexByte(p[2:], '\\')
	if server < 0 {
		return p
	}
	shareStart := 2 + server + 1
	share := strings.IndexByte(p[shareStart:], '\\')
	if share < 0 {
		return p
	}
	return p[:shareStart+share]
}
