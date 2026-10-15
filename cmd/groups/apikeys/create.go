package apikeys

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the apikeys create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --label <label> --scope <scope>",
		Short: "Create an API key",
		Long: `Create an API key with the specified scopes and label.

API keys provide programmatic access to the AhaSend API. Each key has:
- A unique identifier and secret for authentication
- Configurable scopes that define what actions the key can perform
- A label for identification and organization

Choose scopes carefully based on your application's needs. Follow the
principle of least privilege by granting only the minimum required scopes.

The secret will be displayed once after creation and cannot be retrieved
again. Store it securely immediately after creation. Creation is idempotent:
provide your own --idempotency-key to make retries safe, or one is generated
for you.

` + cmdutil.ScopesHelp(cmdutil.AccountKey) + `

Use --ip-allow-list to restrict the source IPs that can authenticate with the
key. Each entry is a CIDR block or a bare IPv4/IPv6 address. Without it the key
works from any IP.`,
		Example: `  # Create with specific scopes and label
  ahasend apikeys create \
    --label "Production API" \
    --scope messages:send:all \
    --scope domains:read

  # Create a read-only key for analytics
  ahasend apikeys create \
    --label "Analytics Dashboard" \
    --scope statistics-transactional:read:all \
    --scope messages:read:all

  # Create a domain-restricted key
  ahasend apikeys create \
    --label "Domain-specific API" \
    --scope messages:send:{example.com} \
    --scope webhooks:read:{example.com}

  # Create a key usable only from specific IPs
  ahasend apikeys create \
    --label "Production servers" \
    --scope messages:send:all \
    --ip-allow-list 203.0.113.0/24 \
    --ip-allow-list 2001:db8::1`,
		Args:         cmdutil.NoArgs,
		RunE:         runAPIKeyCreate,
		SilenceUsage: true,
	}

	cmdutil.AddAPIKeyCreateFlags(cmd, cmdutil.AccountKey)
	cmdutil.AddIdempotencyKeyFlag(cmd)

	return cmd
}

func runAPIKeyCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	req, err := cmdutil.ReadAPIKeyCreate(cmd, cmdutil.AccountKey)
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

	// Log the operation
	logger.Get().WithFields(map[string]interface{}{
		"label":  req.Label,
		"scopes": req.Scopes,
	}).Debug("Creating API key")

	// Create the API key
	apiKey, err := client.CreateAPIKey(req, idempotency...)
	if err != nil {
		return err
	}

	// Handle successful response
	return handler.HandleCreateAPIKey(apiKey, printer.ResultConfig{
		SuccessMessage: "API key created successfully",
	})
}
