//go:build windows

// Package tray puts an icon in the Windows notification area with Open and Quit.
package tray

import (
	"time"

	"fyne.io/systray"

	"github.com/auzring1/librepatchdesktop/internal/appicon"
)

// Start shows the tray icon. open is called when the icon is clicked or "Open" chosen; quit on "Quit".
// It reports whether the icon appeared (within a few seconds).
func Start(open, quit func()) bool {
	ready := make(chan struct{})
	go func() {
		defer func() { recover() }()
		systray.Run(func() {
			systray.SetIcon(appicon.ICO())
			systray.SetTooltip("LibrePatchDesktop")
			systray.SetOnTapped(open)
			o := systray.AddMenuItem("Open LibrePatchDesktop", "")
			systray.AddSeparator()
			q := systray.AddMenuItem("Quit", "")
			close(ready)
			go func() {
				for {
					select {
					case <-o.ClickedCh:
						open()
					case <-q.ClickedCh:
						quit()
					}
				}
			}()
		}, nil)
	}()
	select {
	case <-ready:
		return true
	case <-time.After(5 * time.Second):
		return false
	}
}
