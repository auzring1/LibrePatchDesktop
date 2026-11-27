// Package sysdir gives absolute paths of Windows system programs, so they are never found through PATH.
package sysdir

import (
	"os"
	"strings"
)

// System32 returns %SystemRoot%\System32 joined with parts, for example System32("schtasks.exe").
func System32(parts ...string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return strings.TrimRight(root, `\`) + `\System32\` + strings.Join(parts, `\`)
}

// Windows returns %SystemRoot% joined with parts, for programs that live outside System32 (explorer.exe).
func Windows(parts ...string) string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return strings.TrimRight(root, `\`) + `\` + strings.Join(parts, `\`)
}
