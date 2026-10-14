package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachRootGetsItsOwnPingCommand(t *testing.T) {
	first := NewRootCmdForTesting()
	second := NewRootCmdForTesting()
	firstPing, _, err := first.Find([]string{"ping"})
	require.NoError(t, err)
	secondPing, _, err := second.Find([]string{"ping"})
	require.NoError(t, err)

	assert.NotSame(t, firstPing, secondPing)
	assert.Same(t, first, firstPing.Parent())
}

// assertLongLinesFit checks that every line of the command's Long help is
// under 80 columns.
func assertLongLinesFit(t *testing.T, command *cobra.Command) {
	t.Helper()
	for _, line := range strings.Split(command.Long, "\n") {
		assert.Less(t, len(line), 80, "%s: %q", command.CommandPath(), line)
	}
}

func TestSubaccountsUnsuspendLongLinesFit(t *testing.T) {
	command, _, err := NewRootCmdForTesting().Find([]string{"subaccounts", "unsuspend"})
	require.NoError(t, err)
	assertLongLinesFit(t, command)
}

func TestRequiredFlagsSayTheyAreRequired(t *testing.T) {
	flags := map[string][]string{
		"webhooks create":     {"name", "url"},
		"routes create":       {"name", "url", "recipient"},
		"smtp create":         {"name"},
		"subaccounts create":  {"name", "website"},
		"suppressions create": {"expires"},
	}
	for path, names := range flags {
		t.Run(path, func(t *testing.T) {
			command, _, err := NewRootCmdForTesting().Find(strings.Fields(path))
			require.NoError(t, err)
			for _, name := range names {
				usage := command.Flags().Lookup(name).Usage
				assert.True(t, strings.HasSuffix(usage, "(required)"), "--%s: %s", name, usage)
				assert.Contains(t, command.Use, "--"+name+" <", "the Use line shows --%s", name)
			}
		})
	}
}

func TestRequiredRepeatableFlagsHaveOneCombinedMarker(t *testing.T) {
	flags := map[string]string{
		"apikeys create":              "scope",
		"subaccounts api-keys create": "scope",
	}
	for path, name := range flags {
		t.Run(path, func(t *testing.T) {
			command, _, err := NewRootCmdForTesting().Find(strings.Fields(path))
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(command.Flags().Lookup(name).Usage, "(required; can be used multiple times)"))
		})
	}
}

func TestCommandGroupsFollowHelpConventions(t *testing.T) {
	walkCommands(NewRootCmdForTesting(), func(command *cobra.Command) {
		path := command.CommandPath()
		// Cobra writes the help of its help and completion commands
		if path == "ahasend help" || strings.HasPrefix(path, "ahasend completion") {
			return
		}
		if flag := command.Flags().Lookup("force"); flag != nil {
			assert.Equal(t, "Skip confirmation prompt", flag.Usage, path)
			assert.Contains(t, strings.Join(strings.Fields(command.Long), " "),
				"asks for confirmation unless --force is set", path)
		}
		command.Flags().VisitAll(func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			if flag.Value.Type() == "stringSlice" {
				suffix := "(can be used multiple times)"
				if strings.Contains(flag.Usage, "(required") {
					suffix = "(required; can be used multiple times)"
				}
				assert.True(t, strings.HasSuffix(flag.Usage, suffix), "%s --%s", path, flag.Name)
			}
			if command.Name() == "list" && flag.Value.Type() == "string" &&
				flag.Name != "after" && flag.Name != "before" {
				assert.True(t, strings.HasPrefix(flag.Usage, "Filter"), "%s --%s", path, flag.Name)
			}
		})
		assert.LessOrEqual(t, len(command.Short), 60, "%s: Short is too long for the command list", path)
		assertLongLinesFit(t, command)
		assertExamplesHaveCaptions(t, command)
		assertNoExamplesInLong(t, command)
	})
}

// assertNoExamplesInLong checks that the Long help of command has no
// Examples heading of its own: examples go in Example, which Cobra shows
// under its own heading, so help shows one Examples section.
func assertNoExamplesInLong(t *testing.T, command *cobra.Command) {
	t.Helper()
	for _, line := range strings.Split(command.Long, "\n") {
		heading := strings.ToLower(strings.TrimSpace(line))
		assert.False(t, heading == "examples:" || heading == "example:",
			"%s: Long has an Examples heading; put the examples in Example", command.CommandPath())
	}
}

// TestJSONExamplesDoNotPrompt checks that an example with JSON output skips
// the confirmation prompt, so it prints the API response alone.
func TestJSONExamplesDoNotPrompt(t *testing.T) {
	walkCommands(NewRootCmdForTesting(), func(command *cobra.Command) {
		if command.Flags().Lookup("force") == nil {
			return
		}
		for _, line := range strings.Split(command.Example, "\n") {
			if strings.Contains(line, "--output json") && strings.HasPrefix(strings.TrimSpace(line), command.CommandPath()+" ") {
				assert.Contains(t, line, "--force", "%s: %q", command.CommandPath(), line)
			}
		}
	})
}

// assertExamplesHaveCaptions checks that every command line of the command's
// examples follows a "# caption" line.
func assertExamplesHaveCaptions(t *testing.T, command *cobra.Command) {
	t.Helper()
	lines := strings.Split(command.Example, "\n")
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "ahasend ") {
			continue
		}
		previous := ""
		if i > 0 {
			previous = strings.TrimSpace(lines[i-1])
		}
		assert.True(t, strings.HasPrefix(previous, "# "),
			"%s: example %q has no caption", command.CommandPath(), line)
	}
}
