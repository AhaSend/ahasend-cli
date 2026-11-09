package printer

import (
	stderrors "errors"
	"fmt"
	"io"
	"strings"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
)

// markupWarningsHeader names the row and the plain label of the warnings a
// write of AhaSend Markup (AhaML) returns.
const markupWarningsHeader = "Markup Warnings"

// formatMarkupProblem formats a problem of AhaML that a write refused, or a
// warning about AhaML that a write stored, on one line:
//
//	line <line>, column <column>: <message> (<code>) at <path>; hint: <hint>
//
// leaving out the place when the problem has no line, the path when it has
// none and the hint when it has none. Each field is escaped with
// EscapeControl, so a line break in a message cannot split the line.
func formatMarkupProblem(problem common.MarkupProblem) string {
	var line strings.Builder
	if problem.Line > 0 {
		fmt.Fprintf(&line, "line %d, column %d: ", problem.Line, problem.Column)
	}
	fmt.Fprintf(&line, "%s (%s)", EscapeControl(problem.Message), EscapeControl(problem.Code))
	if problem.Path != "" {
		line.WriteString(" at " + EscapeControl(problem.Path))
	}
	if problem.Hint != "" {
		line.WriteString("; hint: " + EscapeControl(problem.Hint))
	}
	return line.String()
}

// formatMarkupProblems formats each problem with formatMarkupProblem.
func formatMarkupProblems(problems []common.MarkupProblem) []string {
	lines := make([]string, len(problems))
	for i, problem := range problems {
		lines[i] = formatMarkupProblem(problem)
	}
	return lines
}

// markupWarningsRows returns the Markup Warnings row of a table, one warning
// per line, or no row when there is no warning.
func markupWarningsRows(warnings []common.MarkupProblem) [][]string {
	if len(warnings) == 0 {
		return nil
	}
	return [][]string{{markupWarningsHeader, strings.Join(formatMarkupProblems(warnings), "\n")}}
}

// writeMarkupWarnings writes the warnings as a "Markup Warnings:" line
// followed by each warning on its own indented line, or nothing when there
// is no warning.
func writeMarkupWarnings(w io.Writer, warnings []common.MarkupProblem) error {
	if len(warnings) == 0 {
		return nil
	}
	return writeIndentedList(w, markupWarningsHeader, formatMarkupProblems(warnings))
}

// formatMarkupWarningsCSV returns the markup_warnings cell: the warnings as
// compact JSON, or "" when there is none.
func formatMarkupWarningsCSV(warnings []common.MarkupProblem) (string, error) {
	if len(warnings) == 0 {
		return "", nil
	}
	encoded, err := encodeJSON(warnings, "")
	return string(encoded), err
}

// markupProblems returns the problems of the AhaML that the API refused in
// the API error in err's chain, or nil.
func markupProblems(err error) []common.MarkupProblem {
	var apiErr *api.APIError
	if stderrors.As(err, &apiErr) {
		return apiErr.Errors
	}
	return nil
}
