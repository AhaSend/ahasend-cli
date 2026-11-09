package segments

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// NewCreateCommand creates the segments create command.
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name> --definition-file <path>",
		Short: "Create a segment",
		Long: `Create a segment from a definition file. A new segment is building until
its members are found, which can take some minutes on a large account, and
a campaign to it cannot be sent until it is ready; "segments get" shows its
state. An account has at most 100 segments.

--timezone is the IANA time zone, such as Europe/Berlin, that relative times
and days in the definition are read in; without it they are read in UTC.

` + definitionHelp,
		Example: `  # Create a segment
  ahasend segments create --name "Engaged pro customers" --definition-file engaged.json

  # Create a segment whose relative times are read in Berlin time
  ahasend segments create --name "Engaged pro customers" --definition-file engaged.json \
    --description "Pro plan, opened lately" --timezone Europe/Berlin --idempotency-key create-engaged`,
		Args:         cmdutil.NoArgs,
		RunE:         runSegmentsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Segment name (required)")
	addDefinitionFlag(cmd, "JSON file holding the segment definition (required)")
	cmd.Flags().String("description", "", "Segment description")
	cmd.Flags().String("timezone", "", "IANA time zone that relative times and days are read in, such as Europe/Berlin (UTC when omitted)")
	cmdutil.AddIdempotencyKeyFlag(cmd)
	return cmd
}

func runSegmentsCreate(cmd *cobra.Command, _ []string) error {
	name, err := cmdutil.ReadRequiredText(cmd, "name")
	if err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("name", &name, cmdutil.MaxNameLength, strings.TrimSpace); err != nil {
		return err
	}
	if err := cmdutil.CheckMaxCleanedLength("description", cmdutil.ChangedString(cmd, "description"), maxDescriptionLength, strings.TrimSpace); err != nil {
		return err
	}
	definition, err := readDefinition(cmd)
	if err != nil {
		return err
	}
	if definition == nil {
		return errors.NewValidationError("--"+definitionFlag+" is required", nil)
	}

	request := requests.CreateSegmentRequest{
		Name:        name,
		Description: cmdutil.ChangedString(cmd, "description"),
		Definition:  *definition,
		Timezone:    cmdutil.ChangedString(cmd, "timezone"),
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
		"name": name,
	}).Debug("Executing segments create command")

	segment, err := apiClient.CreateSegment(request, idempotency...)
	if err != nil {
		return err
	}
	return printer.GetResponseHandlerFromCommand(cmd).HandleSingleSegment(segment, printer.SingleConfig{
		SuccessMessage: "Segment created successfully",
		EmptyMessage:   "No segment created",
	})
}
