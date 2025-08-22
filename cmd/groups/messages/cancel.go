package messages

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// NewCancelCommand creates the cancel command
func NewCancelCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <message-id>",
		Short: "Cancel a scheduled message",
		Long: `Cancel a scheduled message that has not been sent yet.

This command cancels a message that is scheduled for future delivery.
Once a message has been sent, it cannot be canceled.

The message ID can be obtained from:
- The response when sending a scheduled message
- The messages list command with appropriate filters`,
		Example: `  # Cancel a scheduled message
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000

  # Cancel multiple scheduled messages
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000 550e8400-e29b-41d4-a716-446655440001

  # Cancel with JSON output
  ahasend messages cancel 550e8400-e29b-41d4-a716-446655440000 --output json`,
		Args:         cobra.MinimumNArgs(1),
		RunE:         runMessageCancel,
		SilenceUsage: true,
	}

	// Add a force flag for bypassing confirmation
	cmd.Flags().BoolP("force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runMessageCancel(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// Get authenticated client
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Parse the account ID from client
	accountID, err := uuid.Parse(client.GetAccountID())
	if err != nil {
		return handler.HandleError(errors.NewConfigError("invalid account ID", err))
	}

	// Get flags
	force, _ := cmd.Flags().GetBool("force")

	// Validate message IDs
	messageIDs := args
	for _, msgID := range messageIDs {
		// Validate that it looks like a valid UUID
		if _, err := uuid.Parse(msgID); err != nil {
			// It might be a different format, but warn the user
			logger.Get().WithField("message_id", msgID).Debug("Message ID is not a valid UUID, attempting anyway")
		}
	}

	// Confirmation prompt if not forced
	// TODO: Interactive prompts should ideally be handled by the printer
	// to maintain format-aware behavior, but keeping here for now
	if !force {
		fmt.Printf("⚠️  You are about to cancel %d scheduled message(s).\n", len(messageIDs))
		fmt.Println("This action cannot be undone.")
		fmt.Print("Are you sure you want to continue? (yes/no): ")

		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "yes" && response != "y" {
			return handler.HandleSimpleSuccess("Cancellation aborted")
		}
	}

	// Handle single message case
	if len(messageIDs) == 1 {
		msgID := messageIDs[0]
		logger.Get().WithFields(map[string]interface{}{
			"account_id": accountID.String(),
			"message_id": msgID,
		}).Debug("Canceling message")

		// Execute the cancellation
		_, err := client.CancelMessage(accountID.String(), msgID)
		if err != nil {
			return handler.HandleError(err)
		}

		// Handle successful single message cancellation
		// TODO: Should use HandleCancelMessage when CancelMessageResponse type is properly defined
		return handler.HandleSimpleSuccess(fmt.Sprintf("✅ Message %s canceled successfully", msgID))
	}

	// Handle multiple messages
	successCount := 0
	var errors []string

	for _, msgID := range messageIDs {
		logger.Get().WithFields(map[string]interface{}{
			"account_id": accountID.String(),
			"message_id": msgID,
		}).Debug("Canceling message")

		_, err := client.CancelMessage(accountID.String(), msgID)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Message %s: %s", msgID, err.Error()))
			logger.Get().WithFields(map[string]interface{}{
				"message_id": msgID,
				"error":      err.Error(),
			}).Error("Failed to cancel message")
		} else {
			successCount++
			logger.Get().WithField("message_id", msgID).Debug("Message canceled successfully")
		}
	}

	// Generate summary message
	totalCount := len(messageIDs)
	failureCount := totalCount - successCount

	if failureCount == 0 {
		// All successful
		return handler.HandleSimpleSuccess(fmt.Sprintf("✅ Successfully canceled all %d messages", successCount))
	} else if successCount == 0 {
		// All failed
		return handler.HandleError(fmt.Errorf("failed to cancel all messages: %s", strings.Join(errors, "; ")))
	} else {
		// Partial success
		message := fmt.Sprintf("⚠️  Partial success: %d succeeded, %d failed out of %d total messages", successCount, failureCount, totalCount)
		return handler.HandleError(fmt.Errorf("%s. Failures: %s", message, strings.Join(errors, "; ")))
	}
}
