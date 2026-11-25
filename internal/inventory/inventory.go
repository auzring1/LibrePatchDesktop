// Package inventory detects installed Windows applications.
package inventory

import "sort"

// App is one installed application as shown in Windows "Installed apps".
type App struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Publisher string `json:"publisher,omitempty"`
	Scope     string `json:"scope"` // "machine" or "user"
	// UninstallKey is the registry key name, useful to tell duplicates apart.
	UninstallKey string `json:"uninstallKey,omitempty"`
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
