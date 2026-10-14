package routes

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// maxRecipientLength is the most characters the API takes in a recipient
// filter.
const maxRecipientLength = 255

// validateRecipient checks a --recipient filter as the API does: at most 255
// characters, in the form local@domain, where the local part is not empty
// and may hold * wildcards, and the domain is a domain name. The API checks
// that the domain is one of the account's domains. A route's recipient
// cannot be cleared.
func validateRecipient(recipient string) error {
	if strings.TrimSpace(recipient) == "" {
		return errors.NewValidationError("--recipient cannot be empty: a route's recipient cannot be cleared", nil)
	}
	if utf8.RuneCountInString(recipient) > maxRecipientLength {
		return errors.NewValidationError(fmt.Sprintf("--recipient must be at most %d characters", maxRecipientLength), nil)
	}
	local, domain, found := strings.Cut(recipient, "@")
	if !found || local == "" || !validation.IsDomainName(domain) {
		return validation.InvalidValueError("--recipient", recipient,
			"must be local@domain with one of your domains, such as support@example.com or *@example.com")
	}
	return nil
}

// NewCommand creates the routes command group
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "routes",
		Short: "Manage inbound email routes",
		Long: `Manage inbound email routes for processing incoming messages through webhooks.
Routes allow you to configure how incoming emails are processed and forwarded
to your application endpoints.

Routes enable you to:
- Process inbound emails through webhooks
- Filter emails by recipient patterns
- Control attachment handling and formatting
- Group messages by conversation threads
- Strip reply content for cleaner processing

Common workflow:
  1. Create a route: ahasend routes create
  2. Configure recipient filtering and webhook settings
  3. Test your route endpoint
  4. Monitor route activity: ahasend routes list`,
	}

	// Add subcommands
	cmd.AddCommand(NewListCommand())
	cmd.AddCommand(NewGetCommand())
	cmd.AddCommand(NewCreateCommand())
	cmd.AddCommand(NewUpdateCommand())
	cmd.AddCommand(NewDeleteCommand())
	cmd.AddCommand(NewListenCommand())
	cmd.AddCommand(NewTriggerCommand())

	return cmd
}
