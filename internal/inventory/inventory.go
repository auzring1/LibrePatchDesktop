// Package inventory detects installed Windows applications.
package inventory

import (
	"regexp"
	"sort"
	"strings"
)

// App is one installed application as shown in Windows "Installed apps".
type App struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Publisher string `json:"publisher,omitempty"`
	Scope     string `json:"scope"` // "machine" or "user"
	// UninstallKey is the registry key name, useful to tell duplicates apart.
	UninstallKey string `json:"uninstallKey,omitempty"`
	// UninstallString and QuietUninstallString are the registry's commands to remove the program.
	UninstallString      string `json:"-"`
	QuietUninstallString string `json:"-"`
	WindowsInstaller     bool   `json:"-"` // registered by an MSI package
}

// Scanner lists installed applications.
type Scanner interface {
	Scan() ([]App, error)
}

// Static is a Scanner over a fixed list (tests and --fake demos).
type Static []App

func (s Static) Scan() ([]App, error) { return Clean(s), nil }

// Clean drops entries without name or version, removes exact duplicates
// (the same app listed in both registry views) and sorts by name.
func Clean(in []App) []App {
	seen := map[string]bool{}
	var out []App
	for _, a := range in {
		if a.Name == "" || a.Version == "" {
			continue
		}
		k := a.Name + "\x00" + a.Version + "\x00" + a.Scope
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

var (
	trailingVersion = regexp.MustCompile(`\s+v?\d+(\.\d+){1,}[\w.\-]*$`)
	archSuffix      = regexp.MustCompile(`(?i)\s*[\(\[]\s*(x64|x86|64-bit|32-bit|amd64|arm64)(\s+(x64|x86))?\s*[\)\]]`)
	x86Tag          = regexp.MustCompile(`(?i)\b(x86|32-bit)\b`)
	archWord        = regexp.MustCompile(`(?i)[\s\-]+(x64|x86|amd64)$`)
)

// FriendlyName turns an installed program's name into something short for display: it drops the version
// number and architecture tags, for example "Microsoft Visual C++ 2008 Redistributable - x86 9.0.30729.6161"
// becomes "Microsoft Visual C++ 2008 Redistributable" and "Notepad++ (64-bit x64)" becomes "Notepad++".
// A tag that tells two entries apart ("x86" vs "x64" runtimes) is kept as a trailing "(x86)".
func FriendlyName(name, version string) string {
	n := strings.TrimSpace(name)
	arch := ""
	if x86Tag.MatchString(n) {
		arch = "x86"
	}
	n = archSuffix.ReplaceAllString(n, "")
	n = strings.TrimSpace(strings.TrimSuffix(n, version))
	n = trailingVersion.ReplaceAllString(n, "")
	n = archWord.ReplaceAllString(n, "")
	n = archSuffix.ReplaceAllString(n, "")
	n = strings.TrimRight(strings.TrimSpace(n), " -")
	if n == "" {
		return name
	}
	if arch == "x86" {
		n += " (x86)"
	}
	return n
}
