package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the apikeys update command
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <key-id>",
		Short: "Update an API key",
		Long: `Update an existing API key's label, scopes and IP allow list.

You can update the following properties of an API key:
- Label: Change the descriptive name for identification
- Scopes: Modify the permissions granted to the key
- IP allow list: Restrict the source IPs that can use the key

When updating scopes, the new scopes completely replace the existing ones.
If you want to add a scope, include all existing scopes plus the new one.
--ip-allow-list likewise replaces the whole list, and --clear-ip-allow-list
removes it so the key works from any IP.

The API key secret cannot be changed. If you need a new secret, create a new
API key and delete the old one.

` + cmdutil.ScopesHelp(cmdutil.AccountKey),
		Example: `  # Update API key label
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --label "Updated Label"

  # Update API key scopes
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 \
    --scope messages:send:all \
    --scope messages:read:all \
    --scope statistics-transactional:read:all

  # Update both label and scopes
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 \
    --label "Production API v2" \
    --scope messages:send:all \
    --scope domains:read \
    --scope domains:write

  # Update to domain-specific scopes
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 \
    --scope messages:send:{example.com} \
    --scope webhooks:read:{example.com}

  # Restrict the key to one network
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --ip-allow-list 203.0.113.0/24

  # Allow the key from any IP again
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 --clear-ip-allow-list

  # JSON output for automation
  ahasend apikeys update fcb3f3bc-4ac8-4330-948d-1671fcf9a768 \
    --label "New Label" \
    --output json`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runAPIKeyUpdate,
		SilenceUsage: true,
	}

	cmdutil.AddAPIKeyUpdateFlags(cmd, cmdutil.AccountKey)

	return cmd
}

func runAPIKeyUpdate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	keyID := args[0]

	// Validate before auth: the key ID and the at-least-one-of guard.
	keyUUID, err := validation.ParseUUIDField("API key ID", keyID)
	if err != nil {
		return err
	}
	req, err := cmdutil.ReadAPIKeyUpdate(cmd, cmdutil.AccountKey)
	if err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithField("key_id", keyID).Debug("Updating API key")

	apiKey, err := client.UpdateAPIKey(keyUUID, req)
	if err != nil {
		return err
	}

	return handler.HandleUpdateAPIKey(apiKey, printer.ResultConfig{
		SuccessMessage: "API key updated successfully",
	})
}
