// Package settings keeps the user's choices (theme, view, skipped apps) in a small JSON file.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/schedule"
)

// Data is what is stored.
type Data struct {
	Theme   string   `json:"theme"`   // "system", "dark" or "light"
	View    string   `json:"view"`    // "grid", "list" or "compact"
	Skipped []string `json:"skipped"` // apps the user never wants updated (winget id, or name)

	Schedule         schedule.Schedule `json:"schedule"`
	AskedSchedule    bool              `json:"askedSchedule"`    // the first-run question was answered
	CloseToTray      bool              `json:"closeToTray"`      // closing the window keeps the app running in the tray
	StartWithWindows bool              `json:"startWithWindows"` // mirror of the startup task, for display
	LastRun          time.Time         `json:"lastRun"`          // when the last scheduled run started
	LastResult       string            `json:"lastResult"`       // what it did, in plain words
}

// Store is a Data persisted to a file. A nil path keeps everything in memory only.
type Store struct {
	mu   sync.Mutex
	path string
	d    Data
}

// Open loads path if it exists and fills in defaults.
func Open(path string) *Store {
	s := &Store{path: path, d: Data{Theme: "system", View: "grid", Schedule: schedule.Default(), CloseToTray: true}}
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			json.Unmarshal(b, &s.d)
		}
	}
	s.normalize()
	return s
}

func (s *Store) normalize() {
	switch s.d.Theme {
	case "system", "dark", "light":
	default:
		s.d.Theme = "system"
	}
	if s.d.Schedule.Valid() != nil {
		s.d.Schedule = schedule.Default()
	}
	switch s.d.View {
	case "grid", "list", "compact":
	default:
		s.d.View = "grid"
	}
}

// Get returns a copy.
func (s *Store) Get() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := s.d
	d.Skipped = append([]string(nil), s.d.Skipped...)
	return d
}

// Update changes any fields under the store's lock and saves.
func (s *Store) Update(f func(d *Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.d)
	s.normalize()
	return s.save()
}

// SetLook changes theme and view.
func (s *Store) SetLook(theme, view string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.d.Theme, s.d.View = theme, view
	s.normalize()
	return s.save()
}

// SetSkipped adds or removes an app from the skipped list.
func (s *Store) SetSkipped(key string, skip bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.d.Skipped[:0:0]
	for _, k := range s.d.Skipped {
		if k != key {
			out = append(out, k)
		}
	}
	if skip {
		out = append(out, key)
	}
	s.d.Skipped = out
	return s.save()
}

// IsSkipped reports whether key is skipped.
func (s *Store) IsSkipped(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.d.Skipped {
		if k == key {
			return true
		}
	}
	return false
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s.d, "", "  ")
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
