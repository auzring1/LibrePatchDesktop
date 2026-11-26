//go:build !windows

// Package elevate makes the app run with administrator rights. Only meaningful on Windows.
package elevate

// IsAdmin is always true off Windows (nothing to elevate).
func IsAdmin() bool { return true }

// Relaunch does nothing off Windows.
func Relaunch() error { return nil }

// RunTask does nothing off Windows.
func RunTask(string) error { return nil }
