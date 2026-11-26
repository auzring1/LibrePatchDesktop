// Command librepatchdesktop lists installed applications and updates them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/auzring1/librepatchdesktop/internal/winget"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"text/tabwriter"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/elevate"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/ui"
	"github.com/auzring1/librepatchdesktop/internal/updater"
)

// version is set at build time with -ldflags "-X main.version=1.2.3".
var version = "dev"

const usage = `LibrePatchDesktop: keep your applications up to date (applications only, no Windows Update).

Usage:
  librepatchdesktop [flags] [gui]     open the window (default when started by double-click)
  librepatchdesktop [flags] list      show installed applications
  librepatchdesktop [flags] check     show which have updates
  librepatchdesktop [flags] update    install all available updates silently
Flags:`

func main() {
	catPath := flag.String("catalog", "", "optional extra catalog JSON file, consulted before winget")
	demo := flag.Bool("demo", false, "open the window with made-up apps (nothing is installed)")
	noWinget := flag.Bool("no-winget", false, "do not use the winget community index")
	fake := flag.Bool("fake", false, "use a built-in sample app list instead of the Windows registry")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}
	// Installers usually need administrator rights. An installed copy has a scheduled task that runs it
	// elevated without a prompt; otherwise (portable exe) ask with UAC.
	if !*fake && !*demo && (flag.NArg() == 0 || flag.Arg(0) == "gui" || flag.Arg(0) == "update") && !elevate.IsAdmin() {
		if elevate.RunTask("LibrePatchDesktop") == nil || elevate.Relaunch() == nil {
			return
		}
		fmt.Fprintln(os.Stderr, "continuing without administrator rights: some updates may fail")
	}
	if flag.NArg() == 0 || flag.Arg(0) == "gui" {
		var b ui.Backend
		if *demo {
			b = demoBackend{}
		} else {
			var sc inventory.Scanner = inventory.NewScanner()
			if *fake {
				sc = fakeScanner()
			}
			b = &backend{scanner: sc, catPath: *catPath, noWinget: *noWinget}
		}
		if err := runGUI(b); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(flag.Arg(0), *catPath, *fake, *noWinget); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd, catPath string, fake, noWinget bool) error {
	var sc inventory.Scanner = inventory.NewScanner()
	if fake {
		sc = fakeScanner()
	}
	apps, err := sc.Scan()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	defer w.Flush()
	if cmd == "list" {
		for _, a := range apps {
			fmt.Fprintf(w, "%s\t%s\t%s\n", a.Name, a.Version, a.Publisher)
		}
		return nil
	}
	if cmd != "check" && cmd != "update" {
		return fmt.Errorf("unknown command %q", cmd)
	}
	sources, err := loadSources(catPath, noWinget)
	if err != nil {
		return err
	}
	items := updater.Check(apps, sources)
	failed := 0
	for _, it := range items {
		if it.Status == updater.Unknown {
			continue
		}
		latest := it.Entry.Version
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", it.App.Name, it.App.Version, latest, it.Status)
		if cmd == "update" && it.Status == updater.Outdated {
			w.Flush()
			if err := updater.Install(context.Background(), it.Source, *it.Entry, nil); err != nil {
				fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
				failed++
			} else {
				fmt.Fprintln(os.Stdout, "  updated")
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d update(s) failed", failed)
	}
	return nil
}

// loadSources builds the ordered catalog sources: an optional local file first, then winget.
func loadSources(catPath string, noWinget bool) ([]catalog.Source, error) {
	var out []catalog.Source
	if catPath != "" {
		fs, err := catalog.LoadFile(catPath)
		if err != nil {
			return nil, err
		}
		out = append(out, fs)
	}
	if !noWinget {
		cache, err := os.UserCacheDir()
		if err != nil {
			cache = os.TempDir()
		}
		client := &http.Client{Timeout: 5 * time.Minute}
		snap, err := winget.LoadIndex(context.Background(), client, filepath.Join(cache, "LibrePatchDesktop"))
		if err != nil {
			return nil, err
		}
		arch := map[string]string{"amd64": "x64", "386": "x86", "arm64": "arm64"}[runtime.GOARCH]
		out = append(out, &winget.Provider{Matcher: winget.NewMatcher(snap), Client: client, ManifestBase: winget.ManifestBase, Arch: arch})
	}
	if len(out) == 0 {
		return nil, errors.New("no catalog source: give --catalog or drop --no-winget")
	}
	return out, nil
}

func fakeScanner() inventory.Scanner {
	return inventory.Static{{Name: "7-Zip 23.01 (x64)", Version: "23.01", Scope: "machine"}}
}
