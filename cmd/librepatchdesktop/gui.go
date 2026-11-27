package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/elevate"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/selfupdate"
	"github.com/auzring1/librepatchdesktop/internal/settings"
	"github.com/auzring1/librepatchdesktop/internal/startup"
	"github.com/auzring1/librepatchdesktop/internal/sysdir"
	"github.com/auzring1/librepatchdesktop/internal/tray"
	"github.com/auzring1/librepatchdesktop/internal/ui"
	"github.com/auzring1/librepatchdesktop/internal/uninstall"
	"github.com/auzring1/librepatchdesktop/internal/updater"
	"github.com/auzring1/librepatchdesktop/internal/window"
	"github.com/auzring1/librepatchdesktop/internal/winget"
)

type backend struct {
	scanner  inventory.Scanner
	catPath  string
	noWinget bool
	sources  []catalog.Source
}

func (b *backend) Check(context.Context) ([]updater.Item, error) {
	apps, err := b.scanner.Scan()
	if err != nil {
		return nil, err
	}
	if b.sources == nil {
		if b.sources, err = loadSources(b.catPath, b.noWinget); err != nil {
			return nil, err
		}
	}
	return updater.Check(apps, b.sources), nil
}

func (b *backend) provider() (*winget.Provider, error) {
	if b.sources == nil {
		var err error
		if b.sources, err = loadSources(b.catPath, b.noWinget); err != nil {
			return nil, err
		}
	}
	for _, s := range b.sources {
		if p, ok := s.(*winget.Provider); ok {
			return p, nil
		}
	}
	return nil, errors.New("the winget catalog is turned off")
}

func (b *backend) Library(q string, limit int) []ui.LibraryItem {
	p, err := b.provider()
	if err != nil {
		return nil
	}
	var out []ui.LibraryItem
	for _, pkg := range p.Library(q, limit) {
		out = append(out, ui.LibraryItem{ID: pkg.ID, Name: pkg.Name, Version: pkg.LatestVersion})
	}
	return out
}

func (b *backend) InstallPackage(ctx context.Context, id string) error {
	p, err := b.provider()
	if err != nil {
		return err
	}
	e, ok := p.Entry(id)
	if !ok {
		return errors.New("package not found")
	}
	return updater.Install(ctx, p, e, nil)
}

func (b *backend) Uninstall(_ context.Context, a inventory.App) error {
	plan, err := uninstall.For(a)
	if err != nil {
		return err
	}
	return uninstall.Run(plan)
}

func (b *backend) Startups() ([]startup.Entry, error)         { return startup.List() }
func (b *backend) SetStartupEnabled(id string, on bool) error { return startup.SetEnabled(id, on) }

func (b *backend) LatestRelease(ctx context.Context) (selfupdate.Release, error) {
	return selfupdate.Latest(ctx, &http.Client{Timeout: 20 * time.Second}, selfupdate.APIBase)
}

func (b *backend) InstallRelease(ctx context.Context, rel selfupdate.Release) error {
	return selfupdate.Install(ctx, &http.Client{Timeout: 10 * time.Minute}, rel, selfupdate.RunSetup)
}

func (b *backend) Install(ctx context.Context, it updater.Item) error {
	return updater.Install(ctx, it.Source, *it.Entry, nil)
}

// guiAddr is the fixed local address the window's page is served from.
const guiAddr = "127.0.0.1:47318"

// runGUI serves the page on a loopback port and shows it in a native window (WebView2). With the tray icon
// available, closing the window can keep the app running in the tray (scheduled updates keep working); with
// background set, it starts hidden in the tray. If the native window cannot be created, it falls back to the
// default browser. A second launch while the app is running does nothing.
func runGUI(b ui.Backend, background bool) error {
	url := "http://" + guiAddr + "/"
	ln, err := net.Listen("tcp", guiAddr)
	if err != nil {
		return nil // already running
	}
	s := ui.New(b)
	s.Version = version
	if dir, err := os.UserConfigDir(); err == nil {
		s.Settings = settings.Open(filepath.Join(dir, "LibrePatchDesktop", "settings.json"))
	}
	if dir, err := os.UserCacheDir(); err == nil {
		s.LogDir = filepath.Join(dir, "LibrePatchDesktop", "logs")
		if l, err := ui.OpenLog(s.LogDir); err == nil {
			s.Log = l
			l.Printf("LibrePatchDesktop %s started", version)
		}
	}
	if runtime.GOOS == "windows" {
		s.SetStartup = elevate.SetStartup
	}
	s.Quit = func() { os.Exit(0) }
	s.OnTheme = window.SetTheme
	go http.Serve(ln, s.Handler())
	go s.RunScheduler(context.Background())

	openReq := make(chan struct{}, 1)
	trayOK := tray.Start(func() {
		select {
		case openReq <- struct{}{}:
		default:
		}
	}, func() { os.Exit(0) })

	if background && !trayOK {
		background = false // nothing would show the app, so open the window after all
	}
	for {
		if background {
			<-openReq
			background = false
		}
		if err := window.Run(url, "LibrePatchDesktop", s.Settings.Get().Theme); err != nil {
			fmt.Fprintln(os.Stderr, "no native window:", err)
			fmt.Println("LibrePatchDesktop is running at", url)
			openBrowser(url)
			select {} // keep serving until Quit
		}
		if !trayOK || !s.Settings.Get().CloseToTray {
			return nil
		}
		background = true // window closed: wait in the tray until it is opened again
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(sysdir.System32("rundll32.exe"), "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("/usr/bin/open", url)
	default:
		cmd = exec.Command("/usr/bin/xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "open this address in your browser:", url)
	}
}
