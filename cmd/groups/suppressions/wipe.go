package suppressions

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/spf13/cobra"
)

// NewWipeCommand creates the suppressions wipe command
func NewWipeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wipe",
		Short: "Delete all suppressions from your account",
		Long: `Delete all suppressions from your account.

⚠️  DANGER: This command permanently deletes ALL suppressions in your account.
This includes bounce suppressions, complaint suppressions, and manual suppressions.
After running this command, you will be able to send emails to all previously
suppressed addresses.

This action:
- Cannot be undone
- Affects all domains in your account
- Removes all suppression types (bounce, complaint, unsubscribe, manual, abuse)
- May result in sending emails to invalid addresses
- May impact your sender reputation

RECOMMENDED WORKFLOW:
1. Create a backup: ahasend suppressions list --output json > backup.json
2. Review the backup file
3. Run wipe command with confirmation
4. Monitor your sending carefully after wipe

Use --force flag for automation (NOT recommended for production).`,
		Example: `  # Wipe all suppressions (with confirmation)
  ahasend suppressions wipe

  # Create backup before wiping
  ahasend suppressions list --output json > suppressions-backup.json
  ahasend suppressions wipe

  # Force wipe without confirmation (dangerous)
  ahasend suppressions wipe --force

  # Wipe with JSON output for automation
  ahasend suppressions wipe --force --output json`,
		Args: cobra.NoArgs,
		RunE: runSuppressionsWipe,
	}

	// Add flags
	cmd.Flags().Bool("force", false, "Skip all confirmation prompts (DANGEROUS)")
	cmd.Flags().String("domain", "", "Domain to wipe suppressions for")
	return cmd
}

func runSuppressionsWipe(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flag values
	force, _ := cmd.Flags().GetBool("force")
	domain, _ := cmd.Flags().GetString("domain")
	var domainPtr *string
	if domain != "" {
		domainPtr = &domain
	}

	// Show safety warning and get confirmation (unless --force is used)
	if !force {
		confirmed, err := confirmWipe()
		if err != nil {
			return handler.HandleError(err)
		}
		if !confirmed {
			return handler.HandleSimpleSuccess("Suppression wipe cancelled")
		}
	}

	logger.Get().WithFields(map[string]interface{}{
		"force":  force,
		"action": "wipe_all_suppressions",
	}).Info("Wiping all suppressions")

	// Wipe all suppressions
	_, err = client.WipeSuppressions(domainPtr)
	if err != nil {
		return handler.HandleError(err)
	}

	// Use the new ResponseHandler to display wipe success
	successMsg := "All suppressions have been successfully deleted"
	if domain != "" {
		successMsg = fmt.Sprintf("All suppressions for domain %s have been successfully deleted", domain)
	}

	return handler.HandleDeleteSuppression(true, printer.DeleteConfig{
		SuccessMessage: successMsg,
		ItemName:       "suppressions",
	})
}

func confirmWipe() (bool, error) {
	fmt.Println("🚨 DANGER: PERMANENT SUPPRESSION WIPE 🚨")
	fmt.Println()
	fmt.Println("This will permanently delete ALL suppressions from your account.")
	fmt.Println()
	fmt.Println("Consequences:")
	fmt.Println("• All bounce suppressions will be removed")
	fmt.Println("• All complaint suppressions will be removed")
	fmt.Println("• All unsubscribe suppressions will be removed")
	fmt.Println("• All manual suppressions will be removed")
	fmt.Println("• You may send to invalid addresses and harm your reputation")
	fmt.Println("• This action cannot be undone")
	fmt.Println()
	fmt.Println("RECOMMENDED: Create a backup first:")
	fmt.Println("  ahasend suppressions list --output json > backup.json")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Double confirmation for safety
	fmt.Print("Do you want to create a backup first? (Y/n): ")
	backupResponse, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read backup confirmation: %w", err)
	}

	backupResponse = strings.ToLower(strings.TrimSpace(backupResponse))
	if backupResponse != "n" && backupResponse != "no" {
		fmt.Println()
		fmt.Println("Please create a backup first using:")
		fmt.Println("  ahasend suppressions list --output json > backup.json")
		fmt.Println()
		fmt.Println("Then run this command again when ready.")
		return false, nil
	}

	fmt.Println()
	fmt.Print("Are you absolutely sure you want to delete ALL suppressions? (yes/NO): ")
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read confirmation: %w", err)
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response != "yes" {
		return false, nil
	}

	fmt.Print("This action cannot be undone. Type 'DELETE ALL' to confirm: ")
	finalResponse, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read final confirmation: %w", err)
	}

	finalResponse = strings.TrimSpace(finalResponse)
	return finalResponse == "DELETE ALL", nil
}
