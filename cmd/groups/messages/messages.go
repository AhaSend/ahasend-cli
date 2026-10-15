package messages

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the messages command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "messages",
		Short: "Send and manage email messages",
		Long: `Send email messages and manage message operations using AhaSend: send
emails, list and inspect sent messages, and cancel scheduled ones.

Common workflow:
  1. Send an email: ahasend messages send --from user@example.com \
       --to recipient@example.com --subject "Hi" --text "Hello"
  2. Check its status: ahasend messages list --recipient recipient@example.com
  3. Cancel a scheduled one: ahasend messages cancel <message-id>`,
		Example: `  # Send a simple email
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Hello" --text "Hello World"

  # Send HTML email
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Welcome to AhaSend" --html "<h1>Welcome</h1>"

  # Send an HTML file with variables
  ahasend messages send --from sender@mydomain.com --to recipient@example.com --subject "Hi {{name}}" --html-file email.html --global-substitutions variables.json`,
	}

	// Add subcommands
	cmd.AddCommand(NewSendCommand())
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCancelCommand())

	return cmd
}
