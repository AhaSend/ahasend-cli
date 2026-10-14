package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEveryCommandChecksItsArguments fails when a runnable command has no Args
// check, which lets it accept stray positional arguments silently, or prints
// its usage on errors when run directly.
func TestEveryCommandChecksItsArguments(t *testing.T) {
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		if cmd.Runnable() && cmd.Name() != "help" {
			assert.NotNil(t, cmd.Args, "%s has no Args check", cmd.CommandPath())
			assert.True(t, cmd.SilenceUsage, "%s does not set SilenceUsage", cmd.CommandPath())
		}
	})
}

// shadowedGlobalFlags are the global flags a command defines on purpose:
// auth login saves the --api-key and --account-id it is given under
// --profile, rather than authenticating with them, and auth status shows the
// profile --profile names.
var shadowedGlobalFlags = map[string][]string{
	"ahasend auth login":  {"account-id", "api-key", "profile"},
	"ahasend auth status": {"profile"},
}

// TestNoCommandShadowsAGlobalFlag checks that no command defines a flag, or
// a shorthand, that a global flag of the root command uses, other than
// shadowedGlobalFlags: a local --verbose, say, would take the place of the
// global one in that command.
func TestNoCommandShadowsAGlobalFlag(t *testing.T) {
	root := NewRootCmdForTesting()
	byShorthand := map[string]*pflag.Flag{}
	root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Shorthand != "" {
			byShorthand[flag.Shorthand] = flag
		}
	})
	walkCommands(root, func(cmd *cobra.Command) {
		if cmd == root {
			return
		}
		check := func(flag *pflag.Flag) {
			shadowed := slices.Contains(shadowedGlobalFlags[cmd.CommandPath()], flag.Name)
			if global := root.PersistentFlags().Lookup(flag.Name); global != nil && global != flag && !shadowed {
				t.Errorf("%s defines --%s, a global flag", cmd.CommandPath(), flag.Name)
			}
			if global := byShorthand[flag.Shorthand]; global != nil && global != flag {
				t.Errorf("%s defines -%s, the shorthand of the global --%s", cmd.CommandPath(), flag.Shorthand, global.Name)
			}
		}
		cmd.Flags().VisitAll(check)
		cmd.PersistentFlags().VisitAll(check)
	})
}

// TestTheCommandTree lists every command of the CLI, so a command added,
// removed or moved to another group fails the test, and checks that each
// one has the help text the help and the command list show: a short and a
// long description, and examples on each command that does the work, other
// than the help and completion commands Cobra writes.
func TestTheCommandTree(t *testing.T) {
	want := []string{
		"ahasend",
		"ahasend apikeys",
		"ahasend apikeys create",
		"ahasend apikeys delete",
		"ahasend apikeys get",
		"ahasend apikeys list",
		"ahasend apikeys update",
		"ahasend auth",
		"ahasend auth login",
		"ahasend auth logout",
		"ahasend auth status",
		"ahasend auth switch",
		"ahasend completion",
		"ahasend completion bash",
		"ahasend completion fish",
		"ahasend completion powershell",
		"ahasend completion zsh",
		"ahasend contacts",
		"ahasend contacts batch",
		"ahasend contacts create",
		"ahasend contacts delete",
		"ahasend contacts get",
		"ahasend contacts list",
		"ahasend contacts memberships",
		"ahasend contacts update",
		"ahasend domains",
		"ahasend domains check-dns",
		"ahasend domains create",
		"ahasend domains delete",
		"ahasend domains edit",
		"ahasend domains get",
		"ahasend domains list",
		"ahasend domains update",
		"ahasend domains verify",
		"ahasend help",
		"ahasend lists",
		"ahasend lists create",
		"ahasend lists delete",
		"ahasend lists get",
		"ahasend lists list",
		"ahasend lists members",
		"ahasend lists members add",
		"ahasend lists members list",
		"ahasend lists members remove",
		"ahasend lists members set",
		"ahasend lists update",
		"ahasend messages",
		"ahasend messages cancel",
		"ahasend messages get",
		"ahasend messages list",
		"ahasend messages send",
		"ahasend ping",
		"ahasend routes",
		"ahasend routes create",
		"ahasend routes delete",
		"ahasend routes get",
		"ahasend routes list",
		"ahasend routes listen",
		"ahasend routes trigger",
		"ahasend routes update",
		"ahasend smtp",
		"ahasend smtp create",
		"ahasend smtp delete",
		"ahasend smtp get",
		"ahasend smtp list",
		"ahasend smtp send",
		"ahasend stats",
		"ahasend stats bounces",
		"ahasend stats deliverability",
		"ahasend stats delivery-time",
		"ahasend subaccounts",
		"ahasend subaccounts api-keys",
		"ahasend subaccounts api-keys create",
		"ahasend subaccounts api-keys delete",
		"ahasend subaccounts api-keys get",
		"ahasend subaccounts api-keys list",
		"ahasend subaccounts api-keys update",
		"ahasend subaccounts create",
		"ahasend subaccounts delete",
		"ahasend subaccounts get",
		"ahasend subaccounts list",
		"ahasend subaccounts suspend",
		"ahasend subaccounts unpause-domain",
		"ahasend subaccounts unsuspend",
		"ahasend subaccounts update",
		"ahasend subaccounts usage",
		"ahasend suppressions",
		"ahasend suppressions check",
		"ahasend suppressions create",
		"ahasend suppressions delete",
		"ahasend suppressions list",
		"ahasend suppressions wipe",
		"ahasend templates",
		"ahasend templates create",
		"ahasend templates delete",
		"ahasend templates draft",
		"ahasend templates draft discard",
		"ahasend templates draft get",
		"ahasend templates get",
		"ahasend templates list",
		"ahasend templates publish",
		"ahasend templates update",
		"ahasend templates versions",
		"ahasend templates versions get",
		"ahasend templates versions list",
		"ahasend templates versions restore",
		"ahasend webhooks",
		"ahasend webhooks create",
		"ahasend webhooks delete",
		"ahasend webhooks get",
		"ahasend webhooks list",
		"ahasend webhooks listen",
		"ahasend webhooks trigger",
		"ahasend webhooks update",
	}

	var got []string
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		path := cmd.CommandPath()
		got = append(got, path)
		assert.NotEmpty(t, cmd.Short, "%s has no short description", path)
		assert.NotEmpty(t, cmd.Long, "%s has no long description", path)
		cobraCommand := path == "ahasend help" || strings.HasPrefix(path, "ahasend completion")
		if !cmd.HasSubCommands() && !cobraCommand {
			assert.NotEmpty(t, cmd.Example, "%s has no examples", path)
		}
	})

	assert.Equal(t, want, got)
}

// walkCommands calls visit for command and every command below it.
func walkCommands(command *cobra.Command, visit func(*cobra.Command)) {
	visit(command)
	for _, child := range command.Commands() {
		walkCommands(child, visit)
	}
}

// variadicCommands take any number of arguments, so no argument is stray.
var variadicCommands = map[string]bool{
	"ahasend messages cancel": true,
}

// leafCommands returns every runnable command of the tree but help.
func leafCommands() []*cobra.Command {
	var leaves []*cobra.Command
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		if cmd.Runnable() && cmd.Name() != "help" && !cmd.HasSubCommands() {
			leaves = append(leaves, cmd)
		}
	})
	return leaves
}

// TestEveryCommandNamesAMissingArgument runs every leaf command whose usage
// line names an argument without it, and checks that it fails naming the
// argument, without the usage text, before it authenticates.
func TestEveryCommandNamesAMissingArgument(t *testing.T) {
	for _, leaf := range leafCommands() {
		path := leaf.CommandPath()
		fields := strings.Fields(leaf.Use)
		if len(cmdutil.UsageArguments(leaf)) == 0 || !strings.HasPrefix(fields[1], "<") {
			continue
		}
		t.Run(path, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			args := strings.Fields(strings.TrimPrefix(path, "ahasend "))
			if leaf.Flags().Lookup("non-interactive") != nil {
				args = append(args, "--non-interactive")
			}
			stdout, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: the "+strings.Trim(fields[1], "<>.")+" argument is required\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestEveryCommandRejectsAStrayArgument runs every leaf command with the
// arguments its usage line names plus one, and checks that it fails on the
// arguments before it authenticates, without the usage text.
func TestEveryCommandRejectsAStrayArgument(t *testing.T) {
	for _, leaf := range leafCommands() {
		path := leaf.CommandPath()
		if variadicCommands[path] {
			continue
		}
		t.Run(path, func(t *testing.T) {
			ids := slices.Repeat([]string{rawTestID}, len(cmdutil.UsageArguments(leaf)))
			require.NoError(t, leaf.ValidateArgs(ids), "%s rejects the arguments its usage names", path)
			called := testutil.RefuseAuth(t)

			args := append(strings.Fields(strings.TrimPrefix(path, "ahasend ")), ids...)
			stdout, stderr, code := runRoot(t, append(args, "stray")...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: unexpected argument \"stray\"\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestDeprecatedFlagNoticesNameTheReplacement lists every deprecated flag
// with its notice, so each notice gives the right advice: a negated alias
// names the flag set to false, and --interactive says prompting is the
// default. A deprecated flag missing from the list fails the test, and so
// does --interactive on a command that never had it.
func TestDeprecatedFlagNoticesNameTheReplacement(t *testing.T) {
	want := map[string]string{
		"ahasend apikeys list --cursor":                  "use --after instead",
		"ahasend domains list --cursor":                  "use --after instead",
		"ahasend messages list --cursor":                 "use --after instead",
		"ahasend routes list --cursor":                   "use --after instead",
		"ahasend smtp list --cursor":                     "use --after instead",
		"ahasend subaccounts api-keys list --cursor":     "use --after instead",
		"ahasend subaccounts list --cursor":              "use --after instead",
		"ahasend suppressions list --cursor":             "use --after instead",
		"ahasend webhooks list --cursor":                 "use --after instead",
		"ahasend messages send --tags":                   "use --tag instead",
		"ahasend messages send --text-template":          "use --text-file instead",
		"ahasend messages send --html-template":          "use --html-file instead",
		"ahasend messages send --amp-template":           "use --amp-file instead",
		"ahasend messages list --tags":                   "use --tag instead",
		"ahasend smtp send --credential-id":              "it has no effect; use --username and --password",
		"ahasend stats bounces --classification":         "it has no effect",
		"ahasend stats bounces --raw":                    "it has no effect",
		"ahasend stats bounces --show-domains":           "it has no effect",
		"ahasend stats bounces --show-totals":            "it has no effect",
		"ahasend stats bounces --trends":                 "it has no effect",
		"ahasend stats deliverability --chart":           "it has no effect",
		"ahasend stats deliverability --raw":             "it has no effect",
		"ahasend stats deliverability --show-totals":     "it has no effect",
		"ahasend stats delivery-time --raw":              "it has no effect",
		"ahasend stats delivery-time --show-totals":      "it has no effect",
		"ahasend smtp send --tags":                       "use --tag instead",
		"ahasend stats bounces --tags":                   "use --tag instead",
		"ahasend stats deliverability --tags":            "use --tag instead",
		"ahasend stats delivery-time --tags":             "use --tag instead",
		"ahasend webhooks update --clear-domains":        "use --scope global instead",
		"ahasend webhooks create --disabled":             "use --enabled=false instead",
		"ahasend webhooks create --interactive":          "prompting is the default; use --non-interactive to turn it off",
		"ahasend webhooks update --enable":               "use --enabled instead",
		"ahasend webhooks update --disable":              "use --enabled=false instead",
		"ahasend webhooks update --no-events":            "use --clear-events instead",
		"ahasend routes create --interactive":            "prompting is the default; use --non-interactive to turn it off",
		"ahasend routes update --disabled":               "use --enabled=false instead",
		"ahasend routes update --no-include-attachments": "use --include-attachments=false instead",
		"ahasend routes update --no-include-headers":     "use --include-headers=false instead",
		"ahasend routes update --no-group-by-message-id": "use --group-by-message-id=false instead",
		"ahasend routes update --no-strip-replies":       "use --strip-replies=false instead",
	}

	got := map[string]string{}
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			if flag.Deprecated != "" {
				got[cmd.CommandPath()+" --"+flag.Name] = flag.Deprecated
			}
		})
	})

	assert.Equal(t, want, got)
}
