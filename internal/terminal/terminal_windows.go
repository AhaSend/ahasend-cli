//go:build windows

package terminal

import (
	"os"

	"golang.org/x/sys/windows"
)

// turnOnEscapes turns on the processing of escape sequences of the console
// file writes to, which an old Windows console leaves off, and reports
// whether it is on. fatih/color turns it on for stdout alone.
func turnOnEscapes(file *os.File) bool {
	handle := windows.Handle(file.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	if mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0 {
		return true
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_PROCESSED_OUTPUT|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}
