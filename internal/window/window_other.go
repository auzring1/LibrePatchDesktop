//go:build !windows

// Package window shows a page in a native window. Only implemented on Windows.
package window

import "errors"

// Run is not available off Windows.
func Run(url, title string) error { return errors.New("native window is only available on Windows") }
