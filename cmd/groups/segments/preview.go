package segments

import (
	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewPreviewCommand creates the segments preview command.
func NewPreviewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preview --definition-file <path>",
		Short: "Count the contacts a definition matches",
		Long: `Try a definition file without saving anything: the output is how many
contacts it matches now, then up to 10 of them. The API stops counting after
10 seconds, and the count is then "not counted in time"; the contacts are
shown either way. A preview needs the contacts:read scope as well as
segments:read.

--timezone is the IANA time zone, such as Europe/Berlin, that relative times
and days in the definition are read in; without it they are read in UTC.

` + definitionHelp,
		Example: `  # Preview a definition
  ahasend segments preview --definition-file engaged.json

  # Preview a definition with relative times read in Berlin time
  ahasend segments preview --definition-file engaged.json --timezone Europe/Berlin`,
		Args:         cmdutil.NoArgs,
		RunE:         runSegmentsPreview,
		SilenceUsage: true,
	}

	addDefinitionFlag(cmd, "JSON file holding the segment definition to try (required)")
	cmd.Flags().String("timezone", "", "IANA time zone that relative times and days are read in, such as Europe/Berlin (UTC when omitted)")
	return cmd
}

func runSegmentsPreview(cmd *cobra.Command, _ []string) error {
	definition, err := readDefinition(cmd)
	if err != nil {
		return err
	}
	if definition == nil {
		return errors.NewValidationError("--"+definitionFlag+" is required", nil)
	}

	request := requests.PreviewSegmentRequest{
		Definition: *definition,
		Timezone:   cmdutil.ChangedString(cmd, "timezone"),
	}

	apiClient, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Debug("Executing segments preview command")

	preview, err := apiClient.PreviewSegment(request)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSegmentPreview(preview, printer.SingleConfig{
		EmptyMessage: "No preview returned",
	})
}
