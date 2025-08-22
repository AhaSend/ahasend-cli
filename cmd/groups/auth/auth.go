package auth

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the auth command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication and profiles",
		Long: `Authenticate with AhaSend and manage multiple authentication profiles.
This allows you to store and switch between different API keys and accounts.

Common workflow:
  1. Login with your API key: ahasend auth login
  2. Check your status: ahasend auth status
  3. Switch between profiles: ahasend auth switch <profile>
  4. Logout when done: ahasend auth logout`,
	}

	// Add subcommands
	cmd.AddCommand(NewLoginCommand())
	cmd.AddCommand(NewLogoutCommand())
	cmd.AddCommand(NewStatusCommand())
	cmd.AddCommand(NewSwitchCommand())

	return cmd
}
