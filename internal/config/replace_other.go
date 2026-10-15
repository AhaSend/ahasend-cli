//go:build !windows

package config

import "os"

// replaceFile replaces the file to with the file from.
func replaceFile(from, to string) error {
	return os.Rename(from, to)
}
