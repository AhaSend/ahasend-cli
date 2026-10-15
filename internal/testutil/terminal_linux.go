package testutil

import (
	"bytes"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// Terminal is a pseudo-terminal for tests of prompts: the command reads its
// stdin from Stdin, a terminal, and the test types into it.
type Terminal struct {
	Stdin  *os.File
	master *os.File
}

// NewTerminal opens a pseudo-terminal that closes when the test ends.
func NewTerminal(t *testing.T) *Terminal {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = master.Close() })
	require.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))
	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	require.NoError(t, err)
	stdin, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = stdin.Close() })
	return &Terminal{Stdin: stdin, master: master}
}

// Start runs a leaf command like RunCommand, with the terminal as its stdin,
// while the test types into the terminal. The returned function waits for
// the command to end and returns its stdout, stderr and error; it fails the
// test when the command has not ended after a few seconds.
func (term *Terminal) Start(t *testing.T, cmd *cobra.Command, format string, args ...string) (wait func() (string, string, error)) {
	t.Helper()
	type result struct {
		stdout, stderr string
		err            error
	}
	done := make(chan result, 1)
	go func() {
		stdout, stderr, err := runCommand(t, cmd, format, term.Stdin, args...)
		done <- result{stdout, stderr, err}
	}()
	return func() (string, string, error) {
		select {
		case r := <-done:
			return r.stdout, r.stderr, r.err
		case <-time.After(5 * time.Second):
			require.FailNow(t, "the command still waits for input")
			return "", "", nil
		}
	}
}

// TypeHidden waits until the terminal stops echoing what is typed, as it
// does while a prompt hides its answer, then types line and Enter. It fails
// the test when the terminal still echoes after a few seconds.
func (term *Terminal) TypeHidden(t *testing.T, line string) {
	t.Helper()
	term.PressHidden(t, line+"\n")
}

// PressHidden waits like TypeHidden, then types keys as they are, such as
// "\x04" for Ctrl+D.
func (term *Terminal) PressHidden(t *testing.T, keys string) {
	t.Helper()
	term.WaitForHiding(t)
	_, err := term.master.WriteString(keys)
	require.NoError(t, err)
}

// WaitForHiding waits until the terminal stops echoing what is typed, as it
// does while a prompt hides its answer. It fails the test when the terminal
// still echoes after a few seconds.
func (term *Terminal) WaitForHiding(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for term.Echoes(t) {
		if time.Now().After(deadline) {
			_ = term.master.Close()
			require.FailNow(t, "the prompt did not hide the answer: the terminal still echoes input")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// WaitForEcho waits until the terminal shows what is typed again, as it does
// once a hidden prompt is over. It fails the test when the terminal still
// hides input after a few seconds.
func (term *Terminal) WaitForEcho(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !term.Echoes(t) {
		if time.Now().After(deadline) {
			require.FailNow(t, "the terminal still hides input")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Echoes reports whether the terminal shows what is typed.
func (term *Terminal) Echoes(t *testing.T) bool {
	t.Helper()
	termios, err := unix.IoctlGetTermios(int(term.Stdin.Fd()), unix.TCGETS)
	require.NoError(t, err)
	return termios.Lflag&unix.ECHO != 0
}

// Type types line and Enter, whether the terminal echoes them or not.
func (term *Terminal) Type(t *testing.T, line string) {
	t.Helper()
	_, err := term.master.WriteString(line + "\n")
	require.NoError(t, err)
}

// Shown returns what was written to the terminal, Stdin, which a command can
// write to as its stderr, as the terminal received it: each newline written
// is a carriage return and a newline. Reading starts at the first call; the
// returned function returns what was received so far.
func (term *Terminal) Shown(t *testing.T) func() string {
	t.Helper()
	var mu sync.Mutex
	var shown bytes.Buffer
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := term.master.Read(buf)
			mu.Lock()
			shown.Write(buf[:n])
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return shown.String()
	}
}
