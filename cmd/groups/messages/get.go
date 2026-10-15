package messages

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// NewGetCommand creates the get command
func NewGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <message-id>",
		Short: "Get a message",
		Long: `Get detailed information about a specific message including its content,
status, delivery details, and engagement metrics.

The message ID is the message's UUID, or its Message-ID, such as
<550e8400-e29b-41d4-a716-446655440000@example.com>, which messages send
prints as the ID of each message and the message carries in its Message-ID
header.

The output includes the raw message content.`,
		Example: `  # Get message details
  ahasend messages get 550e8400-e29b-41d4-a716-446655440000

  # Get a message by the Message-ID messages send returned
  ahasend messages get "<550e8400-e29b-41d4-a716-446655440000@example.com>"

  # Get message details with JSON output
  ahasend messages get 550e8400-e29b-41d4-a716-446655440000 --output json

  # Save message content to a file
  ahasend messages get 550e8400-e29b-41d4-a716-446655440000 --output json | jq -r .content > message.txt`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runMessagesGet,
		SilenceUsage: true,
	}

	return cmd
}

func runMessagesGet(cmd *cobra.Command, args []string) error {
	messageID := args[0]
	messageID, err := validation.ParseMessageID("message ID", messageID)
	if err != nil {
		return err
	}

	handler := printer.GetResponseHandlerFromCommand(cmd)
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"message_id": messageID,
	}).Debug("Executing message get command")

	// Get message details
	response, err := client.GetMessage(messageID)
	if err != nil {
		return err
	}

	// Handle successful message response
	// Note: We're using HandleSingleMessage which handles the basic message info
	// The Content field will be displayed appropriately based on the output format
	config := printer.SingleConfig{
		SuccessMessage: "Message retrieved successfully",
		EmptyMessage:   "Message not found",
	}

	return handler.HandleSingleMessage(response, config)
}
