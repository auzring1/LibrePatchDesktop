//go:build windows

package inventory

import (
	"golang.org/x/sys/windows/registry"
)

// NewScanner reads the Windows uninstall registry keys.
func NewScanner() Scanner { return regScanner{} }

type regScanner struct{}

type root struct {
	key   registry.Key
	path  string
	scope string
	flag  uint32
}

func (regScanner) Scan() ([]App, error) {
	const p = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	roots := []root{
		{registry.LOCAL_MACHINE, p, "machine", registry.WOW64_64KEY},
		{registry.LOCAL_MACHINE, p, "machine", registry.WOW64_32KEY},
		{registry.CURRENT_USER, p, "user", registry.WOW64_64KEY},
	}
	var apps []App
	for _, r := range roots {
		k, err := registry.OpenKey(r.key, r.path, registry.ENUMERATE_SUB_KEYS|r.flag)
		if err != nil {
			continue
		}
		names, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, n := range names {
			sk, err := registry.OpenKey(r.key, r.path+`\`+n, registry.QUERY_VALUE|r.flag)
			if err != nil {
				continue
			}
			app := App{Scope: r.scope, UninstallKey: n}
			app.Name, _, _ = sk.GetStringValue("DisplayName")
			app.Version, _, _ = sk.GetStringValue("DisplayVersion")
			app.Publisher, _, _ = sk.GetStringValue("Publisher")
			app.UninstallString, _, _ = sk.GetStringValue("UninstallString")
			app.QuietUninstallString, _, _ = sk.GetStringValue("QuietUninstallString")
			if wi, _, err := sk.GetIntegerValue("WindowsInstaller"); err == nil && wi == 1 {
				app.WindowsInstaller = true
			}
			// Skip Windows components and update entries: this app does not manage them.
			if sc, _, err := sk.GetIntegerValue("SystemComponent"); err == nil && sc == 1 {
				sk.Close()
				continue
			}
			if _, _, err := sk.GetStringValue("ParentKeyName"); err == nil {
				sk.Close()
				continue
			}
			sk.Close()
			apps = append(apps, app)
		}
	}
	return Clean(apps), nil
}
