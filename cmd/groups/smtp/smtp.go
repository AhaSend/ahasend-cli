package smtp

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the smtp command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "smtp",
		Short: "Manage SMTP credentials for email sending",
		Long: `Manage SMTP credentials for sending emails through AhaSend's SMTP service.

SMTP credentials allow you to send emails using standard SMTP protocol through
any email client or application that supports SMTP. This is useful for
integrating AhaSend with legacy systems, mail servers, or applications that
only support SMTP.

Each SMTP credential has:
- Username and password for authentication
- Scope (global or domain-specific)
- Sandbox mode option for testing

Use SMTP credentials to:
- Send emails from email clients (Outlook, Thunderbird, etc.)
- Integrate with applications that only support SMTP
- Use AhaSend from programming languages without SDK support
- Send transactional emails from servers or IoT devices`,
		Example: `  # List SMTP credentials
  ahasend smtp list

  # Create global SMTP credential
  ahasend smtp create --name "Main Server" --scope global

  # Create domain-specific credential
  ahasend smtp create --name "Notifications" --scope scoped --domains "notifications.example.com"

  # Get details of a specific credential
  ahasend smtp get <credential-id>

  # Test SMTP connection
  ahasend smtp send --test --server send.ahasend.com:587

  # Test SMTP sending, with the password from an environment variable
  printf '%s\n' "$SMTP_PASSWORD" | ahasend smtp send \
    --from sender@example.com --to recipient@example.com \
    --subject "SMTP test" --text "Hello" \
    --username smtp-user --password-stdin`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewSendCommand())

	return cmd
}
