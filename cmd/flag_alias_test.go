package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// aliasTargetPattern finds the flag a deprecated alias's notice names.
var aliasTargetPattern = regexp.MustCompile(`^(?:prompting is the default; )?use --([a-z-]+)`)

// flagAlias is a deprecated alias of a command's flag.
type flagAlias struct {
	command       *cobra.Command
	alias, target string
}

// flagAliases returns every flag alias of the command tree.
func flagAliases() []flagAlias {
	var aliases []flagAlias
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
			if match := aliasTargetPattern.FindStringSubmatch(flag.Deprecated); match != nil {
				aliases = append(aliases, flagAlias{cmd, flag.Name, match[1]})
			}
		})
	})
	return aliases
}

// TestNoFlagShowsAnEmptyListAsItsDefault checks the help of every command:
// a list flag without a default, aliased or not, shows no "(default [])".
func TestNoFlagShowsAnEmptyListAsItsDefault(t *testing.T) {
	walkCommands(NewRootCmdForTesting(), func(cmd *cobra.Command) {
		assert.NotContains(t, cmd.Flags().FlagUsages(), "(default [])", cmd.CommandPath())
	})
}

// flagArgs returns the arguments that give the flag name of cmd: the name
// alone for a bool flag, or the name and a value.
func flagArgs(cmd *cobra.Command, name string) []string {
	if cmd.Flags().Lookup(name).NoOptDefVal != "" {
		return []string{"--" + name}
	}
	return []string{"--" + name, "x"}
}

// TestAFlagAndItsAliasCannotBeUsedTogether gives every deprecated alias
// together with the flag it stands for, in both orders, and every two
// aliases of one flag, and checks each is a validation error naming them in
// the order given, after the deprecation notice of each alias given, before
// the command authenticates.
func TestAFlagAndItsAliasCannotBeUsedTogether(t *testing.T) {
	aliases := flagAliases()
	require.NotEmpty(t, aliases)

	type pair struct {
		command       *cobra.Command
		first, second string
	}
	var pairs []pair
	for i, alias := range aliases {
		pairs = append(pairs, pair{alias.command, alias.alias, alias.target}, pair{alias.command, alias.target, alias.alias})
		for _, other := range aliases[i+1:] {
			if other.command == alias.command && other.target == alias.target {
				pairs = append(pairs, pair{alias.command, alias.alias, other.alias})
			}
		}
	}
	for _, p := range pairs {
		path := p.command.CommandPath()
		t.Run(path+" --"+p.first+" --"+p.second, func(t *testing.T) {
			called := testutil.RefuseAuth(t)
			args := strings.Fields(strings.TrimPrefix(path, "ahasend "))
			args = append(append(args, flagArgs(p.command, p.first)...), flagArgs(p.command, p.second)...)

			stdout, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			var notices string
			for _, name := range []string{p.first, p.second} {
				if deprecated := p.command.Flags().Lookup(name).Deprecated; deprecated != "" {
					notices += "Flag --" + name + " has been deprecated, " + deprecated + "\n"
				}
			}
			assert.Equal(t, notices+"Error: --"+p.first+" and --"+p.second+" cannot be used together\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestConflictErrorsNameTheAliasTheUserGave checks that a conflict between a
// flag and a deprecated alias of another flag names the alias.
func TestConflictErrorsNameTheAliasTheUserGave(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, stderr, code := runRoot(t, append(sendArgs("--text", "Hi"), "--text-template", "body.txt")...)

	assert.Equal(t, 4, code)
	assert.Equal(t, "Error: --text and --text-template cannot be used together\n", stderr)
	assert.False(t, *called)
}
