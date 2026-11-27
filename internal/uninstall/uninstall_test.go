package uninstall

import (
	"strings"
	"testing"

	"github.com/auzring1/librepatchdesktop/internal/inventory"
)

func TestFor(t *testing.T) {
	m := func(a inventory.App) inventory.App { a.Scope = "machine"; return a }
	p, err := For(m(inventory.App{UninstallString: `MsiExec.exe /X{23170F69-40C1-2702-2600-000001000000}`, WindowsInstaller: true}))
	if err != nil || !p.Silent || !strings.HasSuffix(p.CommandLine, `/x {23170F69-40C1-2702-2600-000001000000} /qn /norestart`) || !strings.Contains(p.CommandLine, `\System32\msiexec.exe`) {
		t.Fatalf("msi: %+v %v", p, err)
	}
	p, err = For(m(inventory.App{UninstallString: `"C:\Program Files\X\unins000.exe"`, QuietUninstallString: `"C:\Program Files\X\unins000.exe" /VERYSILENT`}))
	if err != nil || !p.Silent || !strings.Contains(p.CommandLine, "/VERYSILENT") {
		t.Fatalf("quiet: %+v %v", p, err)
	}
	p, err = For(m(inventory.App{UninstallString: `C:\Program Files\Y\uninstall.exe /S`}))
	if err != nil || p.Silent {
		t.Fatalf("plain: %+v %v", p, err)
	}
	for name, a := range map[string]inventory.App{
		"user scope": {Scope: "user", UninstallString: `C:\a\b.exe`},
		"empty":      m(inventory.App{}),
		"relative":   m(inventory.App{UninstallString: `uninstall.exe`}),
	} {
		if _, err := For(a); err == nil {
			t.Errorf("%s should be refused", name)
		}
	}
}

func TestSplitExe(t *testing.T) {
	e, r := SplitExe(`"C:\A B\x.exe" /S /x`)
	if e != `C:\A B\x.exe` || r != "/S /x" {
		t.Fatal(e, r)
	}
	e, r = SplitExe(`C:\A\x.exe /S`)
	if e != `C:\A\x.exe` || r != "/S" {
		t.Fatal(e, r)
	}
}
