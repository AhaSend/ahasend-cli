// Package prompt asks the user for input on stderr and reads the answer
// from stdin: confirmations, which --force skips, answers, and secrets,
// which a terminal hides as they are typed. It decides when a command may
// prompt at all: stdin is a terminal and --non-interactive is not set.
package prompt

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// AddForceFlag adds the --force (-f) flag that skips Confirm.
func AddForceFlag(cmd *cobra.Command) {
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")
}

// Confirm asks the user to confirm message unless --force is set. The prompt
// goes to stderr and the answer is read from stdin, so stdout carries only the
// command's output. Any answer but "y" or "yes" cancels the operation, and so
// does stdin that holds no answer or cannot be read, with how to skip the
// confirmation when the command has --force.
func Confirm(cmd *cobra.Command, message string) error {
	if force, _ := cmd.Flags().GetBool("force"); force {
		return nil
	}

	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s (y/N): ", message); err != nil {
		return fmt.Errorf("failed to write confirmation prompt: %w", err)
	}

	answer, err := readLine(cmd)
	if err != nil {
		return unansweredError(cmd, err)
	}

	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return nil
	default:
		return errors.NewValidationError("operation cancelled", nil)
	}
}

// Prompt writes message to stderr and returns the next line of stdin, trimmed.
// A later Prompt or Confirm on the same stdin reads the line after it. Stdin
// ending before an answer cancels the operation, as promptError says.
func Prompt(cmd *cobra.Command, message string) (string, error) {
	answer, err := promptLine(cmd, message)
	return strings.TrimSpace(answer), err
}

// promptLine writes message to stderr and returns the next line of stdin,
// without its newline.
func promptLine(cmd *cobra.Command, message string) (string, error) {
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), message); err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}
	answer, err := readLine(cmd)
	if err != nil {
		return "", promptError(err)
	}
	return answer, nil
}

// PromptSecret is Prompt for a secret, such as an API key or a password,
// which keeps the answer as typed, as trimLineEnding says: when stdin is a
// terminal, the answer is hidden as it is typed. Ctrl+D, Ctrl+C and the end
// of input cancel the operation, as promptError says, and leave the terminal
// as it was; so does a SIGINT or SIGTERM, as exitOnSignal says.
func PromptSecret(cmd *cobra.Command, message string) (string, error) {
	in, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(in.Fd())) {
		answer, err := promptLine(cmd, message)
		return trimLineEnding(answer), err
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), message); err != nil {
		return "", fmt.Errorf("failed to write prompt: %w", err)
	}
	// In raw mode the terminal neither echoes the answer nor turns Ctrl+C
	// into a signal, so the terminal is always restored after the read.
	stopExitOnSignal := exitOnSignal(cmd, in)
	state, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		stopExitOnSignal()
		endPromptLine(cmd)
		return "", promptError(err)
	}
	answer, err := readHiddenLine(in, takeLFAfterCR(in))
	_ = term.Restore(int(in.Fd()), state)
	stopExitOnSignal()
	// The terminal shows neither the hidden answer nor its newline
	endPromptLine(cmd)
	if err != nil {
		return "", promptError(err)
	}
	return trimLineEnding(string(answer)), nil
}

// trimLineEnding returns line, a secret read from stdin or a prompt, such
// as an API key or a password, without the LF or CRLF that ends it. Every
// other character, a space included, is part of the secret. AhaSend API keys
// hold no spaces, so a key with one fails as the API refuses it.
func trimLineEnding(line string) string {
	return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
}

// The keys readHiddenLine reads from a terminal in raw mode.
const (
	keyCtrlC     = 0x03
	keyCtrlD     = 0x04
	keyCtrlU     = 0x15
	keyEscape    = 0x1b
	keyBackspace = 0x7f
)

// exitOnSignal restores the terminal in to the state it is in now and exits
// with 128 plus the signal's number, as the shell reports a process a signal
// ended, when a SIGINT or SIGTERM arrives before the returned function is
// called. In raw mode Ctrl+C is a key that readHiddenLine reads, so such a
// signal comes from elsewhere, such as kill, and would otherwise end the
// process with the terminal still hiding what is typed.
func exitOnSignal(cmd *cobra.Command, in *os.File) (stop func()) {
	state, stateErr := term.GetState(int(in.Fd()))
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-signals:
			if stateErr == nil {
				_ = term.Restore(int(in.Fd()), state)
			}
			endPromptLine(cmd)
			os.Exit(128 + int(sig.(syscall.Signal)))
		case <-done:
		}
	}()
	return func() {
		signal.Stop(signals)
		close(done)
	}
}

// readHiddenLine reads a line from a terminal in raw mode, up to Enter, and
// drops an LF it reads first when skipLF is set. It returns io.EOF on
// Ctrl+D, Ctrl+C or the end of input: a terminal at the end of input reads
// no byte and no error. Backspace deletes the last character and Ctrl+U the
// whole line; the escape sequences of keys such as the arrows and every
// other control character, such as Ctrl+Z or Tab, add nothing, so a key
// pressed by mistake never ends up in the answer. A line that ends with a
// CR, which Enter sends in raw mode, marks in for takeLFAfterCR when more
// input already waits, as the LF of a pasted CRLF does.
func readHiddenLine(in io.Reader, skipLF bool) ([]byte, error) {
	var line []byte
	for first := true; ; first = false {
		b, err := readByte(in)
		if err != nil {
			return nil, err
		}
		if first && skipLF && b == '\n' {
			continue
		}
		if b == '\r' {
			markLFAfterCR(in)
		}
		if b == keyEscape {
			if b, err = afterEscape(in); err != nil {
				return nil, err
			}
		}
		switch {
		case b == '\r' || b == '\n':
			return line, nil
		case b == keyCtrlC || b == keyCtrlD:
			return nil, io.EOF
		case b == keyBackspace || b == '\b':
			if len(line) > 0 {
				line = line[:len(line)-1]
			}
		case b == keyCtrlU:
			line = line[:0]
		case b < ' ':
			// Another control character
		default:
			line = append(line, b)
		}
	}
}

// crEnded is the stdin a hidden answer last ended on with a CR followed by
// input that already waited. An answer pasted with a CRLF line ending leaves
// its LF there, which the next prompt would read as an empty answer, so the
// next prompt on that stdin drops an LF that comes first. An answer typed and
// ended with Enter leaves nothing waiting, so the Enter of an empty answer at
// the next prompt is kept.
var (
	crEndedMu sync.Mutex
	crEnded   *os.File
)

// markLFAfterCR records that a hidden answer read from in ended with a CR,
// when more input already waits on in, as inputWaiting says. Only a
// terminal, an *os.File, is recorded.
func markLFAfterCR(in io.Reader) {
	if file, ok := in.(*os.File); ok && inputWaiting(file) {
		crEndedMu.Lock()
		defer crEndedMu.Unlock()
		crEnded = file
	}
}

// takeLFAfterCR reports whether the last hidden answer read from in ended
// with a CR, so an LF that comes first is to be dropped, and forgets it.
func takeLFAfterCR(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	crEndedMu.Lock()
	defer crEndedMu.Unlock()
	if crEnded != file {
		return false
	}
	crEnded = nil
	return true
}

// readByte reads one byte of in, so a read never takes input past the line;
// a read of no byte and no error is the end of input.
func readByte(in io.Reader) (byte, error) {
	next := make([]byte, 1)
	n, err := in.Read(next)
	if n == 0 {
		if err == nil {
			err = io.EOF
		}
		return 0, err
	}
	return next[0], nil
}

// afterEscape reads what follows an ESC. It reads the rest of an escape
// sequence and returns 0, a control character, which adds nothing to a line:
// a CSI sequence, ESC [ up to a final byte from @ to ~, such as an arrow
// key's ESC [ A, or an SS3 one, ESC O and one byte, such as F1's ESC O P.
// Any other byte, such as a key pressed after Esc or with Alt, it returns as
// typed; another ESC starts again.
func afterEscape(in io.Reader) (byte, error) {
	b, err := readByte(in)
	if err != nil {
		return 0, err
	}
	switch b {
	case keyEscape:
		return afterEscape(in)
	case '[':
		// The Linux console's F1 to F5 send ESC [ [ and a letter
		for {
			if b, err = readByte(in); err != nil {
				return 0, err
			}
			if b >= '@' && b <= '~' && b != '[' {
				return 0, nil
			}
		}
	case 'O':
		_, err = readByte(in)
		return 0, err
	}
	return b, nil
}

// unansweredError is the error of a confirmation whose answer could not be
// read, as promptError says, which names --force when the command has it:
// stdin that holds no answer, such as a pipe or a file, never confirms.
func unansweredError(cmd *cobra.Command, err error) error {
	if cmd.Flags().Lookup("force") == nil {
		return promptError(err)
	}
	const hint = "; use --force to skip the confirmation"
	if err == io.EOF {
		return errors.NewValidationError("operation cancelled"+hint, nil)
	}
	return errors.NewValidationError(fmt.Sprintf("operation cancelled: failed to read the answer: %v%s", err, hint), nil)
}

// promptError is the error of a prompt whose answer could not be read: the
// validation error "operation cancelled", like declining a confirmation,
// which names the read error unless stdin ended before an answer, such as on
// Ctrl+D.
func promptError(err error) error {
	if err == io.EOF {
		return errors.NewValidationError("operation cancelled", nil)
	}
	return errors.NewValidationError("operation cancelled: failed to read the answer", err)
}

// readLine returns the next line of stdin, without its LF, or io.EOF when stdin
// ended before any character of it, dropping an LF that comes first after a
// hidden answer that ended with a CR, as crEnded says. It reads one byte at a
// time, so it never consumes input past the line. When stdin is not a terminal, as when the
// answer is piped, nothing shows the answer and its newline, so it ends the
// prompt line itself.
func readLine(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	skipLF := takeLFAfterCR(in)
	var line []byte
	next := make([]byte, 1)
	for {
		n, err := in.Read(next)
		if n == 1 && skipLF {
			skipLF = false
			if next[0] == '\n' {
				continue
			}
		}
		if n == 1 {
			if next[0] == '\n' {
				if !isTerminal(in) {
					endPromptLine(cmd)
				}
				break
			}
			line = append(line, next[0])
		}
		if err != nil {
			endPromptLine(cmd)
		}
		if err == io.EOF && len(line) > 0 {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return string(line), nil
}

// endPromptLine ends the prompt line on stderr when no newline the terminal
// shows ends it: stdin ended before a newline, or is not a terminal. What
// follows, such as an error or the result, starts on a line of its own. An
// answer the user typed ends with the newline of Enter, which the terminal
// shows.
func endPromptLine(cmd *cobra.Command) {
	fmt.Fprintln(cmd.ErrOrStderr())
}
