// Package catalog describes which applications can be updated and how.
//
// The catalog source is still open (winget community index, own curated list,
// or both); everything here sits behind the Source interface so it can change.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Entry is the newest known release of one application.
type Entry struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Publisher  string   `json:"publisher,omitempty"`
	Match      []string `json:"match"`      // lower-case substrings of the installed DisplayName
	Version    string   `json:"version"`    // latest version
	URL        string   `json:"url"`        // https installer download
	SHA256     string   `json:"sha256"`     // hex, required
	SilentArgs []string `json:"silentArgs"` // e.g. ["/S"] or ["/VERYSILENT","/NORESTART"]
}

// Source provides catalog entries.
type Source interface {
	Entries() ([]Entry, error)
}

// File is a Source backed by a JSON file: {"entries":[...]}.
type File string

func (f File) Entries() ([]Entry, error) {
	b, err := os.ReadFile(string(f))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", f, err)
	}
	for i, e := range doc.Entries {
		if err := e.Validate(); err != nil {
			return nil, fmt.Errorf("catalog entry %d (%s): %w", i, e.ID, err)
		}
	}
	return doc.Entries, nil
}

// Validate enforces the safety rules every entry must meet.
func (e Entry) Validate() error {
	switch {
	case e.ID == "" || e.Version == "" || len(e.Match) == 0:
		return errors.New("id, version and match are required")
	case !strings.HasPrefix(strings.ToLower(e.URL), "https://"):
		return errors.New("url must be https")
	case len(e.SHA256) != 64:
		return errors.New("sha256 must be 64 hex characters")
	case strings.HasSuffix(strings.ToLower(strings.SplitN(e.URL, "?", 2)[0]), ".msi"):
		return errors.New("MSI installers are not supported (home PCs only)")
	}
	return nil
}

// Find returns the entry matching an installed application name, if any.
func Find(entries []Entry, installedName string) (Entry, bool) {
	n := strings.ToLower(installedName)
	for _, e := range entries {
		for _, m := range e.Match {
			if m != "" && strings.Contains(n, strings.ToLower(m)) {
				return e, true
			}
		}
	}
	return Entry{}, false
}
