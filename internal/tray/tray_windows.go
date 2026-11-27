//go:build windows

// Package tray puts an icon in the Windows notification area with Open and Quit.
package tray

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"time"

	"fyne.io/systray"
)

// Start shows the tray icon. open is called when the icon is clicked or "Open" chosen; quit on "Quit".
// It reports whether the icon appeared (within a few seconds).
func Start(open, quit func()) bool {
	ready := make(chan struct{})
	go func() {
		defer func() { recover() }()
		systray.Run(func() {
			systray.SetIcon(icon())
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

// icon draws a teal round badge with a white arrow and wraps it as a PNG-in-ICO file.
func icon() []byte {
	const n = 32
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	teal, white := color.NRGBA{23, 179, 176, 255}, color.NRGBA{255, 255, 255, 255}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if dx*dx+dy*dy <= 15.5*15.5 {
				img.SetNRGBA(x, y, teal)
			}
		}
	}
	for y := 8; y < 24; y++ { // an up arrow
		for x := 14; x < 18; x++ {
			if y >= 12 {
				img.SetNRGBA(x, y, white)
			}
		}
		w := y - 7
		if y < 15 {
			for x := 16 - w; x < 16+w; x++ {
				img.SetNRGBA(x, y, white)
			}
		}
	}
	var p bytes.Buffer
	png.Encode(&p, img)
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, [3]uint16{0, 1, 1}) // ICONDIR: reserved, type icon, count
	b.Write([]byte{n, n, 0, 0})                               // width, height, colours, reserved
	binary.Write(&b, binary.LittleEndian, [2]uint16{1, 32})   // planes, bits per pixel
	binary.Write(&b, binary.LittleEndian, [2]uint32{uint32(p.Len()), 22})
	b.Write(p.Bytes())
	return b.Bytes()
}
