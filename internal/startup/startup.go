// Package startup lists the programs that start with Windows and switches them on or off, the same way Task
// Manager's Startup tab does (the StartupApproved registry keys). Nothing is deleted.
package startup

import (
	"encoding/binary"
	"time"
)

// Entry is one program that starts when the user signs in.
type Entry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	Location string `json:"location"` // for example "Current user (registry)"
	Enabled  bool   `json:"enabled"`
}

// isEnabled reads a StartupApproved value: an even first byte means enabled, an odd one disabled, none enabled.
func isEnabled(v []byte) bool { return len(v) == 0 || v[0]&1 == 0 }

// approvedValue builds the 12 byte StartupApproved value.
func approvedValue(on bool, now time.Time) []byte {
	v := make([]byte, 12)
	if on {
		v[0] = 2
		return v
	}
	v[0] = 3
	// FILETIME: 100ns ticks since 1601.
	ticks := uint64(now.UnixNano()/100) + 116444736000000000
	binary.LittleEndian.PutUint64(v[4:], ticks)
	return v
}
