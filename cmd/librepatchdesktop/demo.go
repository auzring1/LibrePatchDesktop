package main

import (
	"context"
	"strings"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/selfupdate"
	"github.com/auzring1/librepatchdesktop/internal/startup"
	"github.com/auzring1/librepatchdesktop/internal/ui"
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
		mk("Audacity", "3.7.1", "3.7.1", updater.Unknown),
		mk("Cloudflare One Client", "26.8.2115.0", "26.8.2115.0", C),
		mk("Steam", "2.10.91.91", "2.10.91.91", C),
		mk("Smart Switch", "4.3.24094.1", "4.3.24062.1", C),
	}, nil
}

func (demoBackend) Install(context.Context, updater.Item) error {
	time.Sleep(1500 * time.Millisecond)
	return nil
}

func (demoBackend) Library(q string, limit int) []ui.LibraryItem {
	all := []ui.LibraryItem{{ID: "Mozilla.Firefox", Name: "Mozilla Firefox", Version: "161.0"}, {ID: "VideoLAN.VLC", Name: "VLC media player", Version: "3.0.21"},
		{ID: "7zip.7zip", Name: "7-Zip", Version: "26.00"}, {ID: "Microsoft.VisualStudioCode", Name: "Visual Studio Code", Version: "1.105.0"},
		{ID: "Spotify.Spotify", Name: "Spotify", Version: "1.2.80"}, {ID: "Discord.Discord", Name: "Discord", Version: "1.0.9200"}}
	var out []ui.LibraryItem
	for _, i := range all {
		if q == "" || strings.Contains(strings.ToLower(i.Name), strings.ToLower(q)) {
			out = append(out, i)
		}
	}
	return out
}

func (demoBackend) InstallPackage(context.Context, string) error {
	time.Sleep(1500 * time.Millisecond)
	return nil
}

func (demoBackend) Uninstall(context.Context, inventory.App) error {
	time.Sleep(1200 * time.Millisecond)
	return nil
}

func (demoBackend) Startups() ([]startup.Entry, error) {
	return []startup.Entry{
		{ID: "hkcu|run|Spotify", Name: "Spotify", Command: `C:\Users\me\AppData\Roaming\Spotify\Spotify.exe --minimized`, Location: "Current user (registry)", Enabled: true},
		{ID: "hkcu|run|Discord", Name: "Discord", Command: `C:\Users\me\AppData\Local\Discord\Update.exe --processStart Discord.exe`, Location: "Current user (registry)", Enabled: false},
		{ID: "hklm|run|SecurityHealth", Name: "SecurityHealth", Command: `C:\Windows\System32\SecurityHealthSystray.exe`, Location: "All users (registry)", Enabled: true},
	}, nil
}

func (demoBackend) SetStartupEnabled(string, bool) error { return nil }

func (demoBackend) LatestRelease(context.Context) (selfupdate.Release, error) {
	return selfupdate.Release{Version: "0.4.0", PageURL: "https://github.com/auzring1/LibrePatchDesktop/releases"}, nil
}

func (demoBackend) InstallRelease(context.Context, selfupdate.Release) error { return nil }
