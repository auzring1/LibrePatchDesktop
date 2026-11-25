// Package catalog describes which applications can be updated and how.
//
// Sources sit behind the Source interface so more can be added.
package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"os"
	"strings"
)

// Entry is the newest known release of one application.
type Entry struct {
	Source     string   `json:"-"` // which Source produced it ("file", "winget", ...)
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Publisher  string   `json:"publisher,omitempty"`
	Match      []string `json:"match"`          // lower-case substrings of the installed DisplayName
	Version    string   `json:"version"`        // latest version
	Type       string   `json:"type,omitempty"` // exe (default), msi or msix
	URL        string   `json:"url"`            // https installer download
	SHA256     string   `json:"sha256"`         // hex, required
	SilentArgs []string `json:"silentArgs"`     // e.g. ["/S"] or ["/VERYSILENT","/NORESTART"]
}

// Source is one place to learn about application updates. Several can be combined: the first that
// recognises an installed app decides. Today: winget and a JSON file; a curated catalog or automatic
// discovery on vendor sites can be added later as further Sources.
type Source interface {
	Name() string
	// Lookup returns the newest known version for an installed app. The entry may still lack its
	// download details (URL, SHA256, SilentArgs); Resolve fills them in just before installing.
	Lookup(app inventory.App) (Entry, bool)
	Resolve(ctx context.Context, e Entry) (Entry, error)
}

// FileSource is a Source backed by a JSON file: {"entries":[...]}.
type FileSource struct{ entries []Entry }

func (f *FileSource) Name() string { return "file" }

// LoadFile reads and validates a catalog file.
func LoadFile(path string) (*FileSource, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	for i := range doc.Entries {
		doc.Entries[i].Source = "file"
		if len(doc.Entries[i].Match) == 0 {
			return nil, fmt.Errorf("catalog entry %d (%s): match is required", i, doc.Entries[i].ID)
		}
		if err := doc.Entries[i].Validate(); err != nil {
			return nil, fmt.Errorf("catalog entry %d (%s): %w", i, doc.Entries[i].ID, err)
		}
	}
	return &FileSource{entries: doc.Entries}, nil
}

func (f *FileSource) Lookup(app inventory.App) (Entry, bool) { return Find(f.entries, app.Name) }

func (f *FileSource) Resolve(_ context.Context, e Entry) (Entry, error) { return e, nil }

// Validate enforces the safety rules every entry must meet.
func (e Entry) Validate() error {
	switch {
	case e.ID == "" || e.Version == "":
		return errors.New("id and version are required")
	case !strings.HasPrefix(strings.ToLower(e.URL), "https://"):
		return errors.New("url must be https")
	case len(e.SHA256) != 64:
		return errors.New("sha256 must be 64 hex characters")
	case e.Kind() == "":
		return errors.New("type must be exe, msi or msix")
	}
	return nil
}

// Kind is the installer kind: "exe", "msi" or "msix" ("" if unknown). An empty Type means exe.
func (e Entry) Kind() string {
	switch t := strings.ToLower(e.Type); t {
	case "", "exe":
		return "exe"
	case "msi", "msix":
		return t
	}
	return ""
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
