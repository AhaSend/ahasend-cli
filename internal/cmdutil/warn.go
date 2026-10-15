package cmdutil

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// ValidateWebhookURL checks that rawURL, the value of the flag name, is an
// absolute HTTP or HTTPS URL. Commands that accept it call WarnIfInsecureURL
// once every other input is valid.
func ValidateWebhookURL(name, rawURL string) error {
	return validateHTTPURL(name, rawURL, "https://example.com/webhook")
}

// validateHTTPURL checks rawURL, the value of the flag name, with
// validation.CheckHTTPURL; the error shows example.
func validateHTTPURL(name, rawURL, example string) error {
	if err := validation.CheckHTTPURL(rawURL, example); err != nil {
		return errors.NewValidationError(fmt.Sprintf("--%s %v", name, err), nil)
	}
	return nil
}

// WarnIfInsecureURL writes validation.PlainHTTPWarning to stderr when
// rawURL, the value of the flag name, sends what it is sent over plain HTTP
// to a host that is not a loopback host; sent names that, such as "each
// event" for a webhook or "each email" for a route. Warnings never reach
// stdout, so JSON output stays the API response alone.
func WarnIfInsecureURL(cmd *cobra.Command, name, rawURL, sent string) {
	if !validation.SendsInClear(rawURL) {
		return
	}
	fmt.Fprintln(printer.DisplayWriter(cmd.ErrOrStderr()), validation.PlainHTTPWarning("--"+name, rawURL, sent))
}

// What the URL of a webhook and of a route are sent, in the warning of
// WarnIfInsecureURL.
const (
	WebhookSends = "each event"
	RouteSends   = "each email"
)
