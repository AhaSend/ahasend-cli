// Package apikeys implements the nested `subaccounts api-keys` command group.
//
// Sub-account API keys are physically nested under a sub-account, so every
// command in this group takes the sub-account ID as its first positional
// argument and reuses the shared API-key printer handlers for output.
package apikeys

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the nested sub-account API-key command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "api-keys",
		Aliases: []string{"apikeys"},
		Short:   "Manage API keys for a sub-account",
		Long: `Manage API keys that belong to a specific sub-account.

Sub-account API keys are nested under a sub-account, so every command takes the
sub-account ID as its first positional argument.

Common workflow:
  1. List a sub-account's keys: ahasend subaccounts api-keys list <sub-account-id>
  2. Inspect one: ahasend subaccounts api-keys get <sub-account-id> <key-id>
  3. Create one: ahasend subaccounts api-keys create <sub-account-id> --label "CI" --scope messages:send:all
  4. Update or delete: ahasend subaccounts api-keys update|delete <sub-account-id> <key-id>`,
	}

	// Add read-only subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())

	// Add write subcommands
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
