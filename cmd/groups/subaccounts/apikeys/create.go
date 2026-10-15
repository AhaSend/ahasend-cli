package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the `subaccounts api-keys create` command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <sub-account-id> --label <label> --scope <scope>",
		Short: "Create an API key for a sub-account",
		Long: `Create an API key that belongs to a specific sub-account with the given
label and scopes.

The secret is displayed once after creation and cannot be retrieved again, so
store it securely immediately.

Creation is idempotent: provide your own --idempotency-key to make retries
safe, or one is generated for you. If the same idempotency key is replayed
within the 5-minute replay window, the API returns the original key including
its one-time secret; after that window the secret can no longer be recovered.

Use --ip-allow-list to restrict the source IPs that can authenticate with the
key. Each entry is a CIDR block or a bare IPv4/IPv6 address. Without it the key
works from any IP.

` + cmdutil.ScopesHelp(cmdutil.SubAccountKey),
		Example: `  # Create a sub-account API key
  ahasend subaccounts api-keys create 123e4567-e89b-12d3-a456-426614174000 \
    --label "Production API" \
    --scope messages:send:all \
    --scope domains:read

  # Create with a custom idempotency key for safe retries
  ahasend subaccounts api-keys create 123e4567-e89b-12d3-a456-426614174000 \
    --label "CI" \
    --scope messages:send:all \
    --idempotency-key my-unique-key

  # Create a key usable only from one network
  ahasend subaccounts api-keys create 123e4567-e89b-12d3-a456-426614174000 \
    --label "Production servers" \
    --scope messages:send:all \
    --ip-allow-list 203.0.113.0/24`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSubAccountAPIKeyCreate,
		SilenceUsage: true,
	}

	cmdutil.AddAPIKeyCreateFlags(cmd, cmdutil.SubAccountKey)
	cmdutil.AddIdempotencyKeyFlag(cmd)

	return cmd
}

func runSubAccountAPIKeyCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Validate before auth: sub-account ID, label, and every scope.
	subAccountUUID, err := validation.ParseUUIDField("sub-account ID", subAccountID)
	if err != nil {
		return err
	}
	req, err := cmdutil.ReadAPIKeyCreate(cmd, cmdutil.SubAccountKey)
	if err != nil {
		return err
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id": subAccountID,
		"label":          req.Label,
		"scopes":         req.Scopes,
	}).Debug("Executing subaccounts api-keys create command")

	apiKey, err := client.CreateSubAccountAPIKey(subAccountUUID, req, idempotency...)
	if err != nil {
		return err
	}

	return handler.HandleCreateAPIKey(apiKey, printer.ResultConfig{
		SuccessMessage: "API key created successfully",
	})
}
