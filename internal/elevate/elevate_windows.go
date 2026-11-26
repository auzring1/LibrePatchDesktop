//go:build windows

// Package elevate makes the app run with administrator rights, which most installers need.
package elevate

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsAdmin reports whether the process is elevated.
func IsAdmin() bool { return windows.GetCurrentProcessToken().IsElevated() }

// Relaunch starts this program again with the same arguments and a UAC prompt. It returns nil once the new
// process has been started (the caller should then exit), or an error if the prompt was declined.
func Relaunch() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := windows.ComposeCommandLine(os.Args[1:])
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	params, _ := syscall.UTF16PtrFromString(strings.TrimSpace(args))
	return windows.ShellExecute(0, verb, file, params, nil, windows.SW_NORMAL)
}
