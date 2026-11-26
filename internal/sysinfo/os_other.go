//go:build !windows

package sysinfo

import "runtime"

func osName() string { return runtime.GOOS + "/" + runtime.GOARCH }
