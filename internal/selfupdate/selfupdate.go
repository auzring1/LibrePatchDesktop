// Package selfupdate finds a newer LibrePatchDesktop release on GitHub and installs it.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/version"
)

const (
	// APIBase lists the releases (pre-releases included, unlike /releases/latest).
	APIBase     = "https://api.github.com/repos/auzring1/LibrePatchDesktop"
	assetPrefix = "https://github.com/auzring1/LibrePatchDesktop/releases/download/"
	setupName   = "LibrePatchDesktop-Setup.exe"
	sumsName    = "SHA256SUMS.txt"
	maxSetup    = 128 << 20
)

// Release is a published version.
type Release struct {
	Version  string `json:"version"`
	PageURL  string `json:"pageUrl"`
	SetupURL string `json:"-"`
	SumsURL  string `json:"-"`
}

type apiRelease struct {
	Tag    string `json:"tag_name"`
	Draft  bool   `json:"draft"`
	HTML   string `json:"html_url"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// Latest returns the newest release that has the installer and its checksum file.
func Latest(ctx context.Context, c *http.Client, apiBase string) (Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/releases?per_page=15", nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("release list: %s", resp.Status)
	}
	var list []apiRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&list); err != nil {
		return Release{}, err
	}
	var best Release
	for _, r := range list {
		if r.Draft {
			continue
		}
		rel := Release{Version: strings.TrimPrefix(r.Tag, "v"), PageURL: r.HTML}
		for _, a := range r.Assets {
			switch a.Name {
			case setupName:
				rel.SetupURL = a.URL
			case sumsName:
				rel.SumsURL = a.URL
			}
		}
		if rel.SetupURL == "" || rel.SumsURL == "" {
			continue
		}
		if best.Version == "" || version.Compare(rel.Version, best.Version) > 0 {
			best = rel
		}
	}
	if best.Version == "" {
		return Release{}, errors.New("no release with an installer found")
	}
	return best, nil
}

// Newer reports whether rel is newer than the running version. A "dev" build is never offered updates.
func Newer(rel Release, current string) bool {
	return current != "dev" && current != "" && version.Compare(rel.Version, current) > 0
}

// Install downloads the installer, checks it against the release's SHA256SUMS.txt and starts it with
// run (which should launch it and return without waiting; the installer closes this app itself).
func Install(ctx context.Context, c *http.Client, rel Release, run func(path string) error) error {
	for _, u := range []string{rel.SetupURL, rel.SumsURL} {
		if !strings.HasPrefix(u, assetPrefix) {
			return errors.New("refusing to download from an unexpected address")
		}
	}
	sums, err := fetch(ctx, c, rel.SumsURL, 1<<20)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(sums), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[1] == setupName {
			want = strings.ToLower(f[0])
		}
	}
	if len(want) != 64 {
		return errors.New("checksum for the installer not found")
	}
	dir, err := os.MkdirTemp("", "librepatchdesktop-update-")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, setupName)
	body, err := fetch(ctx, c, rel.SetupURL, maxSetup)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if hex.EncodeToString(sum[:]) != want {
		os.RemoveAll(dir)
		return errors.New("downloaded installer does not match its checksum; not running it")
	}
	if err := os.WriteFile(path, body, 0o700); err != nil {
		return err
	}
	return run(path)
}

func fetch(ctx context.Context, c *http.Client, url string, limit int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("download is larger than expected")
	}
	return b, nil
}

// RunSetup starts the installer with a progress window and no questions, without waiting for it.
func RunSetup(path string) error {
	return exec.Command(path, "/SILENT", "/NORESTART", "/CLOSEAPPLICATIONS").Start()
}
