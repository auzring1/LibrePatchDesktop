// Package appicon holds the app's icon (the LibrePatch teal shield with the amber patch) for the window and tray.
package appicon

import _ "embed"

//go:embed icon.ico
var ico []byte

//go:embed icon-32.png
var png32 []byte

// ICO returns the multi-size .ico file.
func ICO() []byte { return ico }

// PNG returns the icon as a 32x32 PNG.
func PNG() []byte { return png32 }
