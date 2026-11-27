package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/elevate"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/settings"
	"github.com/auzring1/librepatchdesktop/internal/sysdir"
	"github.com/auzring1/librepatchdesktop/internal/tray"
	"github.com/auzring1/librepatchdesktop/internal/ui"
	"github.com/auzring1/librepatchdesktop/internal/updater"
	"github.com/auzring1/librepatchdesktop/internal/window"
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
	if runtime.GOOS == "windows" {
		s.SetStartup = elevate.SetStartup
	}
	s.Quit = func() { os.Exit(0) }
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
		if err := window.Run(url, "LibrePatchDesktop"); err != nil {
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
