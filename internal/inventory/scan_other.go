//go:build !windows

package inventory

import "errors"

// NewScanner is only functional on Windows.
func NewScanner() Scanner { return unsupported{} }

type unsupported struct{}

func (unsupported) Scan() ([]App, error) {
	return nil, errors.New("scanning installed applications is only supported on Windows (use --fake to try the app)")
}
