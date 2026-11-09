package segments

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates the segments update command.
func NewUpdateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <segment-id>",
		Short: "Update a segment",
		Long: `Partially update a segment. Only the flags you set are sent, and at least
one is needed. --description "" clears the description, and --timezone ""
reads the definition's relative times and days in UTC.

A change of the definition or the time zone builds the segment's members
again, and the segment is building until that ends. Any update of a failed
segment builds it again; "segments get" shows the reason it failed. A
change of only the name or the description of any other segment leaves its
members and its state as they are.

` + definitionHelp,
		Example: `  # Rename a segment
  ahasend segments update 123e4567-e89b-12d3-a456-426614174000 --name "Engaged customers"

  # Replace the rules of a segment
  ahasend segments update 123e4567-e89b-12d3-a456-426614174000 --definition-file engaged.json

  # Read the relative times of a segment in UTC
  ahasend segments update 123e4567-e89b-12d3-a456-426614174000 --timezone ""`,
		Args:         cmdutil.ExactArgs(1),
		RunE:         runSegmentsUpdate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "New segment name")
	cmd.Flags().String("description", "", "New segment description (empty clears it)")
	addDefinitionFlag(cmd, "JSON file holding the new segment definition, which replaces the rules")
	cmd.Flags().String("timezone", "", "New IANA time zone that relative times and days are read in (empty sets UTC)")
	return cmd
}

func runSegmentsUpdate(cmd *cobra.Command, args []string) error {
	segmentID, err := validation.ParseUUIDField("segment ID", args[0])
	if err != nil {
		return err
	}

	name, err := cmdutil.ReadNonEmpty(cmd, "name", "")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("description", cmdutil.ChangedString(cmd, "description"), maxDescriptionLength, strings.TrimSpace); err != nil {
		return err
	}
	definition, err := readDefinition(cmd)
	if err != nil {
		return err
	}

	request := requests.UpdateSegmentRequest{
		Name:        name,
		Description: cmdutil.ChangedString(cmd, "description"),
		Definition:  definition,
		Timezone:    cmdutil.ChangedString(cmd, "timezone"),
	}
	if request.Name == nil && request.Description == nil && request.Definition == nil && request.Timezone == nil {
		return errors.NewValidationError("at least one of --name, --description, --definition-file or --timezone must be provided", nil)
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"segment_id": args[0],
	}).Debug("Executing segments update command")

	segment, err := apiClient.UpdateSegment(segmentID, request)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleSegment(segment, printer.SingleConfig{
		SuccessMessage: "Segment updated successfully",
		EmptyMessage:   "No segment updated",
	})
}
