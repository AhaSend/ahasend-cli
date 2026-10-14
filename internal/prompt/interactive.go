package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

// isTerminal reports whether r is a terminal. Tests replace it to answer
// prompts from a buffer.
var isTerminal = func(r io.Reader) bool {
	file, ok := r.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// AddNonInteractiveFlag adds --non-interactive to a command that prompts for
// missing required flags.
func AddNonInteractiveFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("non-interactive", false, "Never prompt; fail when a required flag is missing")
}

// readsStdinAnnotation marks a flag added by AddStdinFlag.
const readsStdinAnnotation = "ahasend_reads_stdin"

// AddStdinFlag adds the bool flag name, which makes the command read a value
// from stdin, such as --password-stdin. A command given it never prompts,
// since stdin holds that value.
func AddStdinFlag(cmd *cobra.Command, name, usage string) {
	cmd.Flags().Bool(name, false, usage)
	_ = cmd.Flags().SetAnnotation(name, readsStdinAnnotation, []string{"true"})
}

// ReadStdinLine returns the first line of stdin, without its line break, for
// the flag name added by AddStdinFlag, which reads what, such as "password".
// Stdin without one, or with an empty first line, is a validation error.
func ReadStdinLine(cmd *cobra.Command, name, what string) (string, error) {
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", errors.NewValidationError(fmt.Sprintf("failed to read --%s", name), err)
	}
	line = trimLineEnding(line)
	if line == "" {
		return "", errors.NewValidationError(fmt.Sprintf("--%s read no %s from stdin", name, what), nil)
	}
	return line, nil
}

// readsStdin reports whether a flag added by AddStdinFlag is set.
func readsStdin(cmd *cobra.Command) bool {
	reads := false
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if _, ok := flag.Annotations[readsStdinAnnotation]; ok && flag.Value.String() == "true" {
			reads = true
		}
	})
	return reads
}

// CanPrompt reports whether the command may prompt for a missing required
// flag: stdin is a terminal, --non-interactive is not set, and no flag
// added by AddStdinFlag reads stdin.
func CanPrompt(cmd *cobra.Command) bool {
	if nonInteractive, _ := cmd.Flags().GetBool("non-interactive"); nonInteractive {
		return false
	}
	return !readsStdin(cmd) && isTerminal(cmd.InOrStdin())
}

// TreatStdinAsTerminalForTesting makes CanPrompt treat stdin as a terminal,
// so a test can answer prompts from a buffer. It returns the function that
// restores the check.
func TreatStdinAsTerminalForTesting() (restore func()) {
	previous := isTerminal
	isTerminal = func(io.Reader) bool { return true }
	return func() { isTerminal = previous }
}
