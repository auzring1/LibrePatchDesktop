package winget

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// Installer is the one installer chosen for this PC from a winget manifest.
type Installer struct {
	URL        string
	SHA256     string
	Type       string
	SilentArgs []string
}

type manifest struct {
	InstallerType     string `yaml:"InstallerType"`
	InstallerSwitches struct {
		Silent             string `yaml:"Silent"`
		SilentWithProgress string `yaml:"SilentWithProgress"`
	} `yaml:"InstallerSwitches"`
	Installers []struct {
		Architecture      string `yaml:"Architecture"`
		InstallerType     string `yaml:"InstallerType"`
		InstallerURL      string `yaml:"InstallerUrl"`
		InstallerSha256   string `yaml:"InstallerSha256"`
		InstallerSwitches struct {
			Silent             string `yaml:"Silent"`
			SilentWithProgress string `yaml:"SilentWithProgress"`
		} `yaml:"InstallerSwitches"`
	} `yaml:"Installers"`
}

// defaultSilent are winget's own default switches for installer frameworks whose switches are known.
var defaultSilent = map[string][]string{
	"inno":     {"/SP-", "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"},
	"nullsoft": {"/S"},
	"nsis":     {"/S"},
	"burn":     {"/quiet", "/norestart"},
}

// ErrNoInstaller means the manifest has no installer this app can run (for example only MSI or MSIX).
var ErrNoInstaller = errors.New("no usable installer in the winget manifest")

// PickInstaller chooses an installer for arch ("x64", "x86", "arm64") from a winget installer manifest.
// Only plain executable installers are used: MSI (wix/msi), MSIX/AppX, zip and portable entries are skipped,
// because this product targets home PCs and never installs MSI packages.
func PickInstaller(yamlDoc []byte, arch string) (Installer, error) {
	var m manifest
	if err := yaml.Unmarshal(yamlDoc, &m); err != nil {
		return Installer{}, fmt.Errorf("winget manifest: %w", err)
	}
	prefs := []string{arch}
	switch arch {
	case "x64":
		prefs = append(prefs, "x86", "neutral")
	case "arm64":
		prefs = append(prefs, "x64", "x86", "neutral")
	default:
		prefs = append(prefs, "neutral")
	}
	for _, want := range prefs {
		for _, in := range m.Installers {
			typ := strings.ToLower(in.InstallerType)
			if typ == "" {
				typ = strings.ToLower(m.InstallerType)
			}
			if !strings.EqualFold(in.Architecture, want) {
				continue
			}
			switch typ {
			case "exe", "inno", "nullsoft", "nsis", "burn":
			default:
				continue
			}
			args := strings.Fields(first(in.InstallerSwitches.Silent, in.InstallerSwitches.SilentWithProgress,
				m.InstallerSwitches.Silent, m.InstallerSwitches.SilentWithProgress))
			if len(args) == 0 {
				args = defaultSilent[typ]
			}
			if len(args) == 0 || !strings.HasPrefix(strings.ToLower(in.InstallerURL), "https://") || len(in.InstallerSha256) != 64 {
				continue // generic exe without known silent switches, or missing URL/hash: not safe to run unattended
			}
			return Installer{URL: in.InstallerURL, SHA256: strings.ToLower(in.InstallerSha256), Type: typ, SilentArgs: append([]string(nil), args...)}, nil
		}
	}
	return Installer{}, ErrNoInstaller
}

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// ManifestURL is where winget-pkgs keeps the installer manifest of one package version.
func ManifestURL(base, id, ver string) string {
	segs := strings.Split(id, ".")
	return fmt.Sprintf("%s/manifests/%s/%s/%s/%s.installer.yaml",
		strings.TrimRight(base, "/"), strings.ToLower(id[:1]), strings.Join(segs, "/"), ver, id)
}

// FetchManifest downloads an installer manifest (capped at 4 MiB).
func FetchManifest(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("winget manifest %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}
