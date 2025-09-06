package messages

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <message-id>",
		Short: "Get detailed information about a message",
		Long: `Get detailed information about a specific message including its content,
status, delivery details, and engagement metrics.

This command shows complete message information including the raw message content.`,
		Example: `  # Get message details
  ahasend messages get msg_1234567890abcdef

  # Get message details with JSON output
  ahasend messages get msg_1234567890abcdef --output json

  # Save message content to a file
  ahasend messages get msg_1234567890abcdef --output json | jq -r .content > message.txt`,
		Args:         cobra.ExactArgs(1),
		RunE:         runMessagesGet,
		SilenceUsage: true,
	}

	return cmd
}

func runMessagesGet(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	messageID := args[0]

	logger.Get().WithFields(map[string]interface{}{
		"message_id": messageID,
	}).Debug("Executing message get command")

	// Get message details
	response, err := client.GetMessage(messageID)
	if err != nil {
		return err
	}

	if response == nil {
		return errors.NewNotFoundError(fmt.Sprintf("message '%s' not found", messageID), nil)
	}

	// Handle successful message response
	// Note: We're using HandleSingleMessage which handles the basic message info
	// The Content field will be displayed appropriately based on the output format
	config := printer.SingleConfig{
		SuccessMessage: fmt.Sprintf("Message details for '%s'", messageID),
		EmptyMessage:   "Message not found",
		FieldOrder:     []string{"id", "status", "sender", "recipient", "subject", "created_at", "delivered_at"},
	}

	return handler.HandleSingleMessage(response, config)
}
