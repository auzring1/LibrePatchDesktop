package winget

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
)

const (
	// IndexURL is the index file the winget client itself uses.
	IndexURL = "https://cdn.winget.microsoft.com/cache/source2.msix"
	// ManifestBase serves the community manifests (installer URLs, hashes, silent switches).
	ManifestBase = "https://raw.githubusercontent.com/microsoft/winget-pkgs/master"

	maxIndexDownload = 64 << 20
	indexMaxAge      = 24 * time.Hour
)

// Provider is a catalog.Source backed by the winget community index.
type Provider struct {
	Matcher      *Matcher
	Client       *http.Client
	ManifestBase string
	Arch         string // "x64", "x86" or "arm64"
}

func (p *Provider) Name() string { return "winget" }

// Lookup recognises an installed app (product code, or name plus publisher) and returns the newest version.
func (p *Provider) Lookup(app inventory.App) (catalog.Entry, bool) {
	m, ok := p.Matcher.Find(Installed{Name: app.Name, Publisher: app.Publisher, ItemKey: app.UninstallKey})
	if !ok {
		return catalog.Entry{}, false
	}
	return catalog.Entry{Source: "winget", ID: m.ID, Name: m.Name, Version: m.LatestVersion}, true
}

// Resolve reads the package's manifest to find the installer, its SHA-256 and silent switches.
func (p *Provider) Resolve(ctx context.Context, e catalog.Entry) (catalog.Entry, error) {
	doc, err := FetchManifest(ctx, p.Client, ManifestURL(p.ManifestBase, e.ID, e.Version))
	if err != nil {
		return e, err
	}
	in, err := PickInstaller(doc, p.Arch)
	if err != nil {
		return e, fmt.Errorf("%s %s: %w", e.ID, e.Version, err)
	}
	e.Type, e.URL, e.SHA256, e.SilentArgs = in.Kind, in.URL, in.SHA256, in.SilentArgs
	return e, nil
}

// LoadIndex returns the index, using cacheDir to avoid downloading it more than once a day.
// If the download fails but an older copy exists, the older copy is used.
func LoadIndex(ctx context.Context, client *http.Client, cacheDir string) (*Snapshot, error) {
	file := filepath.Join(cacheDir, "source2.msix")
	if st, err := os.Stat(file); err != nil || time.Since(st.ModTime()) > indexMaxAge {
		if derr := downloadIndex(ctx, client, file); derr != nil && err != nil {
			return nil, derr
		}
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	return ParseMSIX(b)
}

func downloadIndex(ctx context.Context, client *http.Client, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, IndexURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("winget index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("winget index: %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return err
	}
	tmp := dest + ".part"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxIndexDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxIndexDownload {
		err = fmt.Errorf("winget index: download larger than %d MiB", maxIndexDownload>>20)
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}
