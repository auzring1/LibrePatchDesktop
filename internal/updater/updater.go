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

// Item pairs an installed app with its catalog entry.
type Item struct {
	App    inventory.App
	Entry  *catalog.Entry
	Status Status
}

// Check classifies every installed app.
func Check(apps []inventory.App, entries []catalog.Entry) []Item {
	items := make([]Item, 0, len(apps))
	for _, a := range apps {
		it := Item{App: a, Status: Unknown}
		if e, ok := catalog.Find(entries, a.Name); ok {
			e := e
			it.Entry = &e
			if version.Compare(a.Version, e.Version) < 0 {
				it.Status = Outdated
			} else {
				it.Status = UpToDate
			}
		}
		items = append(items, it)
	}
	return items
}

// Install downloads the installer, verifies its SHA-256 and runs it silently.
// Nothing is executed unless the hash matches the catalog.
func Install(ctx context.Context, e catalog.Entry, run func(path string, args []string) error) error {
	if err := e.Validate(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "librepatchdesktop-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "installer.exe")
	if err := download(ctx, e.URL, path, e.SHA256); err != nil {
		return err
	}
	if run == nil {
		run = func(p string, a []string) error { return exec.CommandContext(ctx, p, a...).Run() }
	}
	return run(path, e.SilentArgs)
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
