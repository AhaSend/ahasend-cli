package suppressions

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// suppressionBackupHint tells how to back up the suppressions before a wipe.
// One list call returns one page, so the hint pages through all of them.
const suppressionBackupHint = `Back up first. One list call saves one page of up to 100 suppressions:
  ahasend suppressions list --limit 100 --output json > backup-1.json
Repeat it with --after set to the page's next_cursor, saving each page,
until a page has no next_cursor. Add --domain to back up one domain.`

const suppressionWipeWarning = `WARNING: Wiping suppressions permanently deletes every one except protected
ones (unsubscribes and spam complaints), which the API keeps, so email can be
sent to every other previously suppressed address again. This cannot be
undone.

` + suppressionBackupHint

// NewWipeCommand creates the suppressions wipe command
func NewWipeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wipe",
		Short: "Delete all suppressions from your account",
		Long: `Delete all suppressions from your account, or from one domain with
--domain.

DANGER: This permanently deletes every suppression except protected ones
(recipients who unsubscribed or reported a message as spam), which the API
keeps. Afterwards email can be sent to every other previously suppressed
address, which may hurt your sender reputation. This cannot be undone.

` + suppressionBackupHint + `

Wipe asks for confirmation unless --force is set.`,
		Example: `  # Wipe all suppressions (with confirmation)
  ahasend suppressions wipe

  # Back up the first page of suppressions before wiping them
  ahasend suppressions list --limit 100 --output json > backup-1.json

  # Back up the next page, and so on until a page has no next_cursor
  ahasend suppressions list --limit 100 --after <next_cursor> --output json > backup-2.json

  # Wipe the suppressions of one domain
  ahasend suppressions wipe --domain example.com

  # Force wipe without confirmation (dangerous)
  ahasend suppressions wipe --force

  # Wipe with JSON output for automation
  ahasend suppressions wipe --force --output json`,
		Args:         cmdutil.NoArgs,
		RunE:         runSuppressionsWipe,
		SilenceUsage: true,
	}

	prompt.AddForceFlag(cmd)
	cmd.Flags().String("domain", "", "Domain to wipe suppressions for")
	return cmd
}

func runSuppressionsWipe(cmd *cobra.Command, args []string) error {
	domain, err := cmdutil.ReadNonEmpty(cmd, "domain", "")
	if err != nil {
		return err
	}
	if err := validation.ValidateOptionalDomainName("--domain", domain); err != nil {
		return err
	}

	target := "all suppressions"
	if domain != nil {
		target = fmt.Sprintf("all suppressions for domain %s", *domain)
	}
	question := fmt.Sprintf("%s\n\nAre you sure you want to wipe %s?", suppressionWipeWarning, target)
	if err := prompt.Confirm(cmd, question); err != nil {
		return err
	}

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"domain": cmd.Flag("domain").Value.String(),
		"action": "wipe_all_suppressions",
	}).Info("Wiping all suppressions")

	response, err := client.WipeSuppressions(domain)
	if err != nil {
		return err
	}

	successMsg := "All suppressions deleted successfully"
	if domain != nil {
		successMsg = fmt.Sprintf("All suppressions for domain '%s' deleted successfully", *domain)
	}

	return printer.GetResponseHandlerFromCommand(cmd).HandleSuccessResponse(response, printer.ResultConfig{
		SuccessMessage: successMsg,
	})
}
