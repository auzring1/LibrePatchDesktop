package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
)

func TestCheck(t *testing.T) {
	es := []catalog.Entry{{ID: "7z", Match: []string{"7-zip"}, Version: "24.9"}, {ID: "vlc", Match: []string{"vlc"}, Version: "3.0.21"}}
	apps := []inventory.App{{Name: "7-Zip 23.01", Version: "23.01"}, {Name: "VLC media player", Version: "3.0.21"}, {Name: "Other", Version: "1"}}
	got := Check(apps, es)
	want := []Status{Outdated, UpToDate, Unknown}
	for i, w := range want {
		if got[i].Status != w {
			t.Errorf("%s: %s want %s", apps[i].Name, got[i].Status, w)
		}
	}
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
	if err := Install(context.Background(), entryFor(srv.URL+"/x.exe", body), run); err != nil || !ran {
		t.Fatalf("good hash: err=%v ran=%v", err, ran)
	}
	ran = false
	if err := Install(context.Background(), entryFor(srv.URL+"/x.exe", []byte("other")), run); err == nil || ran {
		t.Fatalf("bad hash must fail and not run: err=%v ran=%v", err, ran)
	}
}
