package integration

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestHiddenPromptRestoresTheTerminalOnASignal sends SIGINT or SIGTERM to
// the CLI while the API key prompt of auth login hides what is typed. The
// CLI leaves the terminal as it was, echoing and reading lines, and exits
// with 128 plus the signal's number, as the shell reports a process a signal
// ended.
func TestHiddenPromptRestoresTheTerminalOnASignal(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the CLI binary")
	}

	binary := buildCLIBinary(t)
	for name, sig := range map[string]syscall.Signal{"SIGINT": syscall.SIGINT, "SIGTERM": syscall.SIGTERM} {
		t.Run(name, func(t *testing.T) {
			terminal := testutil.NewTerminal(t)
			home := t.TempDir()
			command := exec.Command(binary, "auth", "login", "--api-url", "http://127.0.0.1:1")
			command.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home, "NO_COLOR=1")
			command.Stdin = terminal.Stdin
			require.NoError(t, command.Start())

			terminal.WaitForHiding(t)
			require.NoError(t, command.Process.Signal(sig))
			err := command.Wait()

			var exitErr *exec.ExitError
			require.True(t, errors.As(err, &exitErr), "want an exit error, got %v", err)
			assert.Equal(t, 128+int(sig), exitErr.ExitCode())
			termios, err := unix.IoctlGetTermios(int(terminal.Stdin.Fd()), unix.TCGETS)
			require.NoError(t, err)
			assert.NotZero(t, termios.Lflag&unix.ECHO, "the terminal still hides what is typed")
			assert.NotZero(t, termios.Lflag&unix.ICANON, "the terminal is still in raw mode")
		})
	}
}
