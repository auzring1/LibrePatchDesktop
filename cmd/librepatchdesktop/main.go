// Command librepatchdesktop lists installed applications and updates them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/updater"
)

const usage = `LibrePatchDesktop: keep your applications up to date (applications only, no Windows Update).

Usage:
  librepatchdesktop [flags] list      show installed applications
  librepatchdesktop [flags] check     show which have updates
  librepatchdesktop [flags] update    install all available updates silently
Flags:`

func main() {
	catPath := flag.String("catalog", "catalog.json", "path to the catalog JSON file")
	fake := flag.Bool("fake", false, "use a built-in sample app list instead of the Windows registry")
	flag.Usage = func() { fmt.Fprintln(os.Stderr, usage); flag.PrintDefaults() }
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(flag.Arg(0), *catPath, *fake); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(cmd, catPath string, fake bool) error {
	var sc inventory.Scanner = inventory.NewScanner()
	if fake {
		sc = inventory.Static{{Name: "7-Zip 23.01 (x64)", Version: "23.01", Scope: "machine"}}
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
	entries, err := catalog.File(catPath).Entries()
	if err != nil {
		return err
	}
	items := updater.Check(apps, entries)
	failed := 0
	for _, it := range items {
		if it.Status == updater.Unknown {
			continue
		}
		latest := it.Entry.Version
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", it.App.Name, it.App.Version, latest, it.Status)
		if cmd == "update" && it.Status == updater.Outdated {
			w.Flush()
			if err := updater.Install(context.Background(), *it.Entry, nil); err != nil {
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
