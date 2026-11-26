//go:build windows

package sysinfo

import (
	"runtime"

	"golang.org/x/sys/windows/registry"
)

func osName() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows"
	}
	defer k.Close()
	name, _, _ := k.GetStringValue("ProductName")
	ver, _, _ := k.GetStringValue("DisplayVersion")
	build, _, _ := k.GetStringValue("CurrentBuild")
	// ProductName still says "Windows 10" on Windows 11; builds from 22000 are Windows 11.
	if n := atoi(build); n >= 22000 && len(name) >= 10 && name[:10] == "Windows 10" {
		name = "Windows 11" + name[10:]
	}
	s := name
	if ver != "" {
		s += " " + ver
	}
	if runtime.GOARCH == "amd64" {
		s += ", 64-bit"
	}
	return s
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
