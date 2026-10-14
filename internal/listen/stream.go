package listen

import (
	stderrors "errors"
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/client"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/terminal"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// connectStream connects to the websocket stream at wsURL. When another
// connection holds the stream of the kind, such as "webhook", it asks on
// stderr whether to force a reconnect; declining is a validation error.
func connectStream(cmd *cobra.Command, apiClient client.AhaSendClient, wsURL, kind string) (*client.WebSocketClient, error) {
	wsClient, err := apiClient.ConnectWebSocket(wsURL, false)
	if err == nil {
		return wsClient, nil
	}
	if !stderrors.Is(err, client.ErrStreamTaken) {
		return nil, err
	}

	fmt.Fprintln(cmd.ErrOrStderr())
	stderrColor(cmd, color.FgYellow).Fprintln(cmd.ErrOrStderr(), "Another connection exists for this "+kind+".")
	if err := prompt.Confirm(cmd, "Do you want to force reconnect?"); err != nil {
		return nil, errors.NewValidationError("operation cancelled: another connection exists", nil)
	}
	return apiClient.ConnectWebSocket(wsURL, true)
}

// stderrColor returns attr as a color for the command's stderr, on as
// terminal.UseColor decides for stderr. fatih/color decides from stdout.
func stderrColor(cmd *cobra.Command, attr color.Attribute) *color.Color {
	c := color.New(attr)
	if terminal.UseColor(cmd, cmd.ErrOrStderr()) {
		c.EnableColor()
	} else {
		c.DisableColor()
	}
	return c
}

// AddListenFlags adds the --forward-to, --skip-verify and --slim-output flags
// of a listen command.
func AddListenFlags(cmd *cobra.Command) {
	cmd.Flags().String("forward-to", "", "Local endpoint to forward events to")
	cmd.Flags().Bool("skip-verify", false, "Skip SSL certificate verification for local endpoints when forwarding events (needs --forward-to)")
	cmd.Flags().Bool("slim-output", false, "Slim down the payload for printing to the console")
}

// ReadForwardTo returns the --forward-to URL of a listen command, or "" when
// it is empty. A value that is not an HTTP or HTTPS URL, a URL with a user
// name or password, which the banner and logs would show, and --skip-verify
// without --forward-to are validation errors, and a URL that sends events
// over plain HTTP to another host gets the warning of WarnIfInsecureURL, which
// names what is forwarded with sent. It runs once every other input is valid.
func ReadForwardTo(cmd *cobra.Command, sent string) (string, error) {
	forwardTo, _ := cmd.Flags().GetString("forward-to")
	if forwardTo == "" {
		if skipVerify, _ := cmd.Flags().GetBool("skip-verify"); skipVerify {
			return "", errors.NewValidationError("--skip-verify needs --forward-to", nil)
		}
		return "", nil
	}
	if err := cmdutil.ValidateWebhookURL("forward-to", forwardTo); err != nil {
		return "", err
	}
	cmdutil.WarnIfInsecureURL(cmd, "forward-to", forwardTo, sent)
	return forwardTo, nil
}
