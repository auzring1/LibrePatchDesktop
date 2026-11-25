// Package updater works out which applications are outdated and installs updates.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/version"
)

// Status of one installed application against the catalog.
type Status string

const (
	UpToDate Status = "up-to-date"
	Outdated Status = "update-available"
	Unknown  Status = "not-in-catalog"
)

// Item pairs an installed app with the catalog entry that describes its newest version.
type Item struct {
	App    inventory.App
	Entry  *catalog.Entry
	Source catalog.Source
	Status Status
}

// Check classifies every installed app. The first source that recognises an app decides.
func Check(apps []inventory.App, sources []catalog.Source) []Item {
	items := make([]Item, 0, len(apps))
	for _, a := range apps {
		it := Item{App: a, Status: Unknown}
		for _, s := range sources {
			if e, ok := s.Lookup(a); ok {
				e := e
				it.Entry, it.Source = &e, s
				it.Status = UpToDate
				if version.Compare(a.Version, e.Version) < 0 {
					it.Status = Outdated
				}
				break
			}
		}
		items = append(items, it)
	}
	return items
}

// Install resolves the download details, downloads the installer, verifies its SHA-256 and runs it
// silently. Nothing is executed unless the hash matches the one the source gave.
func Install(ctx context.Context, src catalog.Source, e catalog.Entry, run func(name string, argv []string) error) error {
	e, err := src.Resolve(ctx, e)
	if err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "librepatchdesktop-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "installer."+map[string]string{"exe": "exe", "msi": "msi", "msix": "msix"}[e.Kind()])
	if err := download(ctx, e.URL, path, e.SHA256); err != nil {
		return err
	}
	name, argv := Command(e.Kind(), path, e.SilentArgs)
	if run == nil {
		run = func(n string, a []string) error { return exec.CommandContext(ctx, n, a...).Run() }
	}
	return run(name, argv)
}

// Command builds the program and arguments that install a downloaded package silently.
// No shell is involved; the path is a file this program just created in its own temp directory.
func Command(kind, path string, silent []string) (string, []string) {
	switch kind {
	case "msi":
		return "msiexec.exe", append([]string{"/i", path}, silent...)
	case "msix":
		q := strings.ReplaceAll(path, "'", "''")
		return "powershell.exe", []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command",
			"Add-AppxPackage -Path '" + q + "' -ErrorAction Stop"}
	}
	return path, silent
}

func download(ctx context.Context, url, dest, wantHex string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, wantHex) {
		return errors.New("installer hash does not match the catalog; not running it")
	}
	return nil
}
