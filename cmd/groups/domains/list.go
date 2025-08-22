package domains

import (
	"fmt"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
)

// NewListCommand creates the list command
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all domains",
		Long: `List all domains in your AhaSend account with their verification status,
DNS record status, and other details.

The list can be filtered and paginated for large numbers of domains.`,
		Example: `  # List all domains
  ahasend domains list

  # List domains with JSON output
  ahasend domains list --output json

  # List domains with pagination
  ahasend domains list --limit 10

  # Filter by DNS status
  ahasend domains list --status verified`,
		RunE:         runDomainsList,
		SilenceUsage: true,
	}

	cmd.Flags().Int32("limit", 0, "Maximum number of domains to return")
	cmd.Flags().String("cursor", "", "Pagination cursor for next page")
	cmd.Flags().String("status", "", "Filter by DNS status (verified, pending, failed)")

	return cmd
}

func runDomainsList(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	// Get flags
	limit, _ := cmd.Flags().GetInt32("limit")
	cursor, _ := cmd.Flags().GetString("cursor")
	status, _ := cmd.Flags().GetString("status")

	// Handle pagination
	var limitPtr *int32
	var cursorPtr *string

	if limit > 0 {
		limitPtr = &limit
	}
	if cursor != "" {
		cursorPtr = &cursor
	}

	// Log command execution
	logger.Get().WithFields(map[string]interface{}{
		"limit":  limit,
		"cursor": cursor,
		"status": status,
	}).Debug("Executing domains list command")

	// Fetch domains
	response, err := client.ListDomains(limitPtr, cursorPtr)
	if err != nil {
		return handler.HandleError(err)
	}

	// Handle empty response
	if response == nil || response.Data == nil || len(response.Data) == 0 {
		message := "No domains found"
		if status != "" {
			message = fmt.Sprintf("No domains found with status '%s'", status)
		}
		return handler.HandleEmpty(message)
	}

	// Apply status filtering if specified
	// TODO: This should ideally be done server-side for better performance
	if status != "" {
		var filteredData []responses.Domain
		for _, domain := range response.Data {
			var matches bool
			switch status {
			case "verified":
				matches = domain.DNSValid
			case "pending", "failed":
				matches = !domain.DNSValid
			}
			if matches {
				filteredData = append(filteredData, domain)
			}
		}

		// Create a new response with filtered data
		filteredResponse := &responses.PaginatedDomainsResponse{
			Data:       filteredData,
			Pagination: response.Pagination,
		}

		// Handle filtered empty result
		if len(filteredData) == 0 {
			return handler.HandleEmpty(fmt.Sprintf("No domains found with status '%s'", status))
		}

		response = filteredResponse
	}

	// Handle successful domains list response
	config := printer.ListConfig{
		SuccessMessage: "Domains retrieved successfully",
		EmptyMessage:   "No domains found",
		ShowPagination: true,
		FieldOrder:     []string{"domain", "dns_valid", "id", "created_at", "updated_at", "last_dns_check_at"},
	}

	return handler.HandleDomainList(response, config)
}
