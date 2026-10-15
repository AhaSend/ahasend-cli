package cmdutil

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/AhaSend/ahasend-cli/internal/config"
)

// CompleteValues makes the shell complete the flag name of cmd with values,
// the values the flag's validation accepts.
func CompleteValues(cmd *cobra.Command, name string, values []string) {
	_ = cmd.RegisterFlagCompletionFunc(name, cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp))
}

// CompleteProfileNames completes the names of the profiles in the
// configuration file. It reads the file only: it makes no request, and
// completes nothing, silently, when there is no file.
func CompleteProfileNames(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return config.ProfileNames(), cobra.ShellCompDirectiveNoFileComp
}

// CompleteProfileArgument completes the profile argument of a command that
// takes one profile name.
func CompleteProfileArgument(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return CompleteProfileNames(cmd, args, toComplete)
}

// MarkFileFlag marks the flag name of cmd as taking a file path, which the
// shell completes; CompleteNoFiles leaves it so.
func MarkFileFlag(cmd *cobra.Command, name string) {
	_ = cmd.MarkFlagFilename(name)
}

// CompleteNoFiles makes the shell complete no file name where cmd and its
// subcommands take none: for every argument without completions of its own,
// since no command takes a file as an argument, and for every flag that has
// no completions of its own and is not marked with MarkFileFlag, nor a
// deprecated name of such a flag. A flag without either would complete file
// names, Cobra's default.
func CompleteNoFiles(cmd *cobra.Command) {
	if cmd.ValidArgsFunction == nil && len(cmd.ValidArgs) == 0 {
		cmd.ValidArgsFunction = cobra.NoFileCompletions
	}
	// The flags cmd defines, without merging in its parents' persistent
	// flags as LocalFlags would
	noFiles := func(flag *pflag.Flag) {
		if _, ok := cmd.GetFlagCompletionFunc(flag.Name); ok || isFileFlag(cmd, flag) {
			return
		}
		_ = cmd.RegisterFlagCompletionFunc(flag.Name, cobra.NoFileCompletions)
	}
	cmd.Flags().VisitAll(noFiles)
	cmd.PersistentFlags().VisitAll(noFiles)
	for _, sub := range cmd.Commands() {
		CompleteNoFiles(sub)
	}
}

// isFileFlag reports whether flag of cmd takes a file path: it is marked with
// MarkFileFlag, or is a deprecated name of a flag that is.
func isFileFlag(cmd *cobra.Command, flag *pflag.Flag) bool {
	if alias, ok := flag.Value.(*aliasValue); ok {
		flag = cmd.Flags().Lookup(alias.name)
	}
	_, marked := flag.Annotations[cobra.BashCompFilenameExt]
	return marked
}
