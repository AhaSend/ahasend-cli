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
- Send transactional emails from servers or IoT devices

Examples:
  # List all SMTP credentials
  ahasend smtp list

  # Create a new SMTP credential
  ahasend smtp create --name "Production Server"

  # Get details of a specific credential
  ahasend smtp get <credential-id>

  # Test SMTP sending
  ahasend smtp send --from sender@example.com --to recipient@example.com`,
		Example: `  # List SMTP credentials
  ahasend smtp list

  # Create global SMTP credential
  ahasend smtp create --name "Main Server" --scope global

  # Create domain-specific credential
  ahasend smtp create --name "Marketing" --scope scoped --domains "marketing.example.com"

  # Test SMTP connection
  ahasend smtp send --test --server send.ahasend.com:587`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewSendCommand())

	return cmd
}
