package main

import (
	"context"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/updater"
)

// demoBackend shows made-up apps and pretends to install them, so the window can be tried and
// screenshotted without touching the PC (run with --demo).
type demoBackend struct{}

func (demoBackend) Check(context.Context) ([]updater.Item, error) {
	mk := func(name, inst, latest string, st updater.Status) updater.Item {
		e := catalog.Entry{Source: "winget", ID: "Demo." + name, Version: latest}
		return updater.Item{App: inventory.App{Name: name, Version: inst, UninstallKey: name, Scope: "machine"}, Entry: &e, Status: st}
	}
	O, C := updater.Outdated, updater.UpToDate
	return []updater.Item{
		mk("Docker Desktop", "4.96.2", "4.100.1", O),
		mk("Epic Games Launcher", "1.3.193.0", "1.3.210.0", O),
		mk("Git", "2.56.0.2", "2.56.1", O),
		mk("GitHub CLI", "2.102.0", "2.105.0", O),
		mk("Google Chrome", "158.0.8127.37", "158.0.8127.89", O),
		mk("Microsoft Visual C++ v14 Redistributable (x64) - 14.51.36247", "14.51.36247.0", "14.52.37029.0", O),
		mk("Node.js", "26.12.0", "26.12.1", O),
		mk("Microsoft Windows Desktop Runtime - 8.0.8 (x64)", "8.0.8.33916", "8.0.34", updater.Superseded),
		mk("Notepad++ (64-bit x64)", "8.9.9.1", "8.9.9.1", C),
		mk("Cloudflare One Client", "26.8.2115.0", "26.8.2115.0", C),
		mk("Steam", "2.10.91.91", "2.10.91.91", C),
		mk("Smart Switch", "4.3.24094.1", "4.3.24062.1", C),
	}, nil
}

func (demoBackend) Install(context.Context, updater.Item) error {
	time.Sleep(1500 * time.Millisecond)
	return nil
}
