// Package cmdutil holds the flag helpers that commands share: cursor
// pagination, partial-update readers, repeatable flags, required flags that
// can be asked for, idempotency keys, and the content and sender flags of
// templates and campaigns.
//
// Readers return errors from the errors package, so a bad flag fails as a
// validation error before the command authenticates.
package cmdutil

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ChangedString returns the value of a string flag, or nil when the flag was
// not set on the command line. An explicitly empty value is returned as a
// pointer to "".
func ChangedString(cmd *cobra.Command, name string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	value, _ := cmd.Flags().GetString(name)
	return &value
}

// NonEmptyString returns the value of a string flag, or nil when the value is
// empty or blank, of white space only, so such a filter is left out of the
// request.
func NonEmptyString(cmd *cobra.Command, name string) *string {
	value, _ := cmd.Flags().GetString(name)
	if isBlank(value) {
		return nil
	}
	return &value
}

// ChangedBool returns the value of a bool flag, or nil when the flag was not
// set on the command line.
func ChangedBool(cmd *cobra.Command, name string) *bool {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	value, _ := cmd.Flags().GetBool(name)
	return &value
}

// ReadNonEmpty returns the value of a string flag, or nil when the flag was
// not set. A blank value is a validation error; clearFlag names the flag that
// clears the field, which the error points to, or is "" when there is none.
func ReadNonEmpty(cmd *cobra.Command, name, clearFlag string) (*string, error) {
	value := ChangedString(cmd, name)
	if value != nil && isBlank(*value) {
		return nil, emptyValueError(cmd, name, clearFlag)
	}
	return value, nil
}

// ReadRequiredText returns the value of the required text flag name, which
// it never asks for, unlike ReadOrAsk. A missing flag is the validation error
// "--name is required", and a blank value is "--name cannot be empty".
func ReadRequiredText(cmd *cobra.Command, name string) (string, error) {
	value, err := ReadNonEmpty(cmd, name, "")
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", errors.NewValidationError(fmt.Sprintf("--%s is required", name), nil)
	}
	return *value, nil
}

// isBlank reports whether value is empty or only white space. A text flag
// that cannot be empty fails on a blank value as on an empty one, with
// emptyValueError; a secret, which is taken as given, fails only when empty.
func isBlank(value string) bool {
	return strings.TrimSpace(value) == ""
}

// ReadFilterTags returns the values of the repeatable flag --tag of a
// command that filters by tag. The API takes the tags joined with commas and
// splits them at each comma, so a tag that holds one, given in double
// quotes, is a validation error: the API would look for its parts instead.
func ReadFilterTags(cmd *cobra.Command) ([]string, error) {
	tags, err := ReadRepeatable(cmd, "tag", "")
	if err != nil {
		return nil, err
	}
	for _, tag := range tags {
		if strings.Contains(tag, ",") {
			return nil, validation.InvalidValueError("--"+TypedName(cmd, "tag"), tag,
				"a filter tag cannot hold a comma, at which the API splits the tags")
		}
	}
	return tags, nil
}

// JoinFilter returns values joined with commas, the form the API takes a
// filter of several values in, or nil when there are none.
func JoinFilter(values []string) *string {
	if len(values) == 0 {
		return nil
	}
	joined := strings.Join(values, ",")
	return &joined
}

// ReadClearable returns the value of the string flag name, or nil when it was
// not set, and whether the bool flag clearFlag, such as --clear-from, was
// given. A command without clearFlag, such as a create command, has nothing
// to clear. An empty value is a validation error, which points to clearFlag
// when the command has it.
func ReadClearable(cmd *cobra.Command, name, clearFlag string) (*string, bool, error) {
	if cmd.Flags().Lookup(clearFlag) == nil {
		value, err := ReadNonEmpty(cmd, name, "")
		return value, false, err
	}
	cleared, _ := cmd.Flags().GetBool(clearFlag)
	value, err := ReadNonEmpty(cmd, name, clearFlag)
	return value, cleared, err
}

// AddRepeatableFlag adds a flag that can be used multiple times. Like every
// repeatable flag of the CLI, it splits each value on commas, as a CSV line,
// so --tag a,b is --tag a --tag b. A value that holds a comma must be in
// double quotes, with any double quote in it doubled: --tag '"news, weekly"'.
// A value that is not such a CSV line is a validation error.
func AddRepeatableFlag(cmd *cobra.Command, name, usage string) {
	cmd.Flags().StringSlice(name, nil, usage)
	flag := cmd.Flags().Lookup(name)
	flag.Value = &repeatableValue{Value: flag.Value, name: name}
	// Help knows "[]" as the zero value of a list flag only by the value's
	// type, which the wrapper hides; "" is the zero value of every type.
	flag.DefValue = ""
}

// repeatableValue is the value of a repeatable flag, which reports a value
// it cannot split as a validation error rather than as the CSV reader's, and
// records which items a value split into several.
type repeatableValue struct {
	pflag.Value
	name string
	// split holds, for each item in order, whether it came from a value
	// that split into several items, such as both of --header "A: 1,B: 2".
	split []bool
}

func (v *repeatableValue) Set(value string) error {
	return v.setAs(v.name, value)
}

// setAs adds the items of value, given under name, the flag's name or a
// deprecated alias of it, which an error names.
func (v *repeatableValue) setAs(name, value string) error {
	if err := v.Value.Set(value); err != nil {
		return validation.InvalidValueError("--"+name, value,
			`a value with a comma or a double quote must be in double quotes, with each double quote in it doubled, such as '"a, ""b"""'`)
	}
	added := len(v.Value.(pflag.SliceValue).GetSlice()) - len(v.split)
	for range added {
		v.split = append(v.split, added > 1)
	}
	return nil
}

// fromSplitValue reports whether the item at index of the repeatable flag
// name came from a value that split into several items.
func fromSplitValue(cmd *cobra.Command, name string, index int) bool {
	value := cmd.Flags().Lookup(name).Value
	if target, ok := value.(*aliasTarget); ok {
		value = target.Value
	}
	repeatable, ok := value.(*repeatableValue)
	return ok && index < len(repeatable.split) && repeatable.split[index]
}

// ReadRepeatable returns the values of a repeatable flag, each without
// leading and trailing spaces, so --tag "news, weekly" gives news and weekly,
// or nil when the flag was not set, so the request field it fills stays nil.
// An empty value, such as the one after a trailing comma, is a validation
// error; clearFlag names the flag that clears the field, which the error
// points to, or is "" when there is none.
func ReadRepeatable(cmd *cobra.Command, name, clearFlag string) ([]string, error) {
	values, err := readRepeatableAsGiven(cmd, name, clearFlag)
	if err != nil || values == nil {
		return nil, err
	}
	trimmed := make([]string, len(values))
	for i, value := range values {
		trimmed[i] = strings.TrimSpace(value)
		if trimmed[i] == "" {
			return nil, emptyValueError(cmd, name, clearFlag)
		}
	}
	return trimmed, nil
}

// readRepeatableAsGiven is ReadRepeatable without the trimming.
func readRepeatableAsGiven(cmd *cobra.Command, name, clearFlag string) ([]string, error) {
	if !cmd.Flags().Changed(name) {
		return nil, nil
	}
	values, _ := cmd.Flags().GetStringSlice(name)
	if len(values) == 0 || slices.Contains(values, "") {
		return nil, emptyValueError(cmd, name, clearFlag)
	}
	return values, nil
}

// ReadListUpdate returns the list an update sends for the repeatable flag
// name and the bool flag clearFlag: the values, an empty list when clearFlag
// is set, or nil to keep the list unchanged.
func ReadListUpdate(cmd *cobra.Command, name, clearFlag string) (*[]string, error) {
	values, err := ReadRepeatable(cmd, name, clearFlag)
	if err != nil {
		return nil, err
	}
	if err := RejectTogether(cmd, name, clearFlag); err != nil {
		return nil, err
	}
	clearList, _ := cmd.Flags().GetBool(clearFlag)
	switch {
	case clearList:
		return &[]string{}, nil
	case values != nil:
		return &values, nil
	default:
		return nil, nil
	}
}

// emptyValueError is the validation error of an empty value of the flag
// name, which names the flag as the user gave it, such as under a deprecated
// alias, and points to clearFlag unless it is "".
func emptyValueError(cmd *cobra.Command, name, clearFlag string) error {
	message := fmt.Sprintf("--%s cannot be empty", TypedName(cmd, name))
	if clearFlag != "" {
		message += fmt.Sprintf("; use --%s to clear it", clearFlag)
	}
	return errors.NewValidationError(message, nil)
}

// RejectTogether returns a validation error when flags first and second are
// both set on the command line. The error names each flag as the user gave
// it, under a deprecated alias when that is the name used, in the order the
// user gave them.
func RejectTogether(cmd *cobra.Command, first, second string) error {
	if cmd.Flags().Changed(first) && cmd.Flags().Changed(second) {
		first, second = typedOrder(cmd, first, second)
		return errors.NewValidationError(fmt.Sprintf("--%s and --%s cannot be used together",
			TypedName(cmd, first), TypedName(cmd, second)), nil)
	}
	return nil
}

// typedOrder returns the set flags first and second in the order the user
// gave them, under any of their names.
func typedOrder(cmd *cobra.Command, first, second string) (string, string) {
	// Visit follows the order flags were set in when they are not sorted.
	flags := cmd.Flags()
	sorted := flags.SortFlags
	flags.SortFlags = false
	defer func() { flags.SortFlags = sorted }()
	var order []string
	flags.Visit(func(flag *pflag.Flag) { order = append(order, flag.Name) })
	if slices.Index(order, second) < slices.Index(order, first) {
		return second, first
	}
	return first, second
}

// MarkNoEffect hides the flag name and marks it deprecated, as a flag that is
// accepted for compatibility and has no effect. hint, when not empty, tells
// what to use instead.
func MarkNoEffect(cmd *cobra.Command, name, hint string) {
	cmd.Flags().Lookup(name).Usage = "No effect"
	message := "it has no effect"
	if hint != "" {
		message += "; " + hint
	}
	_ = cmd.Flags().MarkDeprecated(name, message)
}

// MaxNameLength is the most characters the API accepts in a name or a label.
const MaxNameLength = 255

// CheckMaxLength checks that value, the value of the flag name, has at most
// max characters, when there is a value.
func CheckMaxLength(name string, value *string, max int) error {
	if value != nil && utf8.RuneCountInString(*value) > max {
		return errors.NewValidationError(fmt.Sprintf("--%s must be at most %d characters", name, max), nil)
	}
	return nil
}

// CheckMaxCleanedLength is CheckMaxLength for a value the API cleans before
// it counts its characters: it counts value once clean has cleaned it as the
// API does, such as strings.TrimSpace for a name the API trims, or
// cleanTemplateText. The value is still sent as given, for the API to clean.
func CheckMaxCleanedLength(name string, value *string, max int, clean func(string) string) error {
	if value == nil {
		return nil
	}
	cleaned := clean(*value)
	return CheckMaxLength(name, &cleaned, max)
}

// The most characters the API accepts in the subject and the preheader
// (preview text) of a template or a campaign, once cleaned.
const (
	maxSubjectLength   = 998
	maxPreheaderLength = 500
)

// CheckSubjectAndPreheader checks that --subject and --preheader, when set,
// fit maxSubjectLength and maxPreheaderLength, counted as the API counts them,
// once cleanTemplateText cleaned them.
func CheckSubjectAndPreheader(cmd *cobra.Command) error {
	if err := CheckMaxCleanedLength("subject", ChangedString(cmd, "subject"),
		maxSubjectLength, cleanTemplateText); err != nil {
		return err
	}
	return CheckMaxCleanedLength("preheader", ChangedString(cmd, "preheader"),
		maxPreheaderLength, cleanTemplateText)
}

// cleanTemplateText cleans a subject or a preview text as the API does before
// it counts its characters: each CRLF line break, each control character
// other than a tab, and each line or paragraph separator (U+2028, U+2029)
// becomes one space, then the spaces around the text are trimmed.
func cleanTemplateText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.Map(func(r rune) rune {
		if r != '\t' && (unicode.IsControl(r) || r == '\u2028' || r == '\u2029') {
			return ' '
		}
		return r
	}, value)
	return strings.TrimSpace(value)
}
