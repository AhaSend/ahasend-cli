package prompt

import (
	"bytes"
	stderrors "errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parsed returns a command whose flags were added by addFlags and parsed from
// args.
func parsed(t *testing.T, addFlags func(*cobra.Command), args ...string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	addFlags(cmd)
	require.NoError(t, cmd.ParseFlags(args))
	return cmd
}

func assertValidationError(t *testing.T, err error, message string) {
	t.Helper()
	require.Error(t, err)
	var cliErr *errors.CLIError
	require.True(t, stderrors.As(err, &cliErr), "want a CLIError, got %T", err)
	assert.Equal(t, errors.ErrCodeValidation, cliErr.Code)
	assert.Contains(t, err.Error(), message)
}

func confirmCommand(t *testing.T, stdin string, args ...string) (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	cmd := parsed(t, AddForceFlag, args...)
	var stdout, stderr bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	return cmd, &stdout, &stderr
}

func TestConfirm_Accepted(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "YES\n", " y "} {
		t.Run(answer, func(t *testing.T) {
			cmd, _, _ := confirmCommand(t, answer)
			assert.NoError(t, Confirm(cmd, "Delete it?"))
		})
	}
}

func TestConfirm_Declined(t *testing.T) {
	for _, answer := range []string{"n\n", "\n", "", "yep\n"} {
		t.Run(answer, func(t *testing.T) {
			cmd, _, _ := confirmCommand(t, answer)
			assertValidationError(t, Confirm(cmd, "Delete it?"), "operation cancelled")
		})
	}
}

// failingReader fails every read as reading a stdin that cannot be read,
// such as a closed terminal, does.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, &os.PathError{Op: "read", Path: "/dev/stdin", Err: syscall.EIO}
}

// TestPromptsCancelWhenStdinCannotBeRead checks that a prompt whose answer
// cannot be read cancels the operation, a validation error that exits 4.
func TestPromptsCancelWhenStdinCannotBeRead(t *testing.T) {
	prompts := map[string]func(*cobra.Command) error{
		"confirm": func(cmd *cobra.Command) error { return Confirm(cmd, "Delete it?") },
		"prompt": func(cmd *cobra.Command) error {
			_, err := Prompt(cmd, "Name: ")
			return err
		},
		"secret": func(cmd *cobra.Command) error {
			_, err := PromptSecret(cmd, "Key: ")
			return err
		},
	}
	for name, prompt := range prompts {
		t.Run(name, func(t *testing.T) {
			cmd, _, _ := confirmCommand(t, "")
			cmd.SetIn(failingReader{})

			err := prompt(cmd)

			want := "operation cancelled: failed to read the answer: read /dev/stdin: input/output error"
			if name == "confirm" {
				want += "; use --force to skip the confirmation"
			}
			require.EqualError(t, err, want)
			assert.Equal(t, 4, errors.GetExitCode(err))
		})
	}
}

// TestConfirm_NoAnswerNamesForce checks that a confirmation stdin holds no
// answer for, such as an empty pipe, says how to skip it when the command
// has --force, and only then.
func TestConfirm_NoAnswerNamesForce(t *testing.T) {
	cmd, _, _ := confirmCommand(t, "")
	assert.EqualError(t, Confirm(cmd, "Delete it?"), "operation cancelled; use --force to skip the confirmation")

	withoutForce := &cobra.Command{Use: "test"}
	withoutForce.SetIn(strings.NewReader(""))
	withoutForce.SetErr(&bytes.Buffer{})
	assert.EqualError(t, Confirm(withoutForce, "Delete it?"), "operation cancelled")
}

func TestConfirm_PromptGoesToStderr(t *testing.T) {
	cmd, stdout, stderr := confirmCommand(t, "y\n")
	require.NoError(t, Confirm(cmd, "Delete it?"))
	assert.Equal(t, "Delete it? (y/N): \n", stderr.String())
	assert.Empty(t, stdout.String())
}

func TestConfirm_NoAnswerEndsThePromptLine(t *testing.T) {
	cmd, _, stderr := confirmCommand(t, "")
	require.Error(t, Confirm(cmd, "Delete it?"))
	assert.Equal(t, "Delete it? (y/N): \n", stderr.String())
}

// TestPrompt_EndsThePromptLineUnlessATerminalShowsTheAnswer checks that the
// prompt line ends after an answer read from anything but a terminal, such
// as a pipe, which shows neither the answer nor its newline, and after no
// answer; a terminal shows the newline of Enter itself.
func TestPrompt_EndsThePromptLineUnlessATerminalShowsTheAnswer(t *testing.T) {
	tests := map[string]struct {
		stdin, stderr string
		terminal      bool
		wantErr       bool
	}{
		"piped answer":               {"Acme\n", "Name: \n", false, false},
		"piped answer without Enter": {"Acme", "Name: \n", false, false},
		"no answer":                  {"", "Name: \n", false, true},
		"answer typed on a terminal": {"Acme\n", "Name: ", true, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.terminal {
				t.Cleanup(TreatStdinAsTerminalForTesting())
			}
			cmd, _, stderr := confirmCommand(t, test.stdin)
			_, err := Prompt(cmd, "Name: ")
			assert.Equal(t, test.wantErr, err != nil, err)
			assert.Equal(t, test.stderr, stderr.String())
		})
	}
}

// TestPromptSecret_HidesTheAnswerOnATerminal checks that a terminal does not
// show the secret as it is typed, and that the prompt line still ends.
func TestPromptSecret_HidesTheAnswerOnATerminal(t *testing.T) {
	terminal := testutil.NewTerminal(t)
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
		answer, err := PromptSecret(cmd, "Password: ")
		cmd.Print(answer)
		return err
	}}

	wait := terminal.Start(t, cmd, "plain")
	terminal.TypeHidden(t, "s3cret")
	stdout, stderr, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "s3cret", stdout)
	assert.Equal(t, "Password: \n", stderr)
}

// TestPromptSecret_CancelsOnCtrlDOrCtrlC checks that Ctrl+D and Ctrl+C at
// a hidden prompt cancel the operation and leave the terminal echoing.
func TestPromptSecret_CancelsOnCtrlDOrCtrlC(t *testing.T) {
	for name, key := range map[string]string{"Ctrl+D": "\x04", "Ctrl+C": "\x03"} {
		t.Run(name, func(t *testing.T) {
			terminal := testutil.NewTerminal(t)
			cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
				_, err := PromptSecret(cmd, "Password: ")
				return err
			}}

			wait := terminal.Start(t, cmd, "plain")
			terminal.PressHidden(t, "s3c"+key)
			_, stderr, err := wait()

			assertValidationError(t, err, "operation cancelled")
			assert.Equal(t, 4, errors.GetExitCode(err))
			assert.Equal(t, "Password: \nError: operation cancelled\n", stderr)
			assert.True(t, terminal.Echoes(t))
		})
	}
}

// TestReadHiddenLine_KeepsOnlyWhatIsTyped checks the keys a hidden prompt
// handles: Backspace and Ctrl+U edit the line, and the escape sequences of
// keys such as the arrows, Esc and every other control key add nothing.
func TestReadHiddenLine_KeepsOnlyWhatIsTyped(t *testing.T) {
	tests := map[string]string{
		"plain":                  "s3cret\r",
		"backspace":              "s3cx\x7fret\r",
		"Ctrl+H":                 "s3cx\bret\r",
		"Ctrl+U clears the line": "wrong\x15s3cret\r",
		"arrow keys":             "s3\x1b[D\x1b[Ccret\x1b[A\r",
		"CSI with parameters":    "s3c\x1b[1;5Cret\x1b[200~\r",
		"SS3 function key":       "s3c\x1bOPret\r",
		"Linux console F1":       "s3c\x1b[[Aret\r",
		"Esc alone":              "s3c\x1bret\r",
		"Esc twice":              "s3c\x1b\x1b[Dret\r",
		"Ctrl+Z and Tab":         "s3\x1ac\tret\r",
		"LF ends it too":         "s3cret\nnext",
	}
	for name, typed := range tests {
		t.Run(name, func(t *testing.T) {
			line, err := readHiddenLine(strings.NewReader(typed), false)

			require.NoError(t, err)
			assert.Equal(t, "s3cret", string(line))
		})
	}
}

// TestPromptSecret_DropsControlKeysOnATerminal checks, on a terminal, that
// an arrow key, Esc, Ctrl+Z and Ctrl+U typed at a hidden prompt never end up
// in the answer.
func TestPromptSecret_DropsControlKeysOnATerminal(t *testing.T) {
	terminal := testutil.NewTerminal(t)
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
		answer, err := PromptSecret(cmd, "Password: ")
		cmd.Print("[" + answer + "]")
		return err
	}}

	wait := terminal.Start(t, cmd, "plain")
	terminal.TypeHidden(t, "oops\x15s3\x1b[Dc\x1ar\x1bet")
	stdout, _, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "[s3cret]", stdout)
}

// TestPromptSecret_DropsTheLFOfAPastedCRLF checks, on a terminal, that a
// secret pasted with a CRLF line ending leaves no LF behind, which the next
// prompt would read as an empty answer.
func TestPromptSecret_DropsTheLFOfAPastedCRLF(t *testing.T) {
	for name, next := range map[string]func(*cobra.Command, string) (string, error){
		"then a prompt": Prompt, "then a hidden prompt": PromptSecret,
	} {
		t.Run(name, func(t *testing.T) {
			terminal := testutil.NewTerminal(t)
			cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
				secret, err := PromptSecret(cmd, "Key: ")
				if err != nil {
					return err
				}
				answer, err := next(cmd, "Next: ")
				cmd.Print("[" + secret + "][" + answer + "]")
				return err
			}}

			wait := terminal.Start(t, cmd, "plain")
			terminal.PressHidden(t, "s3cret\r\n")
			if name == "then a hidden prompt" {
				terminal.TypeHidden(t, "Acme")
			} else {
				terminal.WaitForEcho(t)
				terminal.Type(t, "Acme")
			}
			stdout, _, err := wait()

			require.NoError(t, err)
			assert.Equal(t, "[s3cret][Acme]", stdout)
		})
	}
}

// TestPromptSecret_KeepsTheEnterAfterATypedSecret checks, on a terminal,
// that a secret typed and ended with Enter, a CR in raw mode, leaves the next
// prompt its own Enter: an empty answer there is read as one.
func TestPromptSecret_KeepsTheEnterAfterATypedSecret(t *testing.T) {
	terminal := testutil.NewTerminal(t)
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
		secret, err := PromptSecret(cmd, "Key: ")
		if err != nil {
			return err
		}
		answer, err := Prompt(cmd, "Next: ")
		cmd.Print("[" + secret + "][" + answer + "]")
		return err
	}}

	wait := terminal.Start(t, cmd, "plain")
	terminal.PressHidden(t, "s3cret\r")
	terminal.WaitForEcho(t)
	terminal.Type(t, "")
	stdout, _, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "[s3cret][]", stdout)
}

func TestPromptSecret_ReadsALineFromOtherInput(t *testing.T) {
	cmd, _, stderr := confirmCommand(t, "s3cret\nnext\n")
	answer, err := PromptSecret(cmd, "Password: ")
	require.NoError(t, err)
	assert.Equal(t, "s3cret", answer)
	assert.Equal(t, "Password: \n", stderr.String())
}

// TestPromptSecret_KeepsSpacesFromOtherInput checks that a secret read from
// a pipe loses only its line ending, LF or CRLF, as --password-stdin does.
func TestPromptSecret_KeepsSpacesFromOtherInput(t *testing.T) {
	cmd, _, _ := confirmCommand(t, " s3c ret \r\nnext\n")
	answer, err := PromptSecret(cmd, "Password: ")
	require.NoError(t, err)
	assert.Equal(t, " s3c ret ", answer)
}

// TestPromptSecret_KeepsSpacesOnATerminal checks that a secret typed at a
// hidden prompt keeps the spaces typed around it.
func TestPromptSecret_KeepsSpacesOnATerminal(t *testing.T) {
	terminal := testutil.NewTerminal(t)
	cmd := &cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, _ []string) error {
		answer, err := PromptSecret(cmd, "Password: ")
		cmd.Print("[" + answer + "]")
		return err
	}}

	wait := terminal.Start(t, cmd, "plain")
	terminal.TypeHidden(t, " s3c ret ")
	stdout, _, err := wait()

	require.NoError(t, err)
	assert.Equal(t, "[ s3c ret ]", stdout)
}

// TestPrompt_TrimsTheAnswer checks that an answer that is not a secret, such
// as an account ID, loses the spaces around it.
func TestPrompt_TrimsTheAnswer(t *testing.T) {
	cmd, _, _ := confirmCommand(t, "  answer \r\n")
	answer, err := Prompt(cmd, "Account ID: ")
	require.NoError(t, err)
	assert.Equal(t, "answer", answer)
}

func TestTrimLineEnding(t *testing.T) {
	tests := map[string]string{
		"key\n":       "key",
		"key\r\n":     "key",
		" key \n":     " key ",
		"key":         "key",
		"key\r\r\n":   "key\r",
		"\tkey\t\r\n": "\tkey\t",
	}
	for line, want := range tests {
		assert.Equal(t, want, trimLineEnding(line), "%q", line)
	}
}

func TestPromptSecret_EndOfInputCancels(t *testing.T) {
	cmd, _, _ := confirmCommand(t, "")
	_, err := PromptSecret(cmd, "Password: ")
	assertValidationError(t, err, "operation cancelled")
}

// TestConfirm_LeavesTheNextLineForTheNextPrompt checks that Confirm reads
// only its answer, so input piped for later prompts reaches them.
func TestConfirm_LeavesTheNextLineForTheNextPrompt(t *testing.T) {
	cmd, _, _ := confirmCommand(t, "y\nAcme\n")
	require.NoError(t, Confirm(cmd, "Continue?"))
	name, err := Prompt(cmd, "Name: ")
	require.NoError(t, err)
	assert.Equal(t, "Acme", name)
}

func TestConfirm_ForceSkipsPrompt(t *testing.T) {
	cmd, stdout, stderr := confirmCommand(t, "", "--force")
	require.NoError(t, Confirm(cmd, "Delete it?"))
	assert.Empty(t, stderr.String())
	assert.Empty(t, stdout.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, stderrors.New("broken pipe") }

func TestConfirm_PromptWriteFailure(t *testing.T) {
	cmd, _, _ := confirmCommand(t, "y\n")
	cmd.SetErr(failingWriter{})
	err := Confirm(cmd, "Delete it?")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken pipe")
}
