package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/auzring1/librepatchdesktop/internal/catalog"
	"github.com/auzring1/librepatchdesktop/internal/inventory"
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

// runGUI serves the page on a loopback port and shows it in a native window (WebView2). If that window cannot
// be created, it falls back to the default browser. A second launch while the app is running does nothing.
func runGUI(scanner inventory.Scanner, catPath string, noWinget bool) error {
	url := "http://" + guiAddr + "/"
	ln, err := net.Listen("tcp", guiAddr)
	if err != nil {
		return nil // already running
	}
	s := ui.New(&backend{scanner: scanner, catPath: catPath, noWinget: noWinget})
	s.Quit = func() { os.Exit(0) }
	go http.Serve(ln, s.Handler())
	if err := window.Run(url, "LibrePatchDesktop"); err != nil {
		fmt.Fprintln(os.Stderr, "no native window:", err)
		fmt.Println("LibrePatchDesktop is running at", url)
		openBrowser(url)
		select {} // keep serving until Quit
	}
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "open this address in your browser:", url)
	}
}
