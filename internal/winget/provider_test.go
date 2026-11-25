package winget

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPickInstallerPicksExe(t *testing.T) {
	in, err := PickInstaller(read(t, "7zip.7zip.installer.yaml"), "x64")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(in.URL, "7z2600-x64.exe") || len(in.SilentArgs) != 1 || in.SilentArgs[0] != "/S" || in.Type != "exe" {
		t.Fatalf("unexpected installer %+v", in)
	}

	in, err = PickInstaller(read(t, "7zip.7zip.installer.yaml"), "x86")
	if err != nil || !strings.HasSuffix(in.URL, "7z2600.exe") {
		t.Fatalf("x86: %+v %v", in, err)
	}
}

func TestPickInstallerNullsoftDefaultsAndTopLevelSwitch(t *testing.T) {
	in, err := PickInstaller(read(t, "VideoLAN.VLC.installer.yaml"), "x64")
	if err != nil || in.Type != "nullsoft" || !strings.Contains(in.URL, "win64") || in.SilentArgs[0] != "/S" {
		t.Fatalf("%+v %v", in, err)
	}
}

func TestPickInstallerMSIAndMSIX(t *testing.T) {
	h := strings.Repeat("A", 64)
	doc := "Installers:\n- Architecture: x64\n  InstallerType: wix\n  InstallerUrl: https://e.com/a.msi\n  InstallerSha256: " + h + "\n"
	in, err := PickInstaller([]byte(doc), "x64")
	if err != nil || in.Kind != "msi" || len(in.SilentArgs) != 2 {
		t.Fatalf("msi: %+v %v", in, err)
	}
	doc = strings.Replace(doc, "wix", "msix", 1)
	in, err = PickInstaller([]byte(doc), "x64")
	if err != nil || in.Kind != "msix" || len(in.SilentArgs) != 0 {
		t.Fatalf("msix: %+v %v", in, err)
	}
	// Zip/portable are skipped; a generic exe with no known silent switches must not be run unattended.
	for _, typ := range []string{"zip", "portable", "exe"} {
		d := strings.Replace(doc, "msix", typ, 1)
		if _, err := PickInstaller([]byte(d), "x64"); err != ErrNoInstaller {
			t.Fatalf("%s: got %v", typ, err)
		}
	}
}

func TestManifestURL(t *testing.T) {
	got := ManifestURL("https://h/", "Google.Chrome", "1.2")
	if got != "https://h/manifests/g/Google/Chrome/1.2/Google.Chrome.installer.yaml" {
		t.Fatal(got)
	}
}

func TestResolveFetchesManifest(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write(read(t, "VideoLAN.VLC.installer.yaml"))
	}))
	defer srv.Close()
	p := &Provider{Client: srv.Client(), ManifestBase: srv.URL, Arch: "x64"}
	e, err := p.Resolve(context.Background(), catalog.Entry{ID: "VideoLAN.VLC", Version: "3.0.21"})
	if err != nil || path != "/manifests/v/VideoLAN/VLC/3.0.21/VideoLAN.VLC.installer.yaml" {
		t.Fatalf("path=%q err=%v", path, err)
	}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
}
