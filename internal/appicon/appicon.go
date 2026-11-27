// Package appicon draws the app's icon (a teal round badge with a white up arrow) so no image file is needed.
package appicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
)

const size = 32

// PNG returns the icon as a 32x32 PNG.
func PNG() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	teal, white := color.NRGBA{23, 179, 176, 255}, color.NRGBA{255, 255, 255, 255}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)-15.5, float64(y)-15.5
			if dx*dx+dy*dy <= 15.5*15.5 {
				img.SetNRGBA(x, y, teal)
			}
		}
	}
	for y := 8; y < 24; y++ { // an up arrow
		if y >= 12 {
			for x := 14; x < 18; x++ {
				img.SetNRGBA(x, y, white)
			}
		}
		if y < 15 {
			w := y - 7
			for x := 16 - w; x < 16+w; x++ {
				img.SetNRGBA(x, y, white)
			}
		}
	}
	var p bytes.Buffer
	png.Encode(&p, img)
	return p.Bytes()
}

// ICO wraps the PNG as an .ico file (PNG-in-ICO, supported since Windows Vista).
func ICO() []byte {
	p := PNG()
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, [3]uint16{0, 1, 1}) // reserved, type icon, image count
	b.Write([]byte{size, size, 0, 0})                         // width, height, colours, reserved
	binary.Write(&b, binary.LittleEndian, [2]uint16{1, 32})   // planes, bits per pixel
	binary.Write(&b, binary.LittleEndian, [2]uint32{uint32(len(p)), 22})
	b.Write(p)
	return b.Bytes()
}
