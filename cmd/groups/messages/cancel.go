package messages

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewCancelCommand creates the cancel command
func NewCancelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <message-id>...",
		Short: "Cancel a scheduled message",
		Long: `Cancel a scheduled message that has not been sent yet.

The API stops a message that still waits in the delivery queue, such as one
scheduled for later delivery, and it is not sent. The API answers success for
any message it finds, including one already delivered, which the cancel
leaves as it is; check a message's status with 'ahasend messages get'.

The message ID is the message's UUID, or its Message-ID, such as
<550e8400-e29b-41d4-a716-446655440000@example.com>. Get it from:
- The output of messages send, which shows each message's Message-ID as its
  ID
- The messages list command with appropriate filters

Cancel asks for confirmation unless --force is set. Ctrl+C while several
messages are cancelled stops before the next one: the command shows the
messages it cancelled or failed to cancel and exits with code 130.`,
		Example: `  # Cancel a scheduled message
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000

  # Cancel a message by the Message-ID messages send returned
  ahasend messages cancel "<550e8400-e29b-41d4-a716-446655440000@example.com>"

  # Cancel multiple scheduled messages
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000 550e8400-e29b-41d4-a716-446655440001

  # Cancel with JSON output
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000 --force --output json`,
		Args:         cmdutil.MinimumNArgs(1),
		RunE:         runMessageCancel,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)

	return cmd
}

func runMessageCancel(cmd *cobra.Command, args []string) error {
	messageIDs := make([]string, len(args))
	for i, arg := range args {
		messageID, err := validation.ParseMessageID("message ID", arg)
		if err != nil {
			return err
		}
		messageIDs[i] = messageID
	}

	question := fmt.Sprintf("Are you sure you want to cancel message %s?", messageIDs[0])
	if len(messageIDs) > 1 {
		question = fmt.Sprintf("Are you sure you want to cancel %d messages?", len(messageIDs))
	}
	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Handle single message case
	if len(messageIDs) == 1 {
		msgID := messageIDs[0]
		logger.Get().WithFields(map[string]interface{}{
			"account_id": client.GetAccountID(),
			"message_id": msgID,
		}).Debug("Cancelling message")

		// Execute the cancellation
		response, err := client.CancelMessage(msgID)
		if err != nil {
			return err
		}

		return handler.HandleSuccessResponse(response, printer.ResultConfig{
			SuccessMessage: fmt.Sprintf("Message '%s' cancelled successfully", msgID),
		})
	}

	// Handle multiple messages: one API response per ID, in argument order.
	// SIGINT and SIGTERM stop the cancel before the next request, and the
	// command reports the messages it tried and exits with
	// interruptedExitCode.
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, a second one stops the command at once
	go func() {
		<-ctx.Done()
		stop()
	}()
	results := make([]printer.APIResult, 0, len(messageIDs))
	failed := 0
	for _, msgID := range messageIDs {
		if ctx.Err() != nil {
			break
		}
		logger.Get().WithFields(map[string]interface{}{
			"account_id": client.GetAccountID(),
			"message_id": msgID,
		}).Debug("Cancelling message")

		response, err := client.CancelMessage(msgID)
		if err != nil {
			failed++
			logger.Get().WithFields(map[string]interface{}{
				"message_id": msgID,
				"error":      err.Error(),
			}).Debug("Failed to cancel message")
		} else {
			logger.Get().WithField("message_id", msgID).Debug("Message cancelled successfully")
		}
		results = append(results, printer.APIResult{Item: msgID, Response: response, Err: err})
	}

	cancelled := len(results) - failed
	if len(results) > 0 {
		if err := handler.HandleAPIResults(results, printer.ResultConfig{
			SuccessMessage: cancelSummary(cancelled, len(messageIDs)),
		}); err != nil {
			return err
		}
	}
	if notTried := len(messageIDs) - len(results); notTried > 0 {
		interrupted := errors.NewResultPrintedError(fmt.Sprintf("interrupted: %d of %d messages were not cancelled",
			notTried, len(messageIDs)), interruptedExitCode)
		// JSON output prints nothing when no request was made, and the
		// error is not printed in JSON mode, so stderr says why.
		if len(results) == 0 && handler.GetFormat() == "json" {
			fmt.Fprintln(cmd.ErrOrStderr(), interrupted.Error())
		}
		return interrupted
	}
	if failed > 0 {
		return errors.NewItemsFailedError(failed, len(messageIDs), "messages")
	}
	return nil
}

// cancelSummary is the heading of the results of a cancel of total messages,
// of which cancelled were cancelled, in the words of the send summary.
func cancelSummary(cancelled, total int) string {
	switch cancelled {
	case total:
		return "Messages cancelled successfully"
	case 0:
		return "No message cancelled"
	default:
		return fmt.Sprintf("%d of %d messages cancelled", cancelled, total)
	}
}
