package catalog

import (
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const h = "0000000000000000000000000000000000000000000000000000000000000000"

func TestValidate(t *testing.T) {
	ok := Entry{ID: "x", Version: "1", Match: []string{"x"}, URL: "https://e.com/x.exe", SHA256: h}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	msi := ok
	msi.Type, msi.URL = "msi", "https://e.com/x.msi"
	if err := msi.Validate(); err != nil {
		t.Fatalf("msi must be allowed: %v", err)
	}
	for name, mut := range map[string]func(*Entry){
		"http":   func(e *Entry) { e.URL = "http://e.com/x.exe" },
		"type":   func(e *Entry) { e.Type = "zip" },
		"nohash": func(e *Entry) { e.SHA256 = "" },
	} {
		e := ok
		mut(&e)
		if e.Validate() == nil {
			t.Errorf("%s should be rejected", name)
		}
	}
}

func TestFindAndFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(p, []byte(`{"entries":[{"id":"7zip","name":"7-Zip","match":["7-zip"],"version":"24.9","url":"https://e.com/7z.exe","sha256":"`+h+`","silentArgs":["/S"]}]}`), 0o600)
	fs, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := fs.Lookup(inventory.App{Name: "7-Zip 23.01 (x64)"}); !ok || e.ID != "7zip" {
		t.Fatal("expected match")
	}
	if _, ok := fs.Lookup(inventory.App{Name: "Notepad++"}); ok {
		t.Fatal("unexpected match")
	}
	os.WriteFile(p, []byte(`{"entries":[{"id":"a"}]}`), 0o600)
	if _, err := LoadFile(p); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("want validation error, got %v", err)
	}
}
