// Package sysinfo describes this PC for the summary panel.
package sysinfo

import (
	"os"
	"path/filepath"
)

// Info is shown in the window.
type Info struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Cache    string `json:"cache"`
}

// Get collects the information.
func Get() Info {
	i := Info{OS: osName()}
	i.Hostname, _ = os.Hostname()
	if d, err := os.UserCacheDir(); err == nil {
		i.Cache = filepath.Join(d, "LibrePatchDesktop")
	}
	return i
}
