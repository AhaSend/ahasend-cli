package contacts

import (
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// contactStatuses are the values of --status.
var contactStatuses = []string{"enabled", "disabled", "blocked"}

// addCreateFieldFlags adds the contact field flags of create.
func addCreateFieldFlags(cmd *cobra.Command) {
	cmd.Flags().String("first-name", "", "Contact first name")
	cmd.Flags().String("last-name", "", "Contact last name")
	cmdutil.AddOneOfFlag(cmd, "status", "", "Contact status: %s", contactStatuses)
	cmd.Flags().String("status-reason", "", "Reason for the contact status")
	cmd.Flags().Bool("unsubscribed", false, "Whether the contact is globally unsubscribed")
	cmd.Flags().String("attributes", "", "Contact attributes as a JSON object")
}

// addUpdateFieldFlags adds the contact field flags of update.
func addUpdateFieldFlags(cmd *cobra.Command) {
	cmd.Flags().String("email", "", "New email address")
	cmd.Flags().String("first-name", "", "New first name (empty clears it)")
	cmd.Flags().String("last-name", "", "New last name (empty clears it)")
	cmdutil.AddOneOfFlag(cmd, "status", "", "New status: %s", contactStatuses)
	cmd.Flags().String("status-reason", "", "New reason for the status (empty clears it)")
	cmd.Flags().Bool("unsubscribed", false, "Whether the contact is globally unsubscribed (false resubscribes it)")
	cmd.Flags().String("attributes", "", "Attributes to change as a JSON object (a null value removes that attribute)")
}

// readAttributes returns the parsed --attributes, or nil when it was not set.
func readAttributes(cmd *cobra.Command) (map[string]any, error) {
	value := cmdutil.ChangedString(cmd, "attributes")
	if value == nil {
		return nil, nil
	}
	var attributes map[string]any
	if err := cmdutil.DecodeJSON(strings.NewReader(*value), &attributes, "--attributes", "JSON object"); err != nil {
		return nil, err
	}
	return attributes, nil
}

func readBatchFile(path string) (requests.BatchUpsertContactsRequest, error) {
	items, err := cmdutil.ReadDataItems[requests.BatchUpsertContactInput](path, "contacts", "contact")
	if err != nil {
		return requests.BatchUpsertContactsRequest{}, err
	}
	return requests.BatchUpsertContactsRequest{Data: items}, nil
}
