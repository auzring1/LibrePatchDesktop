// Package uninstall works out how to remove an installed program and runs the command.
package uninstall

import (
	"errors"
	"regexp"
	"strings"

	"github.com/auzring1/librepatchdesktop/internal/inventory"
	"github.com/auzring1/librepatchdesktop/internal/sysdir"
)

var msiProduct = regexp.MustCompile(`(?i)msiexec(?:\.exe)?"?\s+/[ix]\s*("?)(\{[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\})`)

// Plan is what will be run.
type Plan struct {
	CommandLine string // full command line, starting with an absolute program path
	Silent      bool   // false means the program's own uninstaller will show windows the user must click through
}

// ErrNone means the program registered no uninstall command.
var ErrNone = errors.New("this program has no uninstall command")

// For builds the plan for an installed program. Only entries installed for all users are accepted: entries in
// a user's own registry hive can be written by any program running as that user, and this app runs with
// administrator rights.
func For(a inventory.App) (Plan, error) {
	if a.Scope != "machine" {
		return Plan{}, errors.New("this program was installed for one user only; remove it from Windows Settings > Apps")
	}
	cmd := strings.TrimSpace(a.QuietUninstallString)
	silent := cmd != ""
	if !silent {
		cmd = strings.TrimSpace(a.UninstallString)
	}
	if cmd == "" {
		return Plan{}, ErrNone
	}
	if m := msiProduct.FindStringSubmatch(cmd); m != nil {
		return Plan{CommandLine: `"` + sysdir.System32("msiexec.exe") + `" /x ` + m[2] + ` /qn /norestart`, Silent: true}, nil
	}
	if a.WindowsInstaller && !silent {
		return Plan{}, errors.New("unrecognised MSI uninstall command")
	}
	exe, _ := SplitExe(cmd)
	if !isAbsolute(exe) {
		return Plan{}, errors.New("uninstall command does not start with a full program path")
	}
	return Plan{CommandLine: cmd, Silent: silent}, nil
}

// SplitExe returns the program path at the start of a command line, quoted or not, and the rest.
func SplitExe(cmd string) (exe, rest string) {
	cmd = strings.TrimSpace(cmd)
	if strings.HasPrefix(cmd, `"`) {
		if i := strings.Index(cmd[1:], `"`); i >= 0 {
			return cmd[1 : 1+i], strings.TrimSpace(cmd[2+i:])
		}
		return strings.Trim(cmd, `"`), ""
	}
	low := strings.ToLower(cmd)
	if i := strings.Index(low, ".exe"); i >= 0 {
		return cmd[:i+4], strings.TrimSpace(cmd[i+4:])
	}
	if i := strings.IndexByte(cmd, ' '); i >= 0 {
		return cmd[:i], strings.TrimSpace(cmd[i:])
	}
	return cmd, ""
}

func isAbsolute(p string) bool {
	return len(p) > 2 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') || strings.HasPrefix(p, `\\`)
}
