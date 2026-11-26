package settings

import (
	"path/filepath"
	"testing"
)

func TestPersistAndDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	s := Open(p)
	if d := s.Get(); d.Theme != "system" || d.View != "grid" {
		t.Fatalf("defaults: %+v", d)
	}
	if err := s.SetLook("dark", "bogus"); err != nil {
		t.Fatal(err)
	}
	s.SetSkipped("Git.Git", true)
	s.SetSkipped("Git.Git", true)
	s.SetSkipped("Other", true)
	s.SetSkipped("Other", false)
	r := Open(p)
	d := r.Get()
	if d.Theme != "dark" || d.View != "grid" || len(d.Skipped) != 1 || !r.IsSkipped("Git.Git") || r.IsSkipped("Other") {
		t.Fatalf("reloaded: %+v", d)
	}
}
