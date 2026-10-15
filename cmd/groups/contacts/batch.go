package contacts

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewBatchCommand creates the synchronous contact batch command.
func NewBatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "batch --file <path>",
		Short: "Create or update contacts in a batch",
		Long: `Synchronously create or update up to 1,000 contacts from a JSON file whose
only top-level field is data. The limit counts every item, duplicates included.
Items may only hold contact fields, and no field may be null; leave a field out
to keep it unchanged. An attribute value of null removes that attribute.

The API reports each item's outcome. The whole result is printed, and the
command exits 1 when any item failed. Reuse an idempotency key only when
retrying the same file contents.`,
		Example: `  # Create or update the contacts in a file
  ahasend contacts batch --file contacts.json

  # Retry an import safely and print the API response
  ahasend contacts batch --file contacts.json --idempotency-key import-42 --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runContactsBatch,
		SilenceUsage: true,
	}
	cmd.Flags().String("file", "", `JSON file containing {"data":[...]} (required)`)
	cmdutil.MarkFileFlag(cmd, "file")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runContactsBatch(cmd *cobra.Command, _ []string) error {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		return errors.NewValidationError("--file is required", nil)
	}
	request, err := readBatchFile(path)
	if err != nil {
		return err
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"contacts": len(request.Data),
	}).Debug("Executing contacts batch command")

	response, err := apiClient.BatchUpsertContacts(request, idempotency...)
	if err != nil {
		return err
	}
	if err := printer.GetResponseHandlerFromCommand(cmd).HandleBatchContacts(response, printer.SingleConfig{
		SuccessMessage: "Contact batch completed",
		EmptyMessage:   "No contact results returned",
	}); err != nil {
		return err
	}
	if response != nil && response.Failed > 0 {
		return errors.NewItemsFailedError(response.Failed, len(response.Data), "contacts")
	}
	return nil
}
