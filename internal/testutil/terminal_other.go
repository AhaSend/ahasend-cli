//go:build !linux

package testutil

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
)

// Terminal is a pseudo-terminal for tests of prompts. Tests that need one
// run on Linux only.
type Terminal struct {
	Stdin *os.File
}

// NewTerminal skips the test: pseudo-terminals are tested on Linux only.
func NewTerminal(t *testing.T) *Terminal {
	t.Skip("pseudo-terminal tests run on Linux only")
	return nil
}

// Start is never reached: NewTerminal skips the test.
func (term *Terminal) Start(t *testing.T, cmd *cobra.Command, format string, args ...string) (wait func() (string, string, error)) {
	return nil
}

// TypeHidden is never reached: NewTerminal skips the test.
func (term *Terminal) TypeHidden(t *testing.T, line string) {}

// PressHidden is never reached: NewTerminal skips the test.
func (term *Terminal) PressHidden(t *testing.T, keys string) {}

// WaitForHiding is never reached: NewTerminal skips the test.
func (term *Terminal) WaitForHiding(t *testing.T) {}

// WaitForEcho is never reached: NewTerminal skips the test.
func (term *Terminal) WaitForEcho(t *testing.T) {}

// Echoes is never reached: NewTerminal skips the test.
func (term *Terminal) Echoes(t *testing.T) bool { return false }

// Type is never reached: NewTerminal skips the test.
func (term *Terminal) Type(t *testing.T, line string) {}

// Shown is never reached: NewTerminal skips the test.
func (term *Terminal) Shown(t *testing.T) func() string { return nil }
