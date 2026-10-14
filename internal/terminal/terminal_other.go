//go:build !windows

package terminal

import "os"

// turnOnEscapes reports that a terminal acts on escape sequences, as every
// terminal outside Windows does.
func turnOnEscapes(*os.File) bool {
	return true
}
