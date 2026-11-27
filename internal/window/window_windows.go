//go:build windows

// Package window shows a page in a native Windows window using the Microsoft Edge WebView2 runtime
// (already present on Windows 11 and on most Windows 10 PCs with Edge installed).
package window

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2"
	"golang.org/x/sys/windows/registry"

	"github.com/auzring1/librepatchdesktop/internal/appicon"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	dwmapi = syscall.NewLazyDLL("dwmapi.dll")
)

// init makes the process DPI aware before any window exists. Without this Windows renders the whole window at
// 96 DPI and stretches it, which looks blurry on scaled displays.
func init() {
	if p := user32.NewProc("SetProcessDpiAwarenessContext"); p.Find() == nil {
		const perMonitorV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
		if r, _, _ := p.Call(perMonitorV2); r != 0 {
			return
		}
	}
	user32.NewProc("SetProcessDPIAware").Call()
}

var (
	mu   sync.Mutex
	hwnd uintptr
)

// Colours of the title bar, matching the page (COLORREF is 0x00BBGGRR).
const (
	darkBar, darkText   = 0x0B0906, 0xF3F0E8 // #06090b, #e8f0f3
	lightBar, lightText = 0xEEECE4, 0x2A2314 // #e4ecee, #14232a
)

// isDark resolves "system", "dark" or "light".
func isDark(theme string) bool {
	switch theme {
	case "dark":
		return true
	case "light":
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return true
	}
	defer k.Close()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err != nil || v == 0
}

// SetTheme paints the title bar to match the page's theme ("system", "dark" or "light"). It does nothing
// when no window is open. The caption colour needs Windows 11; Windows 10 gets a plain dark or light bar.
func SetTheme(theme string) {
	mu.Lock()
	h := hwnd
	mu.Unlock()
	if h == 0 {
		return
	}
	dark := isDark(theme)
	bar, text := uint32(lightBar), uint32(lightText)
	var d uint32
	if dark {
		bar, text, d = darkBar, darkText, 1
	}
	set := func(attr uintptr, v uint32) {
		dwmapi.NewProc("DwmSetWindowAttribute").Call(h, attr, uintptr(unsafe.Pointer(&v)), 4)
	}
	set(20, d)    // DWMWA_USE_IMMERSIVE_DARK_MODE
	set(35, bar)  // DWMWA_CAPTION_COLOR
	set(36, text) // DWMWA_TEXT_COLOR
	set(34, bar)  // DWMWA_BORDER_COLOR
}

func setIcon(h uintptr) {
	ico := appicon.PNG()
	create := user32.NewProc("CreateIconFromResourceEx")
	hicon, _, _ := create.Call(uintptr(unsafe.Pointer(&ico[0])), uintptr(len(ico)), 1, 0x00030000, 32, 32, 0)
	if hicon == 0 {
		return
	}
	send := user32.NewProc("SendMessageW")
	send.Call(h, 0x0080, 0, hicon) // WM_SETICON, ICON_SMALL
	send.Call(h, 0x0080, 1, hicon) // WM_SETICON, ICON_BIG
}

func dpiScale() uint {
	if p := user32.NewProc("GetDpiForSystem"); p.Find() == nil {
		if d, _, _ := p.Call(); d >= 96 {
			return uint(d)
		}
	}
	return 96
}

// Run opens url in its own window and returns when the window is closed. theme is "system", "dark" or "light".
// It returns an error, without opening anything, if the WebView2 runtime cannot be started.
func Run(url, title, theme string) error {
	runtime.LockOSThread()
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dpi := dpiScale()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dir, "LibrePatchDesktop", "webview"),
		WindowOptions: webview2.WindowOptions{
			Title:  title,
			Width:  1100 * dpi / 96,
			Height: 760 * dpi / 96,
			Center: true,
		},
	})
	if w == nil {
		return errors.New("the Microsoft Edge WebView2 runtime is not available")
	}
	defer w.Destroy()
	h := uintptr(w.Window())
	mu.Lock()
	hwnd = h
	mu.Unlock()
	defer func() {
		mu.Lock()
		hwnd = 0
		mu.Unlock()
	}()
	SetTheme(theme)
	setIcon(h)
	w.Navigate(url)
	w.Run()
	return nil
}
