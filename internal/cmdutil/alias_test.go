package cmdutil

import (
	"bytes"
	stderrors "errors"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addAliasFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("enabled", false, "")
	AddNegatedFlagAlias(cmd, "disabled", "enabled")
	AddRepeatableFlag(cmd, "tag", "")
	AddFlagAlias(cmd, "tags", "tag")
	cmd.Flags().String("recipient", "", "")
}

func TestFlagAliasSetsTheNewFlag(t *testing.T) {
	cmd := parsed(t, addAliasFlags, "--tags", "a", "--tags", "b,c")

	values, err := ReadRepeatable(cmd, "tag", "")
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, values)
}

func TestFlagAndItsAliasCannotBeUsedTogether(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addAliasFlags(cmd)

	err := cmd.ParseFlags([]string{"--tag", "a", "--tags", "b"})

	assertValidationError(t, stderrors.Unwrap(err), "--tag and --tags cannot be used together")
}

func TestRejectTogetherNamesTheAliasTheUserGave(t *testing.T) {
	cmd := parsed(t, addAliasFlags, "--tags", "a", "--disabled")

	err := RejectTogether(cmd, "tag", "enabled")

	assertValidationError(t, err, "--tags and --disabled cannot be used together")
}

func TestRepeatableValueErrorsNameTheAliasTheUserGave(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	addAliasFlags(cmd)

	err := cmd.ParseFlags([]string{"--tags", `"a`})

	assertValidationError(t, err, `invalid --tags value "\"a": a value with a comma or a double quote must be in double quotes`)
}

func TestFilterTagErrorsNameTheAliasTheUserGave(t *testing.T) {
	_, err := ReadFilterTags(parsed(t, addAliasFlags, "--tags", `"a,b"`))

	assertValidationError(t, err, `invalid --tags value "a,b": a filter tag cannot hold a comma`)
}

func TestNegatedFlagAliasSetsTheOppositeValue(t *testing.T) {
	assert.Equal(t, false, *ChangedBool(parsed(t, addAliasFlags, "--disabled"), "enabled"))
}

// TestBoolAliasesTakeNoValue checks that every kind of deprecated switch
// refuses a value, and names what to give in its place for the value given.
func TestBoolAliasesTakeNoValue(t *testing.T) {
	addSwitches := func(cmd *cobra.Command) {
		cmd.Flags().Bool("enabled", false, "")
		AddFlagAlias(cmd, "enable", "enabled")
		AddNegatedFlagAlias(cmd, "disable", "enabled")
		cmd.Flags().String("scope", "", "")
		AddValueAlias(cmd, "clear-domains", "scope", "global")
		prompt.AddNonInteractiveFlag(cmd)
		AddInteractiveAlias(cmd)
	}
	tests := []struct {
		arg     string
		message string
	}{
		{"--enable=false", "--enable takes no value; leave it out, or give --enabled=false"},
		{"--enable=true", "--enable takes no value; give --enabled"},
		{"--enable=maybe", "--enable takes no value; give --enabled"},
		{"--disable=false", "--disable takes no value; leave it out, or give --enabled"},
		{"--disable=true", "--disable takes no value; give --enabled=false"},
		{"--clear-domains=false", "--clear-domains takes no value; leave it out"},
		{"--clear-domains=true", "--clear-domains takes no value; give --scope global"},
		{"--interactive=false", "--interactive takes no value; leave it out, as prompting is the default, or give --non-interactive to turn prompting off"},
		{"--interactive=true", "--interactive takes no value; leave it out, as prompting is the default"},
	}
	for _, test := range tests {
		t.Run(test.arg, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			addSwitches(cmd)

			err := cmd.ParseFlags([]string{test.arg})

			assertValidationError(t, stderrors.Unwrap(err), test.message)
			assert.False(t, cmd.Flags().Changed("enabled"))
		})
	}
	for arg, flag := range map[string]string{"--enable": "enabled", "--disable": "enabled", "--clear-domains": "scope", "--interactive": "non-interactive"} {
		assert.True(t, parsed(t, addSwitches, arg).Flags().Changed(flag), arg)
	}
}

func TestAliasesAreHiddenAndDeprecated(t *testing.T) {
	cmd := parsed(t, addAliasFlags)
	for _, name := range []string{"disabled", "tags"} {
		flag := cmd.Flags().Lookup(name)
		assert.True(t, flag.Hidden, name)
		assert.NotEmpty(t, flag.Deprecated, name)
	}
}

func TestAliasNoticesNameTheReplacement(t *testing.T) {
	cmd := parsed(t, func(cmd *cobra.Command) {
		addAliasFlags(cmd)
		addRequiredFlags(cmd)
	})
	notices := map[string]string{
		"tags":        "use --tag instead",
		"disabled":    "use --enabled=false instead",
		"interactive": "prompting is the default; use --non-interactive to turn it off",
	}
	for name, notice := range notices {
		assert.Equal(t, notice, cmd.Flags().Lookup(name).Deprecated, name)
	}
}

func addRequiredFlags(cmd *cobra.Command) {
	cmd.Flags().String("name", "", "")
	prompt.AddNonInteractiveFlag(cmd)
	AddInteractiveAlias(cmd)
}

// promptedCommand returns a command parsed from args whose stdin holds input.
func promptedCommand(t *testing.T, input string, args ...string) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	cmd := parsed(t, addRequiredFlags, args...)
	var stderr bytes.Buffer
	cmd.SetIn(bytes.NewBufferString(input))
	cmd.SetErr(&stderr)
	return cmd, &stderr
}

func TestReadOrAskPromptsOnATerminal(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	cmd, stderr := promptedCommand(t, "Events\n")

	value, err := ReadOrAsk(cmd, "name", "Name")

	require.NoError(t, err)
	assert.Equal(t, "Events", value)
	assert.Equal(t, "Name: ", stderr.String())
}

func TestReadOrAskDoesNotPromptWithoutATerminal(t *testing.T) {
	cmd, stderr := promptedCommand(t, "Events\n")

	_, err := ReadOrAsk(cmd, "name", "Name")

	assertValidationError(t, err, "--name is required")
	assert.Empty(t, stderr.String())
}

func TestReadOrAskDoesNotPromptWhenNonInteractive(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	cmd, stderr := promptedCommand(t, "Events\n", "--non-interactive")

	_, err := ReadOrAsk(cmd, "name", "Name")

	assertValidationError(t, err, "--name is required")
	assert.Empty(t, stderr.String())
}

func TestReadOrAskUsesTheFlagWithoutPrompting(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	cmd, stderr := promptedCommand(t, "Other\n", "--name", "Events")

	value, err := ReadOrAsk(cmd, "name", "Name")

	require.NoError(t, err)
	assert.Equal(t, "Events", value)
	assert.Empty(t, stderr.String())
}

// TestFlagConflictsNameFlagsInTypedOrderAfterTheirNotices checks one rule for
// every pair of conflicting names: the error names them in the order given,
// and each deprecated name given gets its notice, whether the names are one
// flag's or two flags'.
func TestFlagConflictsNameFlagsInTypedOrderAfterTheirNotices(t *testing.T) {
	const tagsNotice = "Flag --tags has been deprecated, use --tag instead\n"
	tests := []struct {
		args   []string
		err    string
		notice string
	}{
		{[]string{"--tag", "a", "--tags", "b"}, "--tag and --tags cannot be used together", tagsNotice},
		{[]string{"--tags", "b", "--tag", "a"}, "--tags and --tag cannot be used together", tagsNotice},
		{[]string{"--enabled", "--disabled"}, "--enabled and --disabled cannot be used together",
			"Flag --disabled has been deprecated, use --enabled=false instead\n"},
		{[]string{"--recipient", "x", "--tags", "a"}, "--recipient and --tags cannot be used together", tagsNotice},
		{[]string{"--tags", "a", "--recipient", "x"}, "--tags and --recipient cannot be used together", tagsNotice},
		{[]string{"--recipient", "x", "--tag", "a"}, "--recipient and --tag cannot be used together", ""},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			cmd := &cobra.Command{Use: "test", SilenceErrors: true, SilenceUsage: true,
				RunE: func(cmd *cobra.Command, _ []string) error { return RejectTogether(cmd, "tag", "recipient") }}
			addAliasFlags(cmd)
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)
			cmd.SetOut(&stderr)
			cmd.SetArgs(test.args)

			err := cmd.Execute()

			require.Error(t, err)
			assert.Contains(t, err.Error(), test.err)
			assert.Equal(t, test.notice, stderr.String())
		})
	}
}
