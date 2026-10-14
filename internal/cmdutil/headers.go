package cmdutil

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// ReadHeaders returns the custom headers of the repeatable flag name, each
// given as "Name: value", by header name, or nil when the flag was not set. A
// value without a colon or a name is a validation error, and so is a name
// the API refuses: one that holds a space or a character that is not visible
// ASCII. The name is checked as given, so the part of a header value
// after an unquoted ", ", which the flag splits off as another header, fails
// for its leading space; a part after an unquoted "," fails for its missing
// colon. Both errors say to quote a value with a comma. Header names ignore
// case, so a name given twice, in any case, is a validation error.
func ReadHeaders(cmd *cobra.Command, name string) (map[string]string, error) {
	values, err := readRepeatableAsGiven(cmd, name, "")
	if err != nil || values == nil {
		return nil, err
	}
	quoteHint := commaHint(name, "a header value", "Name: a, b")
	headers := make(map[string]string, len(values))
	for i, value := range values {
		header, content, found := strings.Cut(value, ":")
		if !found && fromSplitValue(cmd, name, i) {
			return nil, validation.InvalidValueError("--"+name, value, "not Name: value; "+quoteHint)
		}
		if !found || strings.TrimSpace(header) == "" {
			return nil, errors.NewValidationError(fmt.Sprintf("--%s must be Name: value", name), nil)
		}
		header = strings.TrimRight(header, " \t")
		if !isHeaderName(header) {
			return nil, validation.InvalidValueError("--"+name, value, fmt.Sprintf(
				`%q is not a header name, which is visible ASCII without spaces; %s`, header, quoteHint))
		}
		if _, ok := FindHeader(headers, header); ok {
			return nil, errors.NewValidationError(fmt.Sprintf(
				"--%s gives the header %s twice; header names ignore case", name, header), nil)
		}
		headers[header] = strings.TrimSpace(content)
	}
	return headers, nil
}

// ReservedHeader is a header name that --header refuses, in any case, with
// the hint that says how to set it instead.
type ReservedHeader struct {
	Name string
	Hint string
}

// CheckReservedHeaders returns a validation error that names the first of
// reserved that headers holds, in any case, with its hint.
func CheckReservedHeaders(headers map[string]string, reserved []ReservedHeader) error {
	for _, header := range reserved {
		if name, ok := FindHeader(headers, header.Name); ok {
			return errors.NewValidationError(fmt.Sprintf("--header %s cannot be used; %s", name, header.Hint), nil)
		}
	}
	return nil
}

// FindHeader returns the name in headers that is name in any case, and
// whether there is one.
func FindHeader(headers map[string]string, name string) (string, bool) {
	for given := range headers {
		if strings.EqualFold(given, name) {
			return given, true
		}
	}
	return "", false
}

// commaHint is the hint of a value of the repeatable flag name that the flag
// split at a comma, which what names, such as "a header value", with example
// as such a value.
func commaHint(name, what, example string) string {
	return fmt.Sprintf(`%s with a comma must be in double quotes: --%s '"%s"'`, what, name, example)
}

// isHeaderName reports whether name is a header name the API takes: RFC
// 5322's field name, one or more visible ASCII characters other than the
// colon.
func isHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if c := name[i]; c <= ' ' || c > '~' || c == ':' {
			return false
		}
	}
	return true
}
