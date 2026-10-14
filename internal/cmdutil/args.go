package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// The positional argument checks of the commands. A missing or an extra
// argument is a validation error, which prints no usage text, like the other
// validation errors. A missing argument is named from the command's usage
// line, such as "the domain argument is required" for "get <domain>".

// NoArgs accepts no argument.
func NoArgs(cmd *cobra.Command, args []string) error {
	return checkArgs(cmd, args, 0, 0)
}

// ExactArgs accepts exactly n arguments.
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return checkArgs(cmd, args, n, n)
	}
}

// MinimumNArgs accepts n or more arguments.
func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return checkArgs(cmd, args, n, -1)
	}
}

// MaximumNArgs accepts up to n arguments.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		return checkArgs(cmd, args, 0, n)
	}
}

// checkArgs accepts min to max arguments, or min or more when max is
// negative.
func checkArgs(cmd *cobra.Command, args []string, min, max int) error {
	if len(args) < min {
		names := UsageArguments(cmd)
		if len(args) < len(names) {
			return errors.NewValidationError(fmt.Sprintf("the %s argument is required", names[len(args)]), nil)
		}
		return errors.NewValidationError(fmt.Sprintf("%d arguments are required", min), nil)
	}
	if max >= 0 && len(args) > max {
		return errors.NewValidationError(fmt.Sprintf("unexpected argument %q", args[max]), nil)
	}
	return nil
}

// UsageArguments returns the names of the arguments the usage line of cmd
// names, such as sub-account-id and key-id for
// "get <sub-account-id> <key-id>", and domain for "create [domain]". Flag
// values, such as <label> in "--label <label>", are not arguments.
func UsageArguments(cmd *cobra.Command) []string {
	fields := strings.Fields(cmd.Use)
	var names []string
	for i := 1; i < len(fields); i++ {
		switch {
		case strings.HasPrefix(fields[i], "--"):
			i++ // skip the flag's value
		case strings.HasPrefix(fields[i], "<"), strings.HasPrefix(fields[i], "["):
			names = append(names, strings.Trim(fields[i], "<>[]."))
		}
	}
	return names
}
