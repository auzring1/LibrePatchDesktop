//go:build windows

// Package window shows a page in a native Windows window using the Microsoft Edge WebView2 runtime
// (already present on Windows 11 and on most Windows 10 PCs with Edge installed).
package window

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jchv/go-webview2"
)

// Run opens url in its own window and returns when the window is closed.
// It returns an error, without opening anything, if the WebView2 runtime cannot be started.
func Run(url, title string) error {
	runtime.LockOSThread()
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dir, "LibrePatchDesktop", "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  1000,
			Height: 720,
			Center: true,
		},
	})
	if w == nil {
		return errors.New("the Microsoft Edge WebView2 runtime is not available")
	}
	defer w.Destroy()
	w.Navigate(url)
	w.Run()
	return nil
}
