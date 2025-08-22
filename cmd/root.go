package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/cmd/groups/apikeys"
	"github.com/AhaSend/ahasend-cli/cmd/groups/auth"
	"github.com/AhaSend/ahasend-cli/cmd/groups/domains"
	"github.com/AhaSend/ahasend-cli/cmd/groups/messages"
	"github.com/AhaSend/ahasend-cli/cmd/groups/routes"
	"github.com/AhaSend/ahasend-cli/cmd/groups/smtp"
	"github.com/AhaSend/ahasend-cli/cmd/groups/stats"
	"github.com/AhaSend/ahasend-cli/cmd/groups/suppressions"
	"github.com/AhaSend/ahasend-cli/cmd/groups/webhooks"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ahasend",
	Short: "AhaSend CLI - Command line interface for AhaSend email service",
	Long: `AhaSend CLI is a command-line tool for managing your AhaSend email service.
It provides functionality for sending emails, managing domains, webhooks,
suppressions, and more.

Before using the CLI, you'll need to authenticate with your AhaSend API key:
  ahasend auth login

For more information, visit: https://ahasend.com`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize logger first
		logger.Initialize(cmd)

		// Initialize printer and store in context
		if err := initializePrinter(cmd); err != nil {
			return err
		}

		// Skip auth validation for auth commands and version/help commands
		if cmd.Name() == "auth" || cmd.Parent().Name() == "auth" ||
			cmd.Name() == "help" || cmd.Name() == "version" ||
			cmd.Name() == "completion" {
			return nil
		}

		return validateGlobalAuth(cmd)
	},
	// Let Cobra handle errors and usage display normally
}

// initializePrinter creates and stores the printer instance in the command context
func initializePrinter(cmd *cobra.Command) error {
	// Get output format and color settings
	outputFormat, _ := cmd.Flags().GetString("output")
	noColor, _ := cmd.Flags().GetBool("no-color")
	colorOutput := !noColor

	// Validate output format
	if err := printer.ValidateFormat(outputFormat); err != nil {
		return err
	}

	// Create response handler instance
	handler := printer.GetResponseHandler(outputFormat, colorOutput, cmd.OutOrStdout())

	// Store in command context
	ctx := context.WithValue(cmd.Context(), responseHandlerKey, handler)
	cmd.SetContext(ctx)

	return nil
}

// validateGlobalAuth checks for global API key or existing profile
func validateGlobalAuth(cmd *cobra.Command) error {
	// Check for global --api-key and --account-id flags
	apiKey, _ := cmd.Flags().GetString("api-key")
	accountID, _ := cmd.Flags().GetString("account-id")

	// If --api-key is provided, --account-id is required
	if apiKey != "" && accountID == "" {
		return errors.NewValidationError("--account-id is required when using --api-key", nil)
	}

	// If --account-id is provided, --api-key is required
	if accountID != "" && apiKey == "" {
		return errors.NewValidationError("--api-key is required when using --account-id", nil)
	}

	// No global API key, check for existing profile
	// This validation will be implemented when we create commands that need auth
	return nil
}

// Global variable to track exit code when we suppress error return
var globalExitCode int

type responseHandlerKeyType string

// Context keys for printer instances
const responseHandlerKey = responseHandlerKeyType("responseHandler")

// handleError formats and prints errors using the printer system
func handleError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	// Set exit code based on error type
	if cliErr, ok := err.(*errors.CLIError); ok {
		globalExitCode = errors.GetExitCode(cliErr)
	} else {
		globalExitCode = 1
	}

	// Get handler from context for error formatting
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Handle the error using the ResponseHandler
	handler.HandleError(err)

	// Show usage for argument/flag related errors
	errorMsg := err.Error()
	if isUsageError(errorMsg) {
		fmt.Fprintln(cmd.ErrOrStderr())
		cmd.Usage()
	}
}

// isUsageError determines if an error should trigger usage display
func isUsageError(errorMsg string) bool {
	// Only show usage for command-line syntax errors, not runtime errors
	usageKeywords := []string{
		"accepts", "requires", "unknown flag", "invalid flag", "missing required",
		"unknown command", "unknown shorthand flag", "flag needs an argument",
		"unsupported output format", // Our custom validation errors
	}

	// Explicitly exclude runtime errors that shouldn't show usage
	runtimeKeywords := []string{
		"failed to", "500 internal server error", "connection refused",
		"timeout", "network", "server error", "authentication failed",
		"not found", "forbidden", "unauthorized", "bad request",
		"invalid api key", "invalid account id",
	}

	errorLower := strings.ToLower(errorMsg)

	// If it's a runtime error, don't show usage
	for _, keyword := range runtimeKeywords {
		if strings.Contains(errorLower, keyword) {
			return false
		}
	}

	// If it's a usage error, show usage
	for _, keyword := range usageKeywords {
		if strings.Contains(errorLower, keyword) {
			return true
		}
	}

	return false
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	globalExitCode = 0 // Reset exit code
	err := rootCmd.Execute()
	if err != nil {
		// Error already handled by applyJSONErrorHandling, just exit with proper code
		if cliErr, ok := err.(*errors.CLIError); ok {
			os.Exit(errors.GetExitCode(cliErr))
		}
		os.Exit(1)
	}
	// Check if we need to exit with error code from handleError
	if globalExitCode != 0 {
		os.Exit(globalExitCode)
	}
}

func init() {
	// Global persistent flags
	rootCmd.PersistentFlags().String("api-key", "", "AhaSend API key (overrides profile)")
	rootCmd.PersistentFlags().String("account-id", "", "AhaSend Account ID (required with --api-key)")
	rootCmd.PersistentFlags().String("profile", "", "Profile to use (overrides default)")
	rootCmd.PersistentFlags().String("output", "plain", "Output format (table, json, plain)")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode")

	// Add utility commands
	rootCmd.AddCommand(pingCmd)

	// Add command groups
	rootCmd.AddCommand(apikeys.NewCommand())
	rootCmd.AddCommand(auth.NewCommand())
	rootCmd.AddCommand(domains.NewCommand())
	rootCmd.AddCommand(messages.NewCommand())
	rootCmd.AddCommand(routes.NewCommand())
	rootCmd.AddCommand(smtp.NewCommand())
	rootCmd.AddCommand(stats.NewCommand())
	rootCmd.AddCommand(suppressions.NewCommand())
	rootCmd.AddCommand(webhooks.NewCommand())

	// Apply JSON error handling to all commands recursively
	applyJSONErrorHandling(rootCmd)
}

// applyJSONErrorHandling applies JSON-aware error handling to a command and all its subcommands
func applyJSONErrorHandling(cmd *cobra.Command) {
	// Don't silence errors/usage - let Cobra handle validation errors normally
	// We'll only handle JSON output for actual command execution errors

	// Store the original RunE function if it exists
	if cmd.RunE != nil {
		originalRunE := cmd.RunE
		cmd.RunE = func(c *cobra.Command, args []string) error {
			err := originalRunE(c, args)
			if err != nil {
				handleError(c, err)
				// Return nil to prevent Cobra from handling the error again
				// We've already formatted and printed it
				return nil
			}
			return nil
		}
	}

	// Also handle flag errors
	cmd.SetFlagErrorFunc(func(c *cobra.Command, err error) error {
		handleError(c, err)
		// Return nil to prevent Cobra from handling the error again
		return nil
	})

	// Apply to all subcommands recursively
	for _, subCmd := range cmd.Commands() {
		applyJSONErrorHandling(subCmd)
	}
}

// GetRootCmd returns the root command for testing
func GetRootCmd() *cobra.Command {
	return rootCmd
}

// NewRootCmdForTesting creates a fresh root command instance for testing
// This avoids state contamination between tests by creating new command instances
func NewRootCmdForTesting() *cobra.Command {
	// Create fresh root command
	root := &cobra.Command{
		Use:   "ahasend",
		Short: "AhaSend CLI - Command line interface for AhaSend email service",
		Long: `AhaSend CLI is a command-line tool for managing your AhaSend email service.
It provides functionality for sending emails, managing domains, webhooks,
suppressions, and more.

Before using the CLI, you'll need to authenticate with your AhaSend API key:
  ahasend auth login

For more information, visit: https://ahasend.com`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logger first
			logger.Initialize(cmd)

			// Initialize printer and store in context
			if err := initializePrinter(cmd); err != nil {
				return err
			}

			// Skip auth validation for auth commands and version/help commands
			if cmd.Name() == "auth" || cmd.Parent().Name() == "auth" ||
				cmd.Name() == "help" || cmd.Name() == "version" ||
				cmd.Name() == "completion" {
				return nil
			}

			return validateGlobalAuth(cmd)
		},
		// Let Cobra handle errors and usage display normally
	}

	// Add global persistent flags
	root.PersistentFlags().String("api-key", "", "AhaSend API key (overrides profile)")
	root.PersistentFlags().String("account-id", "", "AhaSend Account ID (required with --api-key)")
	root.PersistentFlags().String("profile", "", "Profile to use (overrides default)")
	root.PersistentFlags().String("output", "plain", "Output format (table, json, plain)")
	root.PersistentFlags().Bool("no-color", false, "Disable colored output")
	root.PersistentFlags().Bool("verbose", false, "Enable verbose output")
	root.PersistentFlags().Bool("debug", false, "Enable debug mode")

	// Flattening configuration flags for complex data structures
	root.PersistentFlags().Int("flatten-arrays", 10, "Maximum array items to show as separate columns in CSV/table output")
	root.PersistentFlags().Int("flatten-depth", 3, "Maximum nesting depth for flattening complex structures")
	root.PersistentFlags().StringSlice("flatten-skip", []string{}, "Field names to skip during flattening (comma-separated)")

	// Add utility commands
	root.AddCommand(pingCmd)

	// Add fresh command group instances
	root.AddCommand(apikeys.NewCommand())
	root.AddCommand(auth.NewCommand())
	root.AddCommand(domains.NewCommand())
	root.AddCommand(messages.NewCommand())
	root.AddCommand(routes.NewCommand())
	root.AddCommand(suppressions.NewCommand())
	root.AddCommand(webhooks.NewCommand())

	return root
}
