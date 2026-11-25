package catalog

import (
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
	for name, mut := range map[string]func(*Entry){
		"http":    func(e *Entry) { e.URL = "http://e.com/x.exe" },
		"msi":     func(e *Entry) { e.URL = "https://e.com/x.MSI?dl=1" },
		"nohash":  func(e *Entry) { e.SHA256 = "" },
		"nomatch": func(e *Entry) { e.Match = nil },
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
	es, err := File(p).Entries()
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := Find(es, "7-Zip 23.01 (x64)"); !ok || e.ID != "7zip" {
		t.Fatal("expected match")
	}
	if _, ok := Find(es, "Notepad++"); ok {
		t.Fatal("unexpected match")
	}
	os.WriteFile(p, []byte(`{"entries":[{"id":"a"}]}`), 0o600)
	if _, err := File(p).Entries(); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("want validation error, got %v", err)
	}
}
