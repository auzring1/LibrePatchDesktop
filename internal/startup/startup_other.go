//go:build !windows

package startup

import "errors"

// List returns nothing off Windows.
func List() ([]Entry, error) { return nil, nil }

// SetEnabled is only available on Windows.
func SetEnabled(string, bool) error {
	return errors.New("startup entries are only supported on Windows")
}
