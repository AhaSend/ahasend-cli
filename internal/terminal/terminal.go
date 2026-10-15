// Package terminal decides what the CLI writes to a terminal: whether a
// stream is a terminal that acts on escape sequences, and whether output to
// it is colored.
package terminal

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// IsEscapeTerminal reports whether w is a terminal that acts on escape
// sequences, such as colors and the progress line's clearing: a terminal
// whose TERM is not dumb, the name of a terminal that acts on none, and
// whose escape processing is on, which turnOnEscapes turns on where it is
// off, as on an old Windows console. Output to anything else, or to a
// console whose processing cannot be turned on, has no escape sequences.
func IsEscapeTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && os.Getenv("TERM") != "dumb" && term.IsTerminal(int(file.Fd())) && turnOnEscapes(file)
}

// UseColor reports whether the command colors what it writes to w: only when
// w is a terminal that acts on escape sequences and the user did not turn
// colors off, with --no-color or with the NO_COLOR environment variable set
// to a non-empty value (https://no-color.org). Logs, stderr notices and the
// listen commands' output all follow it.
func UseColor(cmd *cobra.Command, w io.Writer) bool {
	noColor, _ := cmd.Flags().GetBool("no-color")
	return !noColor && os.Getenv("NO_COLOR") == "" && IsEscapeTerminal(w)
}
