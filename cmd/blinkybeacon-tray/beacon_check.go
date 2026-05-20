//go:build windows

package main

import "github.com/duckfullstop/blinkybeacon/pkg/fsbeacon"

// Compile-time assertion: FarmBeacon must satisfy the local Beacon interface.
// If fsbeacon.Beacon gains new methods, this will fail and alert that the local
// Beacon interface in state.go needs updating.
var _ Beacon = (*fsbeacon.FarmBeacon)(nil)
