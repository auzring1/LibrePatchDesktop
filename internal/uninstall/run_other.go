//go:build !windows

package uninstall

import "errors"

// Run is only available on Windows.
func Run(Plan) error { return errors.New("uninstalling is only supported on Windows") }
