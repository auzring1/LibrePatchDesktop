//go:build !windows

// Package window shows a page in a native window. Only implemented on Windows.
package window

import "errors"

// Start is not available off Windows.
func Start(url, title, theme string, hidden bool, keep func() bool) error {
	return errors.New("native window is only available on Windows")
}

// Show does nothing off Windows.
func Show() {}

// SetTheme does nothing off Windows.
func SetTheme(string) {}
