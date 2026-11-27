//go:build windows

package startup

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

const runPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
const approvedPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved`

type hive struct {
	id    string
	key   registry.Key
	run   string
	flag  uint32
	label string
}

var hives = []hive{
	{"hkcu", registry.CURRENT_USER, runPath, 0, "Current user (registry)"},
	{"hklm", registry.LOCAL_MACHINE, runPath, registry.WOW64_64KEY, "All users (registry)"},
	{"hklm32", registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Run`, 0, "All users, 32-bit (registry)"},
}

func approved(h hive, sub string) (registry.Key, error) {
	return registry.OpenKey(h.key, approvedPath+`\`+sub, registry.QUERY_VALUE|registry.SET_VALUE|h.flag)
}

func folders() map[string]string {
	m := map[string]string{}
	if d := os.Getenv("APPDATA"); d != "" {
		m["user"] = filepath.Join(d, `Microsoft\Windows\Start Menu\Programs\Startup`)
	}
	if d := os.Getenv("ProgramData"); d != "" {
		m["common"] = filepath.Join(d, `Microsoft\Windows\Start Menu\Programs\StartUp`)
	}
	return m
}

// List returns every startup entry with its on/off state.
func List() ([]Entry, error) {
	var out []Entry
	for _, h := range hives {
		k, err := registry.OpenKey(h.key, h.run, registry.QUERY_VALUE|h.flag)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(-1)
		ak, aerr := approved(h, "Run")
		for _, n := range names {
			cmd, _, err := k.GetStringValue(n)
			if err != nil {
				continue
			}
			on := true
			if aerr == nil {
				if v, _, err := ak.GetBinaryValue(n); err == nil {
					on = isEnabled(v)
				}
			}
			out = append(out, Entry{ID: h.id + "|run|" + n, Name: n, Command: cmd, Location: h.label, Enabled: on})
		}
		if aerr == nil {
			ak.Close()
		}
		k.Close()
	}
	for scope, dir := range folders() {
		files, _ := os.ReadDir(dir)
		h := hives[0]
		if scope == "common" {
			h = hives[1]
		}
		ak, aerr := approved(h, "StartupFolder")
		for _, f := range files {
			if f.IsDir() || strings.EqualFold(f.Name(), "desktop.ini") {
				continue
			}
			on := true
			if aerr == nil {
				if v, _, err := ak.GetBinaryValue(f.Name()); err == nil {
					on = isEnabled(v)
				}
			}
			loc := "Startup folder (current user)"
			if scope == "common" {
				loc = "Startup folder (all users)"
			}
			out = append(out, Entry{ID: scope + "|folder|" + f.Name(), Name: strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())), Command: filepath.Join(dir, f.Name()), Location: loc, Enabled: on})
		}
		if aerr == nil {
			ak.Close()
		}
	}
	return out, nil
}

// SetEnabled switches an entry (an ID from List) on or off.
func SetEnabled(id string, on bool) error {
	parts := strings.SplitN(id, "|", 3)
	if len(parts) != 3 || parts[2] == "" {
		return errors.New("unknown startup entry")
	}
	sub, h := "Run", hive{}
	switch {
	case parts[1] == "run":
		for _, x := range hives {
			if x.id == parts[0] {
				h = x
			}
		}
	case parts[1] == "folder":
		sub = "StartupFolder"
		h = hives[0]
		if parts[0] == "common" {
			h = hives[1]
		}
	}
	if h.id == "" {
		return errors.New("unknown startup entry")
	}
	k, _, err := registry.CreateKey(h.key, approvedPath+`\`+sub, registry.SET_VALUE|h.flag)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetBinaryValue(parts[2], approvedValue(on, time.Now()))
}
