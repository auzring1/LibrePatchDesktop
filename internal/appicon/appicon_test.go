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
	if i := ICO(); len(i) < 22 || i[2] != 1 || i[4] < 2 {
		t.Fatal("ico layout")
	}
}
