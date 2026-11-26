package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
)

func TestCheck(t *testing.T) {
	fs, err := catalog.LoadFile(writeCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	es := []catalog.Source{fs}
	apps := []inventory.App{{Name: "7-Zip 23.01", Version: "23.01"}, {Name: "VLC media player", Version: "3.0.21"}, {Name: "Other", Version: "1"}}
	got := Check(apps, es)
	want := []Status{Outdated, UpToDate, Unknown}
	for i, w := range want {
		if got[i].Status != w {
			t.Errorf("%s: %s want %s", apps[i].Name, got[i].Status, w)
		}
	}
}

func writeCatalog(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "c.json")
	h := strings.Repeat("0", 64)
	doc := `{"entries":[{"id":"7z","match":["7-zip"],"version":"24.9","url":"https://e.com/a.exe","sha256":"` + h + `"},
	{"id":"vlc","match":["vlc"],"version":"3.0.21","url":"https://e.com/b.exe","sha256":"` + h + `"}]}`
	if err := os.WriteFile(p, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func entryFor(url string, body []byte) catalog.Entry {
	s := sha256.Sum256(body)
	return catalog.Entry{ID: "x", Match: []string{"x"}, Version: "2", URL: url, SHA256: hex.EncodeToString(s[:]), SilentArgs: []string{"/S"}}
}

func TestInstallVerifiesHash(t *testing.T) {
	body := []byte("installer")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write(body) }))
	defer srv.Close()
	http.DefaultClient = srv.Client()

	ran := false
	run := func(string, []string) error { ran = true; return nil }
	if err := Install(context.Background(), &catalog.FileSource{}, entryFor(srv.URL+"/x.exe", body), run); err != nil || !ran {
		t.Fatalf("good hash: err=%v ran=%v", err, ran)
	}
	ran = false
	if err := Install(context.Background(), &catalog.FileSource{}, entryFor(srv.URL+"/x.exe", []byte("other")), run); err == nil || ran {
		t.Fatalf("bad hash must fail and not run: err=%v ran=%v", err, ran)
	}
}

func TestCommand(t *testing.T) {
	if n, a := Command("exe", "C:\\t\\i.exe", []string{"/S"}); n != "C:\\t\\i.exe" || a[0] != "/S" {
		t.Fatal(n, a)
	}
	if n, a := Command("msi", "C:\\t\\i.msi", []string{"/qn"}); !strings.HasSuffix(n, `\System32\msiexec.exe`) || a[0] != "/i" || a[1] != "C:\\t\\i.msi" || a[2] != "/qn" {
		t.Fatal(n, a)
	}
	n, a := Command("msix", "C:\\o'brien\\i.msix", nil)
	if !strings.HasSuffix(n, `\powershell.exe`) || !strings.Contains(a[len(a)-1], "'C:\\o''brien\\i.msix'") {
		t.Fatal(n, a)
	}
}

func TestSupersededOlderCopy(t *testing.T) {
	src := fixedSource{id: "MS.Runtime", ver: "8.0.34"}
	apps := []inventory.App{{Name: "Runtime 8.0.8", Version: "8.0.8.33916"}, {Name: "Runtime 8.0.34", Version: "8.0.34.36622"}}
	got := Check(apps, []catalog.Source{src})
	if got[0].Status != Superseded || got[1].Status != UpToDate {
		t.Fatalf("%s %s", got[0].Status, got[1].Status)
	}
	got = Check(apps[:1], []catalog.Source{src})
	if got[0].Status != Outdated {
		t.Fatalf("alone it is outdated, got %s", got[0].Status)
	}
}

type fixedSource struct{ id, ver string }

func (f fixedSource) Name() string { return "winget" }
func (f fixedSource) Lookup(inventory.App) (catalog.Entry, bool) {
	return catalog.Entry{Source: "winget", ID: f.id, Version: f.ver}, true
}
func (f fixedSource) Resolve(_ context.Context, e catalog.Entry) (catalog.Entry, error) {
	return e, nil
}
