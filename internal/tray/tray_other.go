//go:build !windows

// Package tray puts an icon in the notification area. Only implemented on Windows.
package tray

// Start does nothing off Windows and reports false.
func Start(open, quit func()) bool { return false }
