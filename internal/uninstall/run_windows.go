//go:build windows

package uninstall

import (
	"os/exec"
	"syscall"
)

// Run starts the plan's command line as written (the registry's own quoting is kept) and waits for it.
func Run(p Plan) error {
	exe, _ := SplitExe(p.CommandLine)
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: p.CommandLine, HideWindow: p.Silent}
	return cmd.Run()
}
