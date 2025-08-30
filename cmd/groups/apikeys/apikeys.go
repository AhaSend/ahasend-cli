package apikeys

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the apikeys command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apikeys",
		Short: "Manage API keys",
		Long: `Manage API keys for authentication and access control.

API keys are used to authenticate requests to the AhaSend API. You can create
multiple API keys with different scopes and labels to organize access for
different applications or team members.

Each API key has:
- A unique identifier and secret
- Configurable scopes (permissions)
- Optional labels for organization
- Creation and last used timestamps

Use these commands to create, list, update, and delete API keys as needed.`,
		Example: `  # List all API keys
  ahasend apikeys list

  # Create a new API key with messaging permissions
  ahasend apikeys create --label "Production API" \
    --scope messages:send:all \
    --scope domains:read

  # Create a limited scope API key for analytics
  ahasend apikeys create --label "Analytics Only" \
    --scope statistics-transactional:read:all \
    --scope messages:read:all

  # Create domain-specific API key
  ahasend apikeys create --label "App Emails" \
    --scope messages:send:{app.example.com} \
    --scope suppressions:read

  # Get details about a specific API key
  ahasend apikeys get ak_1234567890abcdef

  # Update API key label and scopes
  ahasend apikeys update ak_1234567890abcdef \
    --label "Updated Label" \
    --scope messages:send:all \
    --scope webhooks:read:all

  # Delete an API key
  ahasend apikeys delete ak_1234567890abcdef`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())

	return cmd
}
