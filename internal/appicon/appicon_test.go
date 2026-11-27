package appicon

import (
	"bytes"
	"image/png"
	"testing"
)

func TestIcon(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(PNG()))
	if err != nil || img.Bounds().Dx() != 32 {
		t.Fatal(err)
	}
	ico := ICO()
	if len(ico) < 22 || ico[2] != 1 || !bytes.Equal(ico[22:], PNG()) {
		t.Fatal("ico layout")
	}
}
