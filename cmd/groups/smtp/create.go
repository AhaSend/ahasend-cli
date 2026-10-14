package smtp

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the smtp create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name>",
		Short: "Create an SMTP credential",
		Long: `Create an SMTP credential for sending emails via SMTP protocol.

When creating an SMTP credential, you can choose between:
- Global scope: Can send from any verified domain
- Scoped: Can only send from the domains given with --domains

--name is required. When it is missing and stdin is a terminal, the command
asks for it; --non-interactive never asks.

A secure password will be generated automatically. Make sure to save it
as it will only be shown once and cannot be retrieved later.`,
		Example: `  # Create a global SMTP credential, asking for its name
  ahasend smtp create

  # Create with specific name
  ahasend smtp create --name "Production Server"

  # Create scoped credential for specific domains
  ahasend smtp create --name "onboarding" --scope scoped --domains onboarding.example.com --domains drip.example.com

  # Create sandbox credential for testing
  ahasend smtp create --name "Test Server" --sandbox`,
		Args:         cmdutil.NoArgs,
		RunE:         runSMTPCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Credential name (required)")
	cmdutil.AddOneOfFlag(cmd, "scope", "global", "Credential scope: %s", cmdutil.Scopes)
	cmdutil.AddRepeatableFlag(cmd, "domains", "Allowed domain for a scoped credential (can be used multiple times)")
	cmd.Flags().Bool("sandbox", false, "Create as sandbox credential for testing")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	prompt.AddNonInteractiveFlag(cmd)

	return cmd
}

func runSMTPCreate(cmd *cobra.Command, args []string) error {
	scope, _ := cmd.Flags().GetString("scope")
	scope, err := cmdutil.MatchOneOf(cmd, "scope", scope)
	if err != nil {
		return err
	}
	domains, err := cmdutil.ReadRepeatable(cmd, "domains", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateDomainNames("--domains", domains); err != nil {
		return err
	}
	if err := cmdutil.CheckScopeDomains(scope, domains); err != nil {
		return err
	}
	sandbox, _ := cmd.Flags().GetBool("sandbox")

	name, err := cmdutil.ReadOrAsk(cmd, "name", "Credential name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxLength("name", &name, cmdutil.MaxNameLength); err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"name":    name,
		"scope":   scope,
		"sandbox": sandbox,
		"domains": domains,
	}).Debug("Creating SMTP credential")

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	credential, err := client.CreateSMTPCredential(requests.CreateSMTPCredentialRequest{
		Name:    name,
		Scope:   scope,
		Sandbox: sandbox,
		Domains: domains,
	}, idempotency...)
	if err != nil {
		return err
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleCreateSMTP(credential, printer.ResultConfig{
		SuccessMessage: "SMTP credential created successfully",
	})
}
