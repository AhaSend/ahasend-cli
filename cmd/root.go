package cmd

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"os"

	"github.com/AhaSend/ahasend-cli/cmd/groups/apikeys"
	"github.com/AhaSend/ahasend-cli/cmd/groups/auth"
	"github.com/AhaSend/ahasend-cli/cmd/groups/contacts"
	"github.com/AhaSend/ahasend-cli/cmd/groups/domains"
	"github.com/AhaSend/ahasend-cli/cmd/groups/lists"
	"github.com/AhaSend/ahasend-cli/cmd/groups/messages"
	"github.com/AhaSend/ahasend-cli/cmd/groups/routes"
	"github.com/AhaSend/ahasend-cli/cmd/groups/smtp"
	"github.com/AhaSend/ahasend-cli/cmd/groups/stats"
	"github.com/AhaSend/ahasend-cli/cmd/groups/subaccounts"
	"github.com/AhaSend/ahasend-cli/cmd/groups/suppressions"
	"github.com/AhaSend/ahasend-cli/cmd/groups/templates"
	"github.com/AhaSend/ahasend-cli/cmd/groups/webhooks"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/terminal"
	"github.com/AhaSend/ahasend-cli/internal/version"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var rootCmd = newRootCommand()

// newRootCommand builds the root command with its global flags and every
// command group. The production root command and NewRootCmdForTesting both
// use it, so tests run the same command tree that ships.
func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:     "ahasend",
		Version: getVersionString(),
		Short:   "Command line interface for the AhaSend email service",
		Long: `AhaSend CLI is a command-line tool for managing your AhaSend email service.
It provides functionality for sending emails, managing domains, webhooks,
suppressions, and more.

Before using the CLI, you'll need to authenticate with your AhaSend API key:
  ahasend auth login

For more information, visit: https://ahasend.com`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := checkCommandLine(cmd); err != nil {
				return err
			}
			// Cobra checks these after this hook; checking them here makes
			// every error after it the command's own, which run prints with
			// the command's response handler.
			if err := cmd.ValidateRequiredFlags(); err != nil {
				return err
			}
			if err := cmd.ValidateFlagGroups(); err != nil {
				return err
			}
			cmd.SetContext(context.WithValue(cmd.Context(), commandStartedKey{}, true))
			return nil
		},
		// run prints every error through a response handler, so Cobra
		// prints neither the error nor the usage.
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Set custom version template for detailed version information
	root.SetVersionTemplate(fmt.Sprintf(`AhaSend CLI %s
Build Time: %s
Git Commit: %s
`, version.Version, version.BuildTime, version.GitCommit))

	// Global persistent flags
	root.PersistentFlags().String("api-key", "", "AhaSend API key, used with --account-id instead of a profile; always uses the production API")
	root.PersistentFlags().String("account-id", "", "AhaSend Account ID of --api-key; used only with --api-key, which needs it")
	root.PersistentFlags().String("profile", "", "Profile to use (overrides default)")
	cmdutil.AddPersistentOneOfFlag(root, "output", "plain", "Output format: %s", printer.GetSupportedFormats())
	_ = root.RegisterFlagCompletionFunc("profile", cmdutil.CompleteProfileNames)
	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	root.PersistentFlags().Bool("debug", false, "Enable debug mode")

	addCommands(root)
	addCobraCommands(root)
	for _, group := range root.Commands() {
		rejectUnknownSubcommands(group)
	}
	cmdutil.CompleteNoFiles(root)

	return root
}

// checkCommandLine initializes the logger and the response handler of cmd
// and checks the global flags.
func checkCommandLine(cmd *cobra.Command) error {
	// Initialize logger first
	logger.Initialize(cmd)

	// Initialize printer and store in context
	if err := initializePrinter(cmd); err != nil {
		return err
	}

	// The auth commands work on profiles, so only login takes
	// credentials, with flags of its own
	if cmd.Parent() != nil && cmd.Parent().Name() == "auth" {
		return rejectCredentialFlags(cmd)
	}
	// The auth group itself, help and completion take no credentials
	if cmd.Name() == "auth" || cmd.Name() == "help" || cmd.Name() == "completion" {
		return nil
	}

	if err := cmdutil.CheckCredentialFlags(cmd); err != nil {
		return err
	}
	// --api-key always uses the production API, so it cannot take
	// a profile's API URL
	return cmdutil.RejectTogether(cmd, "api-key", "profile")
}

// commandStartedKey marks the context of a command whose command line passed
// every check, so an error it returns is its own.
type commandStartedKey struct{}

// commandStarted reports whether cmd passed every check of its command line
// and ran.
func commandStarted(cmd *cobra.Command) bool {
	ctx := cmd.Context()
	if ctx == nil {
		return false
	}
	started, _ := ctx.Value(commandStartedKey{}).(bool)
	return started
}

// rejectCredentialFlags rejects the global --api-key and --account-id flags
// on an auth command other than login, which would otherwise ignore them.
func rejectCredentialFlags(cmd *cobra.Command) error {
	if cmd.Name() == "login" {
		return nil
	}
	for _, name := range []string{"api-key", "account-id"} {
		if cmd.Flags().Changed(name) {
			return errors.NewValidationError(fmt.Sprintf("--%s cannot be used with auth %s", name, cmd.Name()), nil)
		}
	}
	return nil
}

// initializePrinter creates and stores the printer instance in the command context
func initializePrinter(cmd *cobra.Command) error {
	// Get output format and color settings
	outputFormat, _ := cmd.Flags().GetString("output")
	// The listen commands color their stdout with fatih/color
	color.NoColor = !terminal.UseColor(cmd, cmd.OutOrStdout())

	// Create response handler instance, which checks the format
	handler, err := printer.NewResponseHandler(outputFormat, cmd.OutOrStdout(), cmd.ErrOrStderr())
	if err != nil {
		return err
	}

	// Store in command context
	ctx := context.WithValue(cmd.Context(), printer.ResponseHandlerKey, handler)
	cmd.SetContext(ctx)

	return nil
}

// getVersionString returns the version information
func getVersionString() string {
	if version.Version == "dev" {
		return fmt.Sprintf("%s (built at %s, commit %s)", version.Version, version.BuildTime, version.GitCommit)
	}
	return version.Version
}

// handleError prints an error the command returned with its response
// handler and returns the exit code.
func handleError(cmd *cobra.Command, err error) int {
	// Get handler from context for error formatting
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// The command already printed its whole result, which shows the
	// failure. JSON output stays exactly what was printed; other formats add
	// one line to stderr.
	var itemsFailed *errors.ItemsFailedError
	var resultPrinted *errors.ResultPrintedError
	if stderrors.As(err, &itemsFailed) || stderrors.As(err, &resultPrinted) {
		if handler.GetFormat() != "json" {
			fmt.Fprintln(cmd.ErrOrStderr(), printer.EscapeDisplayText(err.Error()))
		}
		return errors.GetExitCode(err)
	}

	// Every failure exits nonzero in all output formats, including raw API
	// errors printed verbatim in JSON mode.
	handler.HandleError(err)
	return errors.GetExitCode(err)
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	ignoreSIGPIPE()
	if code := run(rootCmd, os.Args[1:]); code != 0 {
		os.Exit(code)
	}
}

// run executes root with args and returns the process exit code.
func run(root *cobra.Command, args []string) int {
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	if commandStarted(cmd) {
		return handleError(cmd, err)
	}

	// The rest come from Cobra (an unknown command or flag, a bad flag value,
	// a wrong argument count, a missing required flag) or from
	// PersistentPreRunE. Cobra's own errors are command-line mistakes, so
	// they are validation errors.
	var cliErr *errors.CLIError
	cobraError := !stderrors.As(err, &cliErr)
	if cobraError {
		err = errors.NewValidationError(err.Error(), nil)
	}
	// A flag value refused with a CLIError, such as a flag given under two
	// of its names, prints that error alone.
	var invalidValue *pflag.InvalidValueError
	if !cobraError && stderrors.As(err, &invalidValue) {
		err = cliErr
	}

	errorHandler(cmd, args).HandleError(err)
	return errors.GetExitCode(err)
}

// errorHandler returns the response handler that prints a command-line
// error: the one for the --output value in args, or the plain one when it is
// missing or unsupported. run reads --output from the raw arguments because a
// command-line error can stop Cobra before it parses the flag.
func errorHandler(cmd *cobra.Command, args []string) printer.ResponseHandler {
	flags := pflag.NewFlagSet("output", pflag.ContinueOnError)
	flags.ParseErrorsAllowlist.UnknownFlags = true
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	format := flags.String("output", "plain", "")
	_ = flags.Parse(args)
	if handler, err := printer.NewResponseHandler(*format, cmd.OutOrStdout(), cmd.ErrOrStderr()); err == nil {
		return handler
	}
	// plain is a supported format, so this cannot fail.
	handler, _ := printer.NewResponseHandler("plain", cmd.OutOrStdout(), cmd.ErrOrStderr())
	return handler
}

// addCommands adds the utility commands and fresh command group instances to
// root, for newRootCommand.
func addCommands(root *cobra.Command) {
	// Add utility commands
	root.AddCommand(newPingCommand())

	// Add command groups
	root.AddCommand(apikeys.NewCommand())
	root.AddCommand(auth.NewCommand())
	root.AddCommand(contacts.NewCommand())
	root.AddCommand(domains.NewCommand())
	root.AddCommand(lists.NewCommand())
	root.AddCommand(messages.NewCommand())
	root.AddCommand(routes.NewCommand())
	root.AddCommand(smtp.NewCommand())
	root.AddCommand(stats.NewCommand())
	root.AddCommand(subaccounts.NewCommand())
	root.AddCommand(suppressions.NewCommand())
	root.AddCommand(templates.NewCommand())
	root.AddCommand(webhooks.NewCommand())
}

// rejectUnknownSubcommands makes cmd and every command group below it a
// command that shows its help when run alone and rejects any argument as an
// unknown command. Cobra would otherwise show the help of a group for an
// unknown subcommand and exit 0. The rejection is a Cobra argument error, so
// run prints it like any other command-line mistake.
func rejectUnknownSubcommands(cmd *cobra.Command) {
	if !cmd.HasSubCommands() {
		return
	}
	if cmd.Run == nil && cmd.RunE == nil {
		cmd.Args = cobra.NoArgs
		cmd.SilenceUsage = true
		cmd.RunE = func(c *cobra.Command, _ []string) error {
			return c.Help()
		}
	}
	for _, child := range cmd.Commands() {
		rejectUnknownSubcommands(child)
	}
}

// addCobraCommands adds to root the help and completion commands, which
// Cobra would add when the CLI runs, so they follow the rules of every other
// command: the help command rejects a topic that names no command as an
// unknown command, with the error Cobra gives one, where Cobra's would
// print the usage and exit 0, and each completion command rejects an
// argument as unexpected. The completion command group gets the rules of
// every group from rejectUnknownSubcommands.
func addCobraCommands(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd()
	completion, _, _ := root.Find([]string{"completion"})
	for _, shell := range completion.Commands() {
		shell.Args = cmdutil.NoArgs
		shell.SilenceUsage = true
	}

	help, _, _ := root.Find([]string{"help"})
	help.Run = nil
	help.RunE = func(c *cobra.Command, args []string) error {
		target, rest, err := c.Root().Find(args)
		if err == nil && len(rest) > 0 {
			err = fmt.Errorf("unknown command %q for %q", rest[0], target.CommandPath())
		}
		if err != nil {
			return errors.NewValidationError(err.Error(), nil)
		}
		target.InitDefaultHelpFlag()
		target.InitDefaultVersionFlag()
		return target.Help()
	}
}

// NewRootCmdForTesting creates a fresh root command instance for testing
// This avoids state contamination between tests by creating new command instances
func NewRootCmdForTesting() *cobra.Command {
	return newRootCommand()
}
