package messages

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the messages command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Send and manage email messages",
		Long: `Send email messages and manage message operations using AhaSend.
This command group provides functionality for sending emails, managing templates,
and tracking message delivery status.

Common workflow:
  1. Send an email: ahasend messages send --from user@domain.com --to recipient@example.com
  2. Send with template: ahasend messages send --from sender@mydomain.com --recipients recipients.json --subject "Order {{order_id}} Confirmation" --html-template order.html
  3. Send to multiple recipients: ahasend messages send --from user@domain.com --to user1@example.com --to user2@example.com`,
		Example: `  # Send a simple email
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Hello" --text "Hello World"

  # Send HTML email
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Welcome to AhaSend" --html "<h1>Welcome</h1>"

  # Send with template and variables
  ahasend messages send --template email.html --data variables.json --from sender@mydomain.com --to recipient@example.com`,
	}

	// Add subcommands
	cmd.AddCommand(NewSendCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCancelCommand())

	return cmd
}
