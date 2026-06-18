package apikeys

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the `subaccounts api-keys create` command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <sub-account-id>",
		Short: "Create a new API key for a sub-account",
		Long: `Create a new API key that belongs to a specific sub-account with the given
label and scopes.

The secret is displayed once after creation and cannot be retrieved again, so
store it securely immediately.

Creation is idempotent: provide your own --idempotency-key to make retries safe,
or one is generated for you. If the same idempotency key is replayed within the
5-minute replay window, the API returns the original key including its one-time
secret; after that window the secret can no longer be recovered.`,
		Example: `  # Create a sub-account API key
  ahasend subaccounts api-keys create 123e4567-e89b-12d3-a456-426614174000 \
    --label "Production API" \
    --scope messages:send:all \
    --scope domains:read

  # Create with a custom idempotency key for safe retries
  ahasend subaccounts api-keys create 123e4567-e89b-12d3-a456-426614174000 \
    --label "CI" \
    --scope messages:send:all \
    --idempotency-key my-unique-key`,
		Args:         cobra.ExactArgs(1),
		RunE:         runSubAccountAPIKeyCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("label", "", "Label for the API key (required)")
	cmd.Flags().StringSlice("scope", []string{}, "Scopes to grant (required, can be used multiple times)")
	cmd.Flags().String("idempotency-key", "", "Idempotency key for safe retries (auto-generated if not provided)")

	cmd.MarkFlagRequired("label")
	cmd.MarkFlagRequired("scope")

	return cmd
}

func runSubAccountAPIKeyCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	subAccountID := args[0]

	// Get flag values
	label, _ := cmd.Flags().GetString("label")
	scopes, _ := cmd.Flags().GetStringSlice("scope")
	idempotencyKey, _ := cmd.Flags().GetString("idempotency-key")

	// Validate before auth: sub-account ID, label, and every scope.
	if err := validation.ValidateUUIDField("sub-account ID", subAccountID); err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		return errors.NewValidationError("--label is required", nil)
	}
	if len(scopes) == 0 {
		return errors.NewValidationError("at least one --scope is required", nil)
	}
	for _, scope := range scopes {
		if err := validation.ValidateScope(scope); err != nil {
			return errors.NewValidationError(err.Error(), nil)
		}
	}

	// Generate an idempotency key when the user did not provide one so the
	// create call is always safe to retry.
	if idempotencyKey == "" {
		idempotencyKey = uuid.NewString()
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"sub_account_id":  subAccountID,
		"label":           label,
		"scopes":          scopes,
		"idempotency_key": idempotencyKey,
	}).Debug("Executing subaccounts api-keys create command")

	// Build the request and create the key under the sub-account.
	req := requests.CreateAPIKeyRequest{
		Label:  label,
		Scopes: scopes,
	}

	apiKey, err := client.CreateSubAccountAPIKey(subAccountID, req, idempotencyKey)
	if err != nil {
		return err
	}

	// Reuse the shared create renderer so JSON stays a verbatim SDK APIKey and
	// the one-time secret renders identically to the top-level apikeys command.
	if err := handler.HandleCreateAPIKey(apiKey, printer.CreateConfig{
		SuccessMessage: "✅ API Key Created Successfully",
		ItemName:       "API key",
		FieldOrder:     []string{"id", "label", "public_key", "secret_key", "scopes", "created_at"},
	}); err != nil {
		return err
	}

	// Emit the replay-window note only for human-readable output and only when
	// the one-time secret is present. The note is local to this nested create
	// path and is never added to the shared printer handlers.
	printReplayWindowNote(cmd, handler, apiKey)

	return nil
}

// printReplayWindowNote prints the 5-minute idempotency replay-window note for
// table and plain output when the response carries a one-time secret.
func printReplayWindowNote(cmd *cobra.Command, handler printer.ResponseHandler, apiKey *responses.APIKey) {
	if apiKey == nil || apiKey.SecretKey == nil || *apiKey.SecretKey == "" {
		return
	}

	switch handler.GetFormat() {
	case "table", "plain":
		fmt.Fprintln(cmd.OutOrStdout(),
			"Note: Save this secret now — it is shown only once. Retrying with the same "+
				"--idempotency-key within the 5-minute replay window returns this same secret; "+
				"after that it cannot be recovered.")
	}
}
