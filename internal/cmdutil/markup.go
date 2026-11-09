package cmdutil

import (
	"fmt"
	"io"
	"os"

	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// AddMarkupFlag adds --markup to the get command of a template, a campaign
// or a campaign template.
func AddMarkupFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("markup", false, "Write only the design's AhaML to stdout, as --markup-file reads it")
}

// ReadMarkupFlag returns whether --markup is set. --markup writes the markup
// and nothing else, so it cannot be used with --output, even set to false.
func ReadMarkupFlag(cmd *cobra.Command) (bool, error) {
	if err := RejectTogether(cmd, "markup", "output"); err != nil {
		return false, err
	}
	markup, _ := cmd.Flags().GetBool("markup")
	return markup, nil
}

// WriteMarkup writes the AhaML of content, the design of what ("template",
// "campaign" or "campaign template") id, to stdout and nothing else: to a
// file or a pipe exactly as the API returned it, and to a terminal escaped
// as table and plain output escape their text. Content with no AhaML is an
// error that says why.
func WriteMarkup(cmd *cobra.Command, what, id string, content *responses.TemplateContent) error {
	if content == nil {
		return fmt.Errorf("%s %s has no content", what, id)
	}
	if content.MarkupError != nil {
		return fmt.Errorf("the design could not be read as AhaML: %s: %s",
			printer.EscapeControl(content.MarkupError.Code), printer.EscapeControl(content.MarkupError.Message))
	}
	if content.Markup == nil || *content.Markup == "" {
		return fmt.Errorf("%s %s has no AhaML: only a Simple Editor design has one", what, id)
	}
	markup := *content.Markup
	out := cmd.OutOrStdout()
	if isTerminal(out) {
		markup = printer.EscapeDisplayText(markup)
	}
	_, err := io.WriteString(out, markup)
	return err
}

// isTerminal reports whether w is a terminal.
func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
