package cmdutil

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The values of the enum flags that several command groups share, as the API
// defines them.
var (
	// Scopes are the scopes of a webhook or an SMTP credential.
	Scopes = []string{"global", "scoped"}
	// SandboxResults are the results a sandbox send simulates.
	SandboxResults = []string{"deliver", "bounce", "defer", "fail", "suppress"}
	// SubscriptionStatusFilters are the membership statuses a list of
	// memberships can be filtered by.
	SubscriptionStatusFilters = []string{"unconfirmed", "confirmed", "unsubscribed", "complained"}
)

// oneOfAnnotation holds the values of a flag added by AddOneOfFlag.
const oneOfAnnotation = "ahasend_one_of"

// AddOneOfFlag adds the string flag name, with the default value, whose
// value is one of values. Its help is usage with the values listed in place
// of its %s, such as "Sending type: transactional or marketing", and the
// shell completes the values. MatchOneOf, ReadOneOf and ReadFilterOneOf check
// a value of it against them.
func AddOneOfFlag(cmd *cobra.Command, name, value, usage string, values []string) {
	cmd.Flags().String(name, value, fmt.Sprintf(usage, listValues(values)))
	markOneOf(cmd, cmd.Flags(), name, values)
}

// AddPersistentOneOfFlag is AddOneOfFlag for a persistent flag, which the
// subcommands of cmd take too.
func AddPersistentOneOfFlag(cmd *cobra.Command, name, value, usage string, values []string) {
	cmd.PersistentFlags().String(name, value, fmt.Sprintf(usage, listValues(values)))
	markOneOf(cmd, cmd.PersistentFlags(), name, values)
}

// AddRepeatableOneOfFlag is AddOneOfFlag for a flag that can be given
// several times, as AddRepeatableFlag adds it; MatchOneOf checks each value.
func AddRepeatableOneOfFlag(cmd *cobra.Command, name, usage string, values []string) {
	AddRepeatableFlag(cmd, name, fmt.Sprintf(usage, listValues(values)))
	markOneOf(cmd, cmd.Flags(), name, values)
}

// markOneOf records values on the flag name of flags and makes the shell
// complete them.
func markOneOf(cmd *cobra.Command, flags *pflag.FlagSet, name string, values []string) {
	_ = flags.SetAnnotation(name, oneOfAnnotation, values)
	CompleteValues(cmd, name, values)
}

// listValues lists values for a help text: "a, b or c".
func listValues(values []string) string {
	if len(values) < 2 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], ", ") + " or " + values[len(values)-1]
}

// MatchOneOf returns the value of the flag name, added by AddOneOfFlag, that
// value matches ignoring case, so the request carries the API's spelling. A
// value that matches none is a validation error.
func MatchOneOf(cmd *cobra.Command, name, value string) (string, error) {
	values, err := oneOfValues(cmd, name)
	if err != nil {
		return "", err
	}
	for _, candidate := range values {
		if strings.EqualFold(value, candidate) {
			return candidate, nil
		}
	}
	return "", errors.NewValidationError(fmt.Sprintf("--%s must be one of: %s", name, strings.Join(values, ", ")), nil)
}

// oneOfValues returns the values of the flag name of cmd, added by
// AddOneOfFlag. A flag cmd does not have, or one added otherwise, is a
// mistake in the CLI, which exits 1, rather than a value no user could get
// right.
func oneOfValues(cmd *cobra.Command, name string) ([]string, error) {
	flag := cmd.Flag(name)
	if flag == nil || len(flag.Annotations[oneOfAnnotation]) == 0 {
		return nil, fmt.Errorf("internal error: %s has no --%s flag added by AddOneOfFlag", cmd.CommandPath(), name)
	}
	return flag.Annotations[oneOfAnnotation], nil
}

// ReadSandboxResult returns --sandbox-result in the API's spelling. A value
// that is not a sandbox result is a validation error, and so is
// --sandbox-result without --sandbox, which would send real mail.
func ReadSandboxResult(cmd *cobra.Command) (string, error) {
	value, _ := cmd.Flags().GetString("sandbox-result")
	result, err := MatchOneOf(cmd, "sandbox-result", value)
	if err != nil {
		return "", err
	}
	sandbox, _ := cmd.Flags().GetBool("sandbox")
	if cmd.Flags().Changed("sandbox-result") && !sandbox {
		return "", errors.NewValidationError("--sandbox-result needs --sandbox", nil)
	}
	return result, nil
}

// ReadOneOf returns the value of the flag name, added by AddOneOfFlag, in
// the spelling of its values, or nil when the flag was not set. An empty
// value, or one that is not one of its values, is a validation error.
func ReadOneOf(cmd *cobra.Command, name string) (*string, error) {
	if _, err := oneOfValues(cmd, name); err != nil {
		return nil, err
	}
	value, err := ReadNonEmpty(cmd, name, "")
	if err != nil || value == nil {
		return nil, err
	}
	match, err := MatchOneOf(cmd, name, *value)
	if err != nil {
		return nil, err
	}
	return &match, nil
}

// ReadFilterOneOf returns the value of the filter flag name, added by
// AddOneOfFlag, in the spelling of its values, or nil when it is empty, so
// an empty filter is left out of the request. A value that is not one of
// its values is a validation error.
func ReadFilterOneOf(cmd *cobra.Command, name string) (*string, error) {
	if _, err := oneOfValues(cmd, name); err != nil {
		return nil, err
	}
	value := NonEmptyString(cmd, name)
	if value == nil {
		return nil, nil
	}
	match, err := MatchOneOf(cmd, name, *value)
	if err != nil {
		return nil, err
	}
	return &match, nil
}

// CheckScopeDomains checks the --scope and --domains of a new webhook or SMTP
// credential: a scoped one needs domains, and a global one takes none, since
// the API would ignore them and leave it unrestricted.
func CheckScopeDomains(scope string, domains []string) error {
	switch {
	case scope == "scoped" && domains == nil:
		return errors.NewValidationError("--domains is required when --scope is scoped", nil)
	case scope != "scoped" && domains != nil:
		return errors.NewValidationError("--domains needs --scope scoped", nil)
	}
	return nil
}
