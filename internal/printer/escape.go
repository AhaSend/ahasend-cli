package printer

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

// EscapeControl returns s with each control character, C0, DEL and C1, and
// each character that hides or reorders text, written as a \uXXXX escape, so
// text from the API, a server or a file, such as a subject, cannot send escape
// sequences to the terminal or show other text than it holds. It is for text
// shown on one line: tabs and newlines are escaped too.
func EscapeControl(s string) string {
	return escapeControl(s, false)
}

// EscapeDisplayText is EscapeControl for text shown on several lines: it
// keeps tabs, newlines and a carriage return that ends a line before a
// newline. Table and plain output, which are display formats, show every
// value with it; JSON and CSV output, which are data formats, print values
// exactly.
func EscapeDisplayText(s string) string {
	return escapeControl(s, true)
}

func escapeControl(s string, keepLines bool) string {
	kept := func(i int, r rune) bool {
		return keepLines && (r == '\t' || r == '\n' || (r == '\r' && strings.HasPrefix(s[i+1:], "\n")))
	}
	escape := false
	for i, r := range s {
		if escaped(r) && !kept(i, r) {
			escape = true
			break
		}
	}
	if !escape {
		return s
	}
	var b strings.Builder
	for i, r := range s {
		if escaped(r) && !kept(i, r) {
			fmt.Fprintf(&b, "\\u%04x", r)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// escaped reports whether r is written as an escape: a control character, or
// a zero-width or bidirectional formatting character, which hides text or
// shows it in another order than it has.
func escaped(r rune) bool {
	switch {
	case unicode.IsControl(r):
		return true
	case r >= '\u200b' && r <= '\u200f', // zero-width characters and direction marks
		r >= '\u202a' && r <= '\u202e', // bidirectional embeddings and overrides
		r >= '\u2066' && r <= '\u2069', // bidirectional isolates
		r == '\ufeff':                  // zero-width no-break space
		return true
	}
	return false
}

// DisplayWriter returns a writer that writes to w what is written to it,
// escaped with EscapeDisplayText, for text that comes from the API, a server
// or a file and is shown on the terminal. Each write must hold whole
// characters, as the fmt functions write. Text with color escape sequences
// must not go through it: escape its values instead.
func DisplayWriter(w io.Writer) io.Writer {
	return displayWriter{w: w}
}

// displayWriter writes to w what table and plain output print, escaped with
// EscapeDisplayText, so no value from the API reaches the terminal with its
// control characters. Each write must hold whole characters, as the fmt
// functions and the table writer write.
type displayWriter struct {
	w io.Writer
}

func (d displayWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(d.w, EscapeDisplayText(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}
