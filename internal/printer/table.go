package printer

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/olekukonko/tablewriter"
)

// tableHandler handles table output formatting with complete type safety
type tableHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *tableHandler) GetFormat() string {
	return "table"
}

// Error handling
func (h *tableHandler) HandleError(err error) error {
	fmt.Fprintf(h.writer, "Error: %v\n", err)
	// Return the original error to ensure non-zero exit code
	return err
}

// Domain responses
func (h *tableHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	// Define headers - respect FieldOrder if provided
	headers := []string{"Domain", "Status", "Created", "Updated", "Last DNS Check"}
	if len(config.FieldOrder) > 0 {
		// Use custom field order if specified
		headerMap := map[string]string{
			"domain":            "Domain",
			"dns_valid":         "Status",
			"status":            "Status",
			"created_at":        "Created",
			"updated_at":        "Updated",
			"last_dns_check_at": "Last DNS Check",
			"id":                "ID",
		}

		var orderedHeaders []string
		for _, field := range config.FieldOrder {
			if header, exists := headerMap[field]; exists {
				orderedHeaders = append(orderedHeaders, header)
			}
		}
		if len(orderedHeaders) > 0 {
			headers = orderedHeaders
		}
	}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	// Add data rows
	for _, domain := range response.Data {
		var row []string

		if len(config.FieldOrder) > 0 {
			// Build row according to field order
			fieldMap := map[string]string{
				"domain":            domain.Domain,
				"dns_valid":         formatDNSStatus(domain.DNSValid),
				"status":            formatDNSStatus(domain.DNSValid),
				"created_at":        formatTime(domain.CreatedAt),
				"updated_at":        formatTime(domain.UpdatedAt),
				"last_dns_check_at": formatTimePtr(domain.LastDNSCheckAt),
				"id":                formatUUID(domain.ID),
			}

			for _, field := range config.FieldOrder {
				if value, exists := fieldMap[field]; exists {
					row = append(row, value)
				}
			}
		} else {
			// Default order
			row = []string{
				domain.Domain,
				formatDNSStatus(domain.DNSValid),
				formatTime(domain.CreatedAt),
				formatTime(domain.UpdatedAt),
				formatTimePtr(domain.LastDNSCheckAt),
			}
		}

		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\nShowing %d domains", len(response.Data))
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, " (more available)")
		}
		fmt.Fprintf(h.writer, "\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	// Create field map for ordering
	fieldMap := map[string]string{
		"domain":            domain.Domain,
		"id":                formatUUID(domain.ID),
		"account_id":        formatUUID(domain.AccountID),
		"dns_valid":         formatDNSStatus(domain.DNSValid),
		"status":            formatDNSStatus(domain.DNSValid),
		"created_at":        formatTime(domain.CreatedAt),
		"updated_at":        formatTime(domain.UpdatedAt),
		"last_dns_check_at": formatTimePtr(domain.LastDNSCheckAt),
	}

	// Apply field ordering if specified
	var rows [][]string
	if len(config.FieldOrder) > 0 {
		// Use custom field order
		fieldNameMap := map[string]string{
			"domain":            "Domain",
			"id":                "ID",
			"account_id":        "Account ID",
			"dns_valid":         "DNS Status",
			"status":            "DNS Status",
			"created_at":        "Created",
			"updated_at":        "Updated",
			"last_dns_check_at": "Last DNS Check",
		}

		for _, field := range config.FieldOrder {
			if value, exists := fieldMap[field]; exists {
				fieldName := fieldNameMap[field]
				if fieldName == "" {
					fieldName = field
				}
				if value == "" && field == "last_dns_check_at" {
					value = "Never"
				}
				rows = append(rows, []string{fieldName, value})
			}
		}
	} else {
		// Default order
		rows = [][]string{
			{"Domain", domain.Domain},
			{"ID", formatUUID(domain.ID)},
			{"Account ID", formatUUID(domain.AccountID)},
			{"DNS Status", formatDNSStatus(domain.DNSValid)},
			{"Created", formatTime(domain.CreatedAt)},
			{"Updated", formatTime(domain.UpdatedAt)},
			{"Last DNS Check", func() string {
				if domain.LastDNSCheckAt != nil {
					return formatTimePtr(domain.LastDNSCheckAt)
				}
				return "Never"
			}()},
		}
	}

	// Add rows to table
	for _, row := range rows {
		addTableRow(table, row)
	}

	renderTable(table)

	// Show DNS records if any
	if len(domain.DNSRecords) > 0 {
		fmt.Fprintf(h.writer, "\nDNS Records:\n")
		dnsTable := h.createTable()
		dnsTable.Header("Host", "Record")

		for _, record := range domain.DNSRecords {
			addTableRow(
				dnsTable,
				[]string{
					record.Host,
					fmt.Sprintf("Type: %s\nContent: %s\nRequired: %s\nPropagated: %s\n",
						record.Type,
						record.Content,
						formatBooleanStatus(record.Required),
						formatBooleanStatus(record.Propagated),
					),
				},
			)
		}

		renderTable(dnsTable)
	}

	return nil
}

// Message responses
func (h *tableHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	// Define headers based on FieldOrder if provided, otherwise use defaults
	headers := []string{"API ID", "From", "To", "Subject", "Status", "Created", "Delivered", "Opens", "Clicks"}
	if len(config.FieldOrder) > 0 {
		headers = []string{}
		for _, field := range config.FieldOrder {
			switch field {
			case "api_id":
				headers = append(headers, "API ID")
			case "sender":
				headers = append(headers, "From")
			case "recipient":
				headers = append(headers, "To")
			case "subject":
				headers = append(headers, "Subject")
			case "status":
				headers = append(headers, "Status")
			case "created":
				headers = append(headers, "Created")
			case "delivered":
				headers = append(headers, "Delivered")
			case "opens":
				headers = append(headers, "Opens")
			case "clicks":
				headers = append(headers, "Clicks")
			case "message_id":
				headers = append(headers, "Message ID")
			case "direction":
				headers = append(headers, "Direction")
			case "domain_id":
				headers = append(headers, "Domain ID")
			case "attempts":
				headers = append(headers, "Attempts")
			case "tags":
				headers = append(headers, "Tags")
			case "bounce_class":
				headers = append(headers, "Bounce Class")
			case "retain_until":
				headers = append(headers, "Retain Until")
			}
		}
	}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	// Add rows
	for _, message := range response.Data {
		row := []string{}
		for _, field := range config.FieldOrder {
			switch field {
			case "api_id":
				row = append(row, formatUUID(message.ApiID))
			case "sender":
				row = append(row, message.Sender)
			case "recipient":
				row = append(row, message.Recipient)
			case "subject":
				row = append(row, message.Subject)
			case "status":
				row = append(row, message.Status)
			case "created":
				row = append(row, formatTime(message.CreatedAt))
			case "delivered":
				row = append(row, formatTimePtr(message.DeliveredAt))
			case "opens":
				row = append(row, formatInt(int(message.OpenCount)))
			case "clicks":
				row = append(row, formatInt(int(message.ClickCount)))
			case "message_id":
				row = append(row, message.MessageID)
			case "direction":
				row = append(row, message.Direction)
			case "domain_id":
				row = append(row, formatUUID(message.DomainID))
			case "attempts":
				row = append(row, formatInt(int(message.NumAttempts)))
			case "tags":
				row = append(row, formatStringSlice(message.Tags))
			case "bounce_class":
				row = append(row, formatOptionalString(message.BounceClassification))
			case "retain_until":
				row = append(row, formatTime(message.RetainUntil))
			}
		}

		// If no field order specified, use default row
		if len(config.FieldOrder) == 0 {
			row = []string{
				formatUUID(message.ApiID),
				message.Sender,
				message.Recipient,
				message.Subject,
				message.Status,
				formatTime(message.CreatedAt),
				formatTimePtr(message.DeliveredAt),
				formatInt(int(message.OpenCount)),
				formatInt(int(message.ClickCount)),
			}
		}

		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\nShowing %d messages", len(response.Data))
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, " (more available)")
		}
		fmt.Fprintf(h.writer, "\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	// Core message details
	addTableRow(table, []string{"API ID", formatUUID(message.ApiID)})
	addTableRow(table, []string{"AhaSend ID", message.AhasendID})
	addTableRow(table, []string{"Account ID", formatUUID(message.AccountID)})
	addTableRow(table, []string{"From", message.Sender})
	addTableRow(table, []string{"To", message.Recipient})
	addTableRow(table, []string{"Subject", message.Subject})
	addTableRow(table, []string{"Status", message.Status})
	addTableRow(table, []string{"Direction", message.Direction})
	addTableRow(table, []string{"Created", formatTime(message.CreatedAt)})
	addTableRow(table, []string{"Updated", formatTime(message.UpdatedAt)})

	// Optional delivery details
	if message.DeliveredAt != nil {
		addTableRow(table, []string{"Delivered", formatTimePtr(message.DeliveredAt)})
	} else {
		addTableRow(table, []string{"Delivered", "Not yet"})
	}

	// Engagement metrics
	addTableRow(table, []string{"Opens", formatInt(int(message.OpenCount))})
	addTableRow(table, []string{"Clicks", formatInt(int(message.ClickCount))})

	// Delivery details
	addTableRow(table, []string{"Attempts", formatInt(int(message.NumAttempts))})
	if message.BounceClassification != nil {
		addTableRow(table, []string{"Bounce Class", formatOptionalString(message.BounceClassification)})
	}

	// Technical details
	addTableRow(table, []string{"Message ID", message.MessageID})
	addTableRow(table, []string{"Domain ID", formatUUID(message.DomainID)})

	// Metadata
	if len(message.Tags) > 0 {
		addTableRow(table, []string{"Tags", formatStringSlice(message.Tags)})
	}
	addTableRow(table, []string{"Retain Until", formatTime(message.RetainUntil)})

	renderTable(table)

	return nil
}

func (h *tableHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error {
	if response == nil {
		fmt.Fprintf(h.writer, "No message data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Show summary
	fmt.Fprintf(h.writer, "Successfully sent %d messages\n\n", len(response.Data))

	// Create table for message details
	table := h.createTable()
	headerArgs := []any{"#", "Message ID", "Recipient", "Status", "Error"}
	table.Header(headerArgs...)

	for i, messageData := range response.Data {
		errorMsg := "-"
		if messageData.Error != nil {
			errorMsg = *messageData.Error
		}

		addTableRow(table, []string{
			fmt.Sprintf("%d", i+1),
			formatOptionalString(messageData.ID),
			messageData.Recipient.Email,
			messageData.Status,
			errorMsg,
		})
	}

	renderTable(table)

	return nil
}

func (h *tableHandler) HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error {
	if response == nil {
		fmt.Fprintf(h.writer, "No cancellation data received\n")
		return nil
	}

	if response.Success {
		fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "Message cancellation failed\n\n")
	}

	// Create table for cancellation details
	table := h.createBorderedTable()
	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Message ID", response.MessageID})
	addTableRow(table, []string{"Success", formatBooleanStatus(response.Success)})
	if response.Error != "" {
		addTableRow(table, []string{"Error", response.Error})
	}

	renderTable(table)

	return nil
}

// Webhook responses
func (h *tableHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	// Define headers - respect FieldOrder if provided
	headers := []string{"Name", "URL", "Enabled", "Events", "Created", "Updated"}
	if len(config.FieldOrder) > 0 {
		// Use custom field order if specified
		headerMap := map[string]string{
			"name":       "Name",
			"url":        "URL",
			"enabled":    "Enabled",
			"events":     "Events",
			"created_at": "Created",
			"updated_at": "Updated",
			"id":         "ID",
			"account_id": "Account ID",
			"secret":     "Secret",
			"domains":    "Domains",
			"scope":      "Scope",
		}

		var orderedHeaders []string
		for _, field := range config.FieldOrder {
			if header, exists := headerMap[field]; exists {
				orderedHeaders = append(orderedHeaders, header)
			}
		}
		if len(orderedHeaders) > 0 {
			headers = orderedHeaders
		}
	}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	// Add data rows
	for _, webhook := range response.Data {
		var row []string

		if len(config.FieldOrder) > 0 {
			// Build row according to field order
			fieldMap := map[string]string{
				"name":       webhook.Name,
				"url":        webhook.URL,
				"enabled":    formatBooleanStatus(webhook.Enabled),
				"events":     formatWebhookEvents(&webhook),
				"created_at": formatTime(webhook.CreatedAt),
				"updated_at": formatTime(webhook.UpdatedAt),
				"id":         formatUUID(webhook.ID),
				"account_id": formatUUID(webhook.AccountID),
				"secret":     formatWebhookSecret(webhook.Secret),
				"domains":    formatStringSlice(webhook.Domains),
				"scope":      webhook.Scope,
			}

			for _, field := range config.FieldOrder {
				if value, exists := fieldMap[field]; exists {
					row = append(row, value)
				}
			}
		} else {
			// Default order
			row = []string{
				webhook.Name,
				webhook.URL,
				formatBooleanStatus(webhook.Enabled),
				formatWebhookEvents(&webhook),
				formatTime(webhook.CreatedAt),
				formatTime(webhook.UpdatedAt),
			}
		}

		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\nShowing %d webhooks", len(response.Data))
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, " (more available)")
		}
		fmt.Fprintf(h.writer, "\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	// Create field map for ordering
	fieldMap := map[string]string{
		"name":       webhook.Name,
		"id":         formatUUID(webhook.ID),
		"account_id": formatUUID(webhook.AccountID),
		"url":        webhook.URL,
		"enabled":    formatBooleanStatus(webhook.Enabled),
		"events":     formatWebhookEvents(webhook),
		"secret":     formatWebhookSecret(webhook.Secret),
		"scope":      webhook.Scope,
		"domains":    formatStringSlice(webhook.Domains),
		"created_at": formatTime(webhook.CreatedAt),
		"updated_at": formatTime(webhook.UpdatedAt),
	}

	// Apply field ordering if specified
	var rows [][]string
	if len(config.FieldOrder) > 0 {
		// Use custom field order
		fieldNameMap := map[string]string{
			"name":       "Name",
			"id":         "ID",
			"account_id": "Account ID",
			"url":        "URL",
			"enabled":    "Enabled",
			"events":     "Events",
			"secret":     "Secret",
			"scope":      "Scope",
			"domains":    "Domain Restrictions",
			"created_at": "Created",
			"updated_at": "Updated",
		}

		for _, field := range config.FieldOrder {
			if value, exists := fieldMap[field]; exists {
				fieldName := fieldNameMap[field]
				if fieldName == "" {
					fieldName = field
				}
				rows = append(rows, []string{fieldName, value})
			}
		}
	} else {
		// Default order
		rows = [][]string{
			{"Name", webhook.Name},
			{"ID", formatUUID(webhook.ID)},
			{"Account ID", formatUUID(webhook.AccountID)},
			{"URL", webhook.URL},
			{"Enabled", formatBooleanStatus(webhook.Enabled)},
			{"Events", formatWebhookEvents(webhook)},
			{"Secret", formatWebhookSecret(webhook.Secret)},
			{"Scope", webhook.Scope},
			{"Domain Restrictions", func() string {
				if len(webhook.Domains) == 0 {
					return "None (all domains)"
				}
				return formatStringSlice(webhook.Domains)
			}()},
			{"Created", formatTime(webhook.CreatedAt)},
			{"Updated", formatTime(webhook.UpdatedAt)},
		}
	}

	// Add rows to table
	for _, row := range rows {
		addTableRow(table, row)
	}

	renderTable(table)

	// Show webhook statistics if available
	if webhook.SuccessCount > 0 || webhook.ErrorCount > 0 {
		fmt.Fprintf(h.writer, "\nWebhook Statistics:\n")
		statsTable := h.createTable()
		statsTable.Header("Metric", "Value")

		addTableRow(statsTable, []string{"Successful calls", formatUint64(webhook.SuccessCount)})
		addTableRow(statsTable, []string{"Failed calls", formatUint64(webhook.ErrorCount)})
		addTableRow(statsTable, []string{"Errors Since Last Success", formatInt(webhook.ErrorsSinceLastSuccess)})
		if webhook.LastRequestAt != nil {
			addTableRow(statsTable, []string{"Last Request", formatTimePtr(webhook.LastRequestAt)})
		} else {
			addTableRow(statsTable, []string{"Last Request", "Never"})
		}

		renderTable(statsTable)
	}

	return nil
}

func (h *tableHandler) HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "No webhook data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Show main webhook details
	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Name", webhook.Name})
	addTableRow(table, []string{"ID", formatUUID(webhook.ID)})
	addTableRow(table, []string{"URL", webhook.URL})
	addTableRow(table, []string{"Enabled", formatBooleanStatus(webhook.Enabled)})
	addTableRow(table, []string{"Events", formatWebhookEvents(webhook)})
	addTableRow(table, []string{"Secret", formatWebhookSecretCreation(webhook.Secret)})
	addTableRow(table, []string{"Scope", webhook.Scope})

	if len(webhook.Domains) > 0 {
		addTableRow(table, []string{"Domain Restrictions", formatStringSlice(webhook.Domains)})
	} else {
		addTableRow(table, []string{"Domain Restrictions", "None (all domains)"})
	}

	addTableRow(table, []string{"Created", formatTime(webhook.CreatedAt)})

	renderTable(table)

	// Show security note
	fmt.Fprintf(h.writer, "\n🔐 Security Note: Save the webhook secret above - it won't be shown again.\n")
	fmt.Fprintf(h.writer, "Use this secret to verify webhook signatures for security.\n")

	return nil
}

func (h *tableHandler) HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "No webhook data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Show updated webhook details
	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Name", webhook.Name})
	addTableRow(table, []string{"ID", formatUUID(webhook.ID)})
	addTableRow(table, []string{"URL", webhook.URL})
	addTableRow(table, []string{"Enabled", formatBooleanStatus(webhook.Enabled)})
	addTableRow(table, []string{"Events", formatWebhookEvents(webhook)})
	addTableRow(table, []string{"Secret", formatWebhookSecret(webhook.Secret)})
	addTableRow(table, []string{"Scope", webhook.Scope})

	if len(webhook.Domains) > 0 {
		addTableRow(table, []string{"Domain Restrictions", formatStringSlice(webhook.Domains)})
	} else {
		addTableRow(table, []string{"Domain Restrictions", "None (all domains)"})
	}

	addTableRow(table, []string{"Created", formatTime(webhook.CreatedAt)})
	addTableRow(table, []string{"Updated", formatTime(webhook.UpdatedAt)})

	renderTable(table)

	// Show webhook statistics if available
	if webhook.SuccessCount > 0 || webhook.ErrorCount > 0 {
		fmt.Fprintf(h.writer, "\nWebhook Statistics:\n")
		statsTable := h.createTable()
		statsTable.Header("Metric", "Value")

		addTableRow(statsTable, []string{"Total Events", formatInt(int(webhook.SuccessCount + webhook.ErrorCount))})
		addTableRow(statsTable, []string{"Successful", formatInt(int(webhook.SuccessCount))})
		addTableRow(statsTable, []string{"Failed", formatInt(int(webhook.ErrorCount))})
		addTableRow(statsTable, []string{"Errors Since Last Success", formatInt(webhook.ErrorsSinceLastSuccess)})
		if webhook.LastRequestAt != nil {
			addTableRow(statsTable, []string{"Last Request", formatTimePtr(webhook.LastRequestAt)})
		}

		renderTable(statsTable)
	}

	return nil
}

func (h *tableHandler) HandleDeleteWebhook(success bool, config DeleteConfig) error {
	if success {
		fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "Webhook deletion failed\n\n")
	}

	// Show deletion summary
	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Operation", "Delete Webhook"})
	addTableRow(table, []string{"Success", formatBooleanStatus(success)})
	if !success {
		addTableRow(table, []string{"Status", "Failed - webhook may not exist or insufficient permissions"})
	} else {
		addTableRow(table, []string{"Status", "Webhook permanently deleted"})
	}

	renderTable(table)

	return nil
}

func (h *tableHandler) HandleTriggerWebhook(webhookID string, events []string, config TriggerConfig) error {
	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Show trigger summary
	table := h.createBorderedTable()
	table.Header("Field", "Value")
	
	addTableRow(table, []string{"Operation", "Trigger Webhook"})
	addTableRow(table, []string{"Webhook ID", webhookID})
	addTableRow(table, []string{"Events Triggered", strings.Join(events, ", ")})
	addTableRow(table, []string{"Success", "✓ True"})

	renderTable(table)

	return nil
}

// Route responses
func (h *tableHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	if response == nil || len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()
	table.Header("ID", "Name", "URL", "Enabled", "Recipient", "Attachments", "Headers", "Group Messages", "Strip Replies", "Created", "Updated")

	for _, route := range response.Data {
		// Truncate URL for better table display
		url := route.URL
		if len(url) > 50 {
			url = url[:47] + "..."
		}

		row := []string{
			formatUUID(route.ID)[:8] + "...", // Show short ID
			route.Name,
			url,
			formatBooleanStatus(route.Enabled),
			route.Recipient,
			formatBooleanStatus(route.Attachments),
			formatBooleanStatus(route.Headers),
			formatBooleanStatus(route.GroupByMessageID),
			formatBooleanStatus(route.StripReplies),
			formatTime(route.CreatedAt),
			formatTime(route.UpdatedAt),
		}
		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if available
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\nShowing %d routes", len(response.Data))
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, " (more available)")
		}
		fmt.Fprintf(h.writer, "\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	data := [][]string{
		{"ID", formatUUID(route.ID)},
		{"Name", route.Name},
		{"URL", route.URL},
		{"Enabled", formatBooleanStatus(route.Enabled)},
		{"Recipient Filter", route.Recipient},
		{"Include Attachments", formatBooleanStatus(route.Attachments)},
		{"Include Headers", formatBooleanStatus(route.Headers)},
		{"Group by Message ID", formatBooleanStatus(route.GroupByMessageID)},
		{"Strip Replies", formatBooleanStatus(route.StripReplies)},
		{"Created", formatTime(route.CreatedAt)},
		{"Updated", formatTime(route.UpdatedAt)},
	}

	// Apply field ordering if specified
	if len(config.FieldOrder) > 0 {
		fieldMap := make(map[string]string)
		for _, row := range data {
			fieldMap[row[0]] = row[1]
		}
		orderedData := orderFields(fieldMap, config.FieldOrder)
		for _, row := range orderedData {
			addTableRow(table, row)
		}
	} else {
		for _, row := range data {
			addTableRow(table, row)
		}
	}

	renderTable(table)

	// Show route statistics if available
	fmt.Fprintf(h.writer, "\nRoute Statistics:\n")
	statsTable := h.createTable()
	statsTable.Header("Metric", "Value")

	addTableRow(statsTable, []string{"Successful calls", formatUint64(route.SuccessCount)})
	addTableRow(statsTable, []string{"Failed calls", formatUint64(route.ErrorCount)})
	addTableRow(statsTable, []string{"Errors Since Last Success", formatInt(route.ErrorsSinceLastSuccess)})
	if route.LastRequestAt != nil {
		addTableRow(statsTable, []string{"Last Request", formatTimePtr(route.LastRequestAt)})
	} else {
		addTableRow(statsTable, []string{"Last Request", "Never"})
	}

	renderTable(statsTable)

	return nil
}

func (h *tableHandler) HandleCreateRoute(route *responses.Route, config CreateConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "No route data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	data := [][]string{
		{"Route ID", formatUUID(route.ID)},
		{"Name", route.Name},
		{"URL", route.URL},
		{"Enabled", formatBooleanStatus(route.Enabled)},
		{"Recipient Filter", route.Recipient},
		{"Include Attachments", formatBooleanStatus(route.Attachments)},
		{"Include Headers", formatBooleanStatus(route.Headers)},
		{"Group by Message ID", formatBooleanStatus(route.GroupByMessageID)},
		{"Strip Replies", formatBooleanStatus(route.StripReplies)},
		{"Created", formatTime(route.CreatedAt)},
	}

	for _, row := range data {
		addTableRow(table, row)
	}

	renderTable(table)

	fmt.Fprintf(h.writer, "\n📋 Your route has been created and is ready to receive inbound emails.\n")
	if !route.Enabled {
		fmt.Fprintf(h.writer, "⚠️  Note: The route is currently disabled. Enable it to start processing emails.\n")
	}

	return nil
}

func (h *tableHandler) HandleUpdateRoute(route *responses.Route, config UpdateConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "No route data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	data := [][]string{
		{"Route ID", formatUUID(route.ID)},
		{"Name", route.Name},
		{"URL", route.URL},
		{"Enabled", formatBooleanStatus(route.Enabled)},
		{"Recipient Filter", route.Recipient},
		{"Include Attachments", formatBooleanStatus(route.Attachments)},
		{"Include Headers", formatBooleanStatus(route.Headers)},
		{"Group by Message ID", formatBooleanStatus(route.GroupByMessageID)},
		{"Strip Replies", formatBooleanStatus(route.StripReplies)},
		{"Last Updated", formatTime(route.UpdatedAt)},
	}

	for _, row := range data {
		addTableRow(table, row)
	}

	renderTable(table)

	fmt.Fprintf(h.writer, "\n✅ Route configuration has been updated successfully.\n")

	return nil
}

func (h *tableHandler) HandleDeleteRoute(success bool, config DeleteConfig) error {
	if !success {
		fmt.Fprintf(h.writer, "\n❌ Failed to delete %s\n", config.ItemName)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Status", "Action")

	addTableRow(table, []string{"✅ Success", fmt.Sprintf("%s has been permanently deleted", config.ItemName)})

	renderTable(table)

	fmt.Fprintf(h.writer, "\n⚠️  This action cannot be undone. Inbound emails will no longer be processed by this route.\n")

	return nil
}

// Suppression responses
func (h *tableHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	headers := []string{"ID", "Email", "Domain", "Reason", "Created", "Expires"}

	// Apply field ordering if provided
	if len(config.FieldOrder) > 0 {
		orderedHeaders := make([]string, 0)
		headerMap := make(map[string]bool)
		for _, header := range headers {
			headerMap[header] = true
		}

		for _, field := range config.FieldOrder {
			switch field {
			case "id", "ID":
				orderedHeaders = append(orderedHeaders, "ID")
				delete(headerMap, "ID")
			case "email", "Email":
				orderedHeaders = append(orderedHeaders, "Email")
				delete(headerMap, "Email")
			case "domain", "Domain":
				orderedHeaders = append(orderedHeaders, "Domain")
				delete(headerMap, "Domain")
			case "reason", "Reason":
				orderedHeaders = append(orderedHeaders, "Reason")
				delete(headerMap, "Reason")
			case "created", "created_at", "Created":
				orderedHeaders = append(orderedHeaders, "Created")
				delete(headerMap, "Created")
			case "expires", "expires_at", "Expires":
				orderedHeaders = append(orderedHeaders, "Expires")
				delete(headerMap, "Expires")
			}
		}

		// Add remaining headers
		for _, header := range headers {
			if headerMap[header] {
				orderedHeaders = append(orderedHeaders, header)
			}
		}
		headers = orderedHeaders
	}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	for _, suppression := range response.Data {
		row := make([]string, len(headers))
		for i, header := range headers {
			switch header {
			case "ID":
				row[i] = fmt.Sprintf("%d", suppression.ID)
			case "Email":
				row[i] = suppression.Email
			case "Domain":
				if suppression.Domain != "" {
					row[i] = suppression.Domain
				} else {
					row[i] = "-"
				}
			case "Reason":
				if suppression.Reason != "" {
					row[i] = suppression.Reason
				} else {
					row[i] = "-"
				}
			case "Created":
				row[i] = formatTime(suppression.CreatedAt)
			case "Expires":
				row[i] = formatTime(suppression.ExpiresAt)
			}
		}
		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination && response.Pagination.HasMore {
		fmt.Fprintf(h.writer, "\nMore suppressions available\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error {
	if suppression == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Email", suppression.Email})
	addTableRow(table, []string{"ID", fmt.Sprintf("%d", suppression.ID)})
	addTableRow(table, []string{"Account ID", formatUUID(suppression.AccountID)})

	if suppression.Domain != "" {
		addTableRow(table, []string{"Domain", suppression.Domain})
	}
	if suppression.Reason != "" {
		addTableRow(table, []string{"Reason", suppression.Reason})
	}

	addTableRow(table, []string{"Created", formatTime(suppression.CreatedAt)})
	addTableRow(table, []string{"Updated", formatTime(suppression.UpdatedAt)})
	addTableRow(table, []string{"Expires", formatTime(suppression.ExpiresAt)})

	renderTable(table)
	return nil
}

func (h *tableHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty("No suppressions created")
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()
	table.Header("Email", "ID", "Domain", "Reason", "Expires")

	for _, suppression := range response.Data {
		domain := "-"
		if suppression.Domain != "" {
			domain = suppression.Domain
		}

		reason := "-"
		if suppression.Reason != "" {
			reason = suppression.Reason
		}

		row := []string{
			suppression.Email,
			fmt.Sprintf("%d", suppression.ID),
			domain,
			reason,
			formatTime(suppression.ExpiresAt),
		}
		addTableRow(table, row)
	}

	renderTable(table)
	return nil
}

func (h *tableHandler) HandleDeleteSuppression(success bool, config DeleteConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

func (h *tableHandler) HandleWipeSuppression(count int, config WipeConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Wiped %d suppressions\n", count)
	return nil
}

func (h *tableHandler) HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error {
	if found {
		fmt.Fprintf(h.writer, "%s\n\n", config.FoundMessage)
		if suppression != nil {
			table := h.createBorderedTable()
			table.Header("Field", "Value")

			addTableRow(table, []string{"Email", suppression.Email})
			addTableRow(table, []string{"ID", fmt.Sprintf("%d", suppression.ID)})

			if suppression.Domain != "" {
				addTableRow(table, []string{"Domain", suppression.Domain})
			}
			if suppression.Reason != "" {
				addTableRow(table, []string{"Reason", suppression.Reason})
			}

			addTableRow(table, []string{"Created", formatTime(suppression.CreatedAt)})
			addTableRow(table, []string{"Expires", formatTime(suppression.ExpiresAt)})

			renderTable(table)
		}
	} else {
		fmt.Fprintf(h.writer, "%s\n", config.NotFoundMessage)
	}
	return nil
}

// SMTP responses
func (h *tableHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	if response == nil || len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	// Define headers respecting FieldOrder if provided
	headers := []string{"ID", "Name", "Username", "Scope", "Domains", "Sandbox", "Created", "Updated"}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	for _, credential := range response.Data {
		domains := ""
		if len(credential.Domains) > 0 {
			domains = formatStringSlice(credential.Domains)
			// Truncate long domain lists
			if len(domains) > 30 {
				domains = domains[:27] + "..."
			}
		} else {
			domains = "-"
		}

		row := []string{
			fmt.Sprintf("%d", credential.ID),
			credential.Name,
			credential.Username,
			credential.Scope,
			domains,
			formatBooleanStatus(credential.Sandbox),
			formatTime(credential.CreatedAt),
			formatTime(credential.UpdatedAt),
		}
		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination && response.Pagination.HasMore {
		fmt.Fprintf(h.writer, "\nMore SMTP credentials available. Use --cursor to see next page.\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Create bordered table for detailed view
	table := h.createBorderedTable()

	// Convert headers to []any for table.Header
	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	// Add credential details
	addTableRow(table, []string{"Name", credential.Name})
	addTableRow(table, []string{"ID", fmt.Sprintf("%d", credential.ID)})
	addTableRow(table, []string{"Username", credential.Username})
	addTableRow(table, []string{"Password", "[HIDDEN - Never displayed for security]"})
	addTableRow(table, []string{"Scope", credential.Scope})

	if len(credential.Domains) > 0 {
		addTableRow(table, []string{"Domains", formatStringSlice(credential.Domains)})
	} else {
		addTableRow(table, []string{"Domains", "-"})
	}

	addTableRow(table, []string{"Sandbox Mode", formatBooleanStatus(credential.Sandbox)})
	addTableRow(table, []string{"Account ID", formatUUID(credential.AccountID)})
	addTableRow(table, []string{"Created", formatTime(credential.CreatedAt)})
	addTableRow(table, []string{"Updated", formatTime(credential.UpdatedAt)})

	renderTable(table)

	// Show SMTP connection settings
	fmt.Fprintf(h.writer, "\n📧 SMTP Connection Settings:\n\n")

	settingsTable := h.createBorderedTable()
	settingsHeaders := []any{"Setting", "Value"}
	settingsTable.Header(settingsHeaders...)

	addTableRow(settingsTable, []string{"Server", "send.ahasend.com"})
	addTableRow(settingsTable, []string{"Port (STARTTLS)", "587"})
	addTableRow(settingsTable, []string{"Port (SSL/TLS)", "465"})
	addTableRow(settingsTable, []string{"Username", credential.Username})
	addTableRow(settingsTable, []string{"Password", "[Use the password provided during creation]"})
	addTableRow(settingsTable, []string{"Authentication", "Plain or Login"})

	renderTable(settingsTable)

	return nil
}

func (h *tableHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Create bordered table for new credential
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Name", credential.Name})
	addTableRow(table, []string{"ID", fmt.Sprintf("%d", credential.ID)})
	addTableRow(table, []string{"Username", credential.Username})

	// Show password only on creation
	if credential.Password != "" {
		addTableRow(table, []string{"Password", credential.Password})
	} else {
		addTableRow(table, []string{"Password", "[Not provided]"})
	}

	addTableRow(table, []string{"Scope", credential.Scope})

	if len(credential.Domains) > 0 {
		addTableRow(table, []string{"Domains", formatStringSlice(credential.Domains)})
	} else {
		addTableRow(table, []string{"Domains", "-"})
	}

	addTableRow(table, []string{"Sandbox Mode", formatBooleanStatus(credential.Sandbox)})
	addTableRow(table, []string{"Created", formatTime(credential.CreatedAt)})

	renderTable(table)

	// Show important password warning if password was provided
	if credential.Password != "" {
		fmt.Fprintf(h.writer, "\n⚠️  IMPORTANT: Save the password shown above! It won't be displayed again.\n")
	}

	// Show SMTP connection settings
	fmt.Fprintf(h.writer, "\n📧 SMTP Connection Settings:\n\n")

	settingsTable := h.createBorderedTable()
	settingsHeaders := []any{"Setting", "Value"}
	settingsTable.Header(settingsHeaders...)

	addTableRow(settingsTable, []string{"Server", "send.ahasend.com"})
	addTableRow(settingsTable, []string{"Port (STARTTLS)", "587"})
	addTableRow(settingsTable, []string{"Port (SSL/TLS)", "465"})
	addTableRow(settingsTable, []string{"Username", credential.Username})
	if credential.Password != "" {
		addTableRow(settingsTable, []string{"Password", "[Use the password shown above]"})
	} else {
		addTableRow(settingsTable, []string{"Password", "[Use your saved password]"})
	}

	renderTable(settingsTable)

	return nil
}

func (h *tableHandler) HandleDeleteSMTP(success bool, config DeleteConfig) error {
	if success {
		fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "SMTP credential deletion failed\n\n")
	}

	// Show deletion summary
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Operation", "Delete SMTP Credential"})
	addTableRow(table, []string{"Success", formatBooleanStatus(success)})
	if !success {
		addTableRow(table, []string{"Status", "Failed - credential may not exist or insufficient permissions"})
	} else {
		addTableRow(table, []string{"Status", "SMTP credential permanently deleted"})
		addTableRow(table, []string{"Impact", "Applications using this credential will fail to authenticate"})
	}

	renderTable(table)

	return nil
}

func (h *tableHandler) HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result")
	}

	if result.TestMode {
		fmt.Fprintf(h.writer, "🔧 SMTP Connection Test Result\n\n")

		table := h.createBorderedTable()
		headerArgs := []any{"Test", "Result"}
		table.Header(headerArgs...)

		if result.Success {
			addTableRow(table, []string{"Connection", "✓ Successful"})
			addTableRow(table, []string{"Authentication", "✓ Valid"})
			addTableRow(table, []string{"Server Response", "Ready to accept messages"})
			addTableRow(table, []string{"Status", "SMTP configuration is working correctly"})
		} else {
			addTableRow(table, []string{"Connection", "✗ Failed"})
			if result.Error != "" {
				addTableRow(table, []string{"Error", result.Error})
			}
			addTableRow(table, []string{"Status", "Please check your SMTP settings"})
		}

		renderTable(table)
	} else {
		if result.Success {
			fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

			table := h.createBorderedTable()
			headerArgs := []any{"Field", "Value"}
			table.Header(headerArgs...)

			addTableRow(table, []string{"Status", "✓ Sent successfully"})
			if result.MessageID != "" {
				addTableRow(table, []string{"Message ID", result.MessageID})
			}
			addTableRow(table, []string{"Delivery", "Message queued for delivery"})

			renderTable(table)
		} else {
			fmt.Fprintf(h.writer, "❌ Failed to send message\n\n")

			table := h.createBorderedTable()
			headerArgs := []any{"Field", "Value"}
			table.Header(headerArgs...)

			addTableRow(table, []string{"Status", "✗ Send failed"})
			if result.Error != "" {
				addTableRow(table, []string{"Error", result.Error})
			}
			addTableRow(table, []string{"Action", "Check SMTP settings and try again"})

			renderTable(table)
		}
	}

	return nil
}

// API Key responses
func (h *tableHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	if response == nil || len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createTable()

	// Define headers respecting FieldOrder if provided
	headers := []string{"ID", "Label", "Public Key", "Scopes", "Last Used", "Created", "Updated"}

	// Convert headers to []any for table.Header
	headerArgs := make([]any, len(headers))
	for i, header := range headers {
		headerArgs[i] = header
	}
	table.Header(headerArgs...)

	for _, key := range response.Data {
		// Format scopes
		scopes := ""
		if len(key.Scopes) > 0 {
			scopeNames := make([]string, len(key.Scopes))
			for i, scope := range key.Scopes {
				scopeNames[i] = scope.Scope
			}
			scopes = formatStringSlice(scopeNames)
			// Truncate long scope lists
			if len(scopes) > 40 {
				scopes = scopes[:37] + "..."
			}
		} else {
			scopes = "-"
		}

		// Format last used
		lastUsed := "Never"
		if key.LastUsedAt != nil {
			lastUsed = formatTime(*key.LastUsedAt)
		}

		// Truncate public key for table display
		publicKey := key.PublicKey
		if len(publicKey) > 20 {
			publicKey = publicKey[:17] + "..."
		}

		row := []string{
			formatUUID(key.ID)[:8] + "...", // Show short ID
			key.Label,
			publicKey,
			scopes,
			lastUsed,
			formatTime(key.CreatedAt),
			formatTime(key.UpdatedAt),
		}
		addTableRow(table, row)
	}

	renderTable(table)

	// Show pagination info if enabled
	if config.ShowPagination && response.Pagination.HasMore {
		fmt.Fprintf(h.writer, "\nMore API keys available. Use --cursor to see next page.\n")
	}

	return nil
}

func (h *tableHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Create bordered table for detailed view
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	// Add API key details
	addTableRow(table, []string{"Label", key.Label})
	addTableRow(table, []string{"ID", formatUUID(key.ID)})
	addTableRow(table, []string{"Public Key", key.PublicKey})
	addTableRow(table, []string{"Secret Key", "[HIDDEN - Never displayed for security]"})
	addTableRow(table, []string{"Account ID", formatUUID(key.AccountID)})

	if key.LastUsedAt != nil {
		addTableRow(table, []string{"Last Used", formatTime(*key.LastUsedAt)})
	} else {
		addTableRow(table, []string{"Last Used", "Never"})
	}

	addTableRow(table, []string{"Created", formatTime(key.CreatedAt)})
	addTableRow(table, []string{"Updated", formatTime(key.UpdatedAt)})

	renderTable(table)

	// Show scopes in separate table if any
	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "\n🔑 API Key Scopes:\n\n")

		scopesTable := h.createBorderedTable()
		scopesHeaders := []any{"Scope", "Domain ID", "Created"}
		scopesTable.Header(scopesHeaders...)

		for _, scope := range key.Scopes {
			domainID := "-"
			if scope.DomainID != nil {
				domainID = formatUUID(*scope.DomainID)
			}

			addTableRow(scopesTable, []string{
				scope.Scope,
				domainID,
				formatTime(scope.CreatedAt),
			})
		}

		renderTable(scopesTable)
	} else {
		fmt.Fprintf(h.writer, "\n🔑 API Key Scopes: None\n")
	}

	return nil
}

func (h *tableHandler) HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Create bordered table for new API key
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Label", key.Label})
	addTableRow(table, []string{"ID", formatUUID(key.ID)})
	addTableRow(table, []string{"Public Key", key.PublicKey})

	// Show secret key only on creation
	if key.SecretKey != nil && *key.SecretKey != "" {
		addTableRow(table, []string{"Secret Key", *key.SecretKey})
	} else {
		addTableRow(table, []string{"Secret Key", "[Not provided in response]"})
	}

	addTableRow(table, []string{"Account ID", formatUUID(key.AccountID)})
	addTableRow(table, []string{"Created", formatTime(key.CreatedAt)})

	renderTable(table)

	// Show important secret key warning if provided
	if key.SecretKey != nil && *key.SecretKey != "" {
		fmt.Fprintf(h.writer, "\n⚠️  IMPORTANT: Save the secret key shown above! It won't be displayed again.\n")
	}

	// Show scopes if any
	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "\n🔑 Granted Scopes:\n\n")

		scopesTable := h.createBorderedTable()
		scopesHeaders := []any{"Scope", "Domain ID"}
		scopesTable.Header(scopesHeaders...)

		for _, scope := range key.Scopes {
			domainID := "-"
			if scope.DomainID != nil {
				domainID = formatUUID(*scope.DomainID)
			}

			addTableRow(scopesTable, []string{
				scope.Scope,
				domainID,
			})
		}

		renderTable(scopesTable)
	}

	return nil
}

func (h *tableHandler) HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	// Create bordered table for updated API key
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Label", key.Label})
	addTableRow(table, []string{"ID", formatUUID(key.ID)})
	addTableRow(table, []string{"Public Key", key.PublicKey})
	addTableRow(table, []string{"Account ID", formatUUID(key.AccountID)})
	addTableRow(table, []string{"Updated", formatTime(key.UpdatedAt)})

	renderTable(table)

	// Show current scopes if any
	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "\n🔑 Current Scopes:\n\n")

		scopesTable := h.createBorderedTable()
		scopesHeaders := []any{"Scope", "Domain ID", "Updated"}
		scopesTable.Header(scopesHeaders...)

		for _, scope := range key.Scopes {
			domainID := "-"
			if scope.DomainID != nil {
				domainID = formatUUID(*scope.DomainID)
			}

			addTableRow(scopesTable, []string{
				scope.Scope,
				domainID,
				formatTime(scope.UpdatedAt),
			})
		}

		renderTable(scopesTable)
	} else {
		fmt.Fprintf(h.writer, "\n🔑 Current Scopes: None\n")
	}

	return nil
}

func (h *tableHandler) HandleDeleteAPIKey(success bool, config DeleteConfig) error {
	if success {
		fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "API key deletion failed\n\n")
	}

	// Show deletion summary
	table := h.createBorderedTable()

	headerArgs := []any{"Field", "Value"}
	table.Header(headerArgs...)

	addTableRow(table, []string{"Operation", "Delete API Key"})
	addTableRow(table, []string{"Success", formatBooleanStatus(success)})
	if !success {
		addTableRow(table, []string{"Status", "Failed - API key may not exist or insufficient permissions"})
	} else {
		addTableRow(table, []string{"Status", "API key permanently deleted"})
		addTableRow(table, []string{"Impact", "Applications using this API key will no longer authenticate"})
	}

	renderTable(table)

	return nil
}

// Statistics responses
func (h *tableHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No deliverability statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	table := h.createTable()

	// Define headers - respect field order if provided and convert to display format
	defaultHeaders := []string{
		"Time Period", "Reception", "Delivered", "Deferred",
		"Bounced", "Failed", "Suppressed", "Opened", "Clicked",
		"Delivery Rate", "Open Rate",
	}
	headers := defaultHeaders
	if len(config.FieldOrder) > 0 {
		// Convert field order to display headers
		headers = make([]string, len(config.FieldOrder))
		for i, field := range config.FieldOrder {
			switch field {
			case "time_bucket", "time_period":
				headers[i] = "TIME BUCKET"
			case "sent", "reception", "reception_count":
				headers[i] = "SENT"
			case "delivered", "delivered_count":
				headers[i] = "DELIVERED"
			case "deferred", "deferred_count":
				headers[i] = "DEFERRED"
			case "bounced", "bounced_count":
				headers[i] = "BOUNCED"
			case "failed", "failed_count", "rejected":
				headers[i] = "REJECTED"
			case "suppressed", "suppressed_count":
				headers[i] = "SUPPRESSED"
			case "opened", "opened_count":
				headers[i] = "OPENED"
			case "clicked", "clicked_count":
				headers[i] = "CLICKED"
			case "delivery_rate":
				headers[i] = "DELIVERY RATE"
			case "open_rate":
				headers[i] = "OPEN RATE"
			default:
				// Fallback: convert snake_case to UPPER CASE
				headers[i] = strings.ToUpper(strings.ReplaceAll(field, "_", " "))
			}
		}
	}

	// Convert to []any for tablewriter
	headerAny := make([]any, len(headers))
	for i, header := range headers {
		headerAny[i] = header
	}
	table.Header(headerAny...)

	for _, stat := range response.Data {
		timePeriod := fmt.Sprintf("%s to %s",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		// Calculate rates
		deliveryRate := "N/A"
		if stat.ReceptionCount > 0 {
			rate := (float64(stat.DeliveredCount) / float64(stat.ReceptionCount)) * 100
			deliveryRate = fmt.Sprintf("%.2f%%", rate)
		}

		openRate := "N/A"
		if stat.DeliveredCount > 0 {
			rate := (float64(stat.OpenedCount) / float64(stat.DeliveredCount)) * 100
			openRate = fmt.Sprintf("%.2f%%", rate)
		}

		row := []string{
			timePeriod,
			formatInt(stat.ReceptionCount),
			formatInt(stat.DeliveredCount),
			formatInt(stat.DeferredCount),
			formatInt(stat.BouncedCount),
			formatInt(stat.FailedCount),
			formatInt(stat.SuppressedCount),
			formatInt(stat.OpenedCount),
			formatInt(stat.ClickedCount),
			deliveryRate,
			openRate,
		}

		// Filter row based on field order if specified
		if len(config.FieldOrder) > 0 {
			fieldMap := map[string]string{
				// Human-readable field names
				"Time Period":   timePeriod,
				"Reception":     formatInt(stat.ReceptionCount),
				"Delivered":     formatInt(stat.DeliveredCount),
				"Deferred":      formatInt(stat.DeferredCount),
				"Bounced":       formatInt(stat.BouncedCount),
				"Failed":        formatInt(stat.FailedCount),
				"Suppressed":    formatInt(stat.SuppressedCount),
				"Opened":        formatInt(stat.OpenedCount),
				"Clicked":       formatInt(stat.ClickedCount),
				"Delivery Rate": deliveryRate,
				"Open Rate":     openRate,
				// Command-compatible field names
				"time_bucket":      timePeriod,
				"time_period":      timePeriod,
				"sent":             formatInt(stat.ReceptionCount),
				"reception":        formatInt(stat.ReceptionCount),
				"reception_count":  formatInt(stat.ReceptionCount),
				"delivered":        formatInt(stat.DeliveredCount),
				"delivered_count":  formatInt(stat.DeliveredCount),
				"deferred":         formatInt(stat.DeferredCount),
				"deferred_count":   formatInt(stat.DeferredCount),
				"bounced":          formatInt(stat.BouncedCount),
				"bounced_count":    formatInt(stat.BouncedCount),
				"failed":           formatInt(stat.FailedCount),
				"failed_count":     formatInt(stat.FailedCount),
				"rejected":         formatInt(stat.FailedCount), // "rejected" maps to failed
				"suppressed":       formatInt(stat.SuppressedCount),
				"suppressed_count": formatInt(stat.SuppressedCount),
				"opened":           formatInt(stat.OpenedCount),
				"opened_count":     formatInt(stat.OpenedCount),
				"clicked":          formatInt(stat.ClickedCount),
				"clicked_count":    formatInt(stat.ClickedCount),
				"delivery_rate":    deliveryRate,
				"open_rate":        openRate,
			}

			orderedRow := make([]string, len(config.FieldOrder))
			for i, field := range config.FieldOrder {
				if value, exists := fieldMap[field]; exists {
					orderedRow[i] = value
				} else {
					orderedRow[i] = "-"
				}
			}
			row = orderedRow
		}

		addTableRow(table, row)
	}

	renderTable(table)
	return nil
}

func (h *tableHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No bounce statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	// First create a summary table showing time periods and totals
	summaryTable := h.createTable()
	summaryTable.Header("Time Period", "Total Bounces", "Classifications")

	for _, stat := range response.Data {
		timePeriod := fmt.Sprintf("%s to %s",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		totalBounces := 0
		var classifications []string
		for _, bounce := range stat.Bounces {
			totalBounces += bounce.Count
			classifications = append(classifications, bounce.Classification)
		}

		classificationStr := "-"
		if len(classifications) > 0 {
			classificationStr = fmt.Sprintf("%d types", len(classifications))
		}

		addTableRow(summaryTable, []string{
			timePeriod,
			formatInt(totalBounces),
			classificationStr,
		})
	}

	renderTable(summaryTable)

	// Then create detailed bounce classification tables for each time period
	for i, stat := range response.Data {
		if len(stat.Bounces) == 0 {
			continue
		}

		if i > 0 || len(response.Data) > 1 {
			fmt.Fprintf(h.writer, "\n")
		}

		timePeriod := fmt.Sprintf("%s to %s",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))
		fmt.Fprintf(h.writer, "Bounce Classifications - %s:\n\n", timePeriod)

		bounceTable := h.createBorderedTable()
		bounceTable.Header("Classification", "Count", "Percentage")

		totalBounces := 0
		for _, bounce := range stat.Bounces {
			totalBounces += bounce.Count
		}

		for _, bounce := range stat.Bounces {
			percentage := "0.0%"
			if totalBounces > 0 {
				pct := (float64(bounce.Count) / float64(totalBounces)) * 100
				percentage = fmt.Sprintf("%.1f%%", pct)
			}

			addTableRow(bounceTable, []string{
				bounce.Classification,
				formatInt(bounce.Count),
				percentage,
			})
		}

		renderTable(bounceTable)
	}

	return nil
}

func (h *tableHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No delivery time statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	// Main summary table
	summaryTable := h.createTable()

	defaultHeaders := []string{"Time Period", "Delivered Count", "Avg Delivery Time", "Per-Domain Stats"}
	headers := defaultHeaders
	if len(config.FieldOrder) > 0 {
		// Map command field names to human-readable headers
		headerMap := map[string]string{
			"time_bucket":       "TIME BUCKET",
			"avg_delivery_time": "AVG DELIVERY TIME",
			"message_count":     "MESSAGE COUNT",
			// Also support human-readable names
			"Time Period":       "TIME PERIOD",
			"Delivered Count":   "DELIVERED COUNT",
			"Avg Delivery Time": "AVG DELIVERY TIME",
			"Per-Domain Stats":  "PER-DOMAIN STATS",
		}

		mappedHeaders := make([]string, len(config.FieldOrder))
		for i, field := range config.FieldOrder {
			if header, exists := headerMap[field]; exists {
				mappedHeaders[i] = header
			} else {
				mappedHeaders[i] = strings.ToUpper(field)
			}
		}
		headers = mappedHeaders
	}

	headerAny := make([]any, len(headers))
	for i, header := range headers {
		headerAny[i] = header
	}
	summaryTable.Header(headerAny...)

	for _, stat := range response.Data {
		timePeriod := fmt.Sprintf("%s to %s",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		// Helper function to format delivery time
		formatDeliveryTime := func(seconds float64) string {
			timeStr := fmt.Sprintf("%.2fs", seconds)
			if seconds >= 60 {
				minutes := seconds / 60
				if minutes >= 60 {
					hours := minutes / 60
					timeStr = fmt.Sprintf("%.2fh", hours)
				} else {
					timeStr = fmt.Sprintf("%.2fm", minutes)
				}
			}
			return timeStr
		}

		// Format delivery times
		avgTimeStr := formatDeliveryTime(stat.AvgDeliveryTime)

		perDomainCount := "-"
		if len(stat.DeliveryTimes) > 0 {
			perDomainCount = fmt.Sprintf("%d domains", len(stat.DeliveryTimes))
		}

		row := []string{
			timePeriod,
			formatInt(stat.DeliveredCount),
			avgTimeStr,
			perDomainCount,
		}

		// Handle field ordering if specified
		if len(config.FieldOrder) > 0 {
			fieldMap := map[string]string{
				// Human-readable field names
				"Time Period":       timePeriod,
				"Delivered Count":   formatInt(stat.DeliveredCount),
				"Avg Delivery Time": avgTimeStr,
				"Per-Domain Stats":  perDomainCount,
				// Command-compatible field names
				"time_bucket":       timePeriod,
				"avg_delivery_time": avgTimeStr,
				"message_count":     formatInt(stat.DeliveredCount),
			}

			orderedRow := make([]string, len(config.FieldOrder))
			for i, field := range config.FieldOrder {
				if value, exists := fieldMap[field]; exists {
					orderedRow[i] = value
				} else {
					orderedRow[i] = "-"
				}
			}
			row = orderedRow
		}

		addTableRow(summaryTable, row)
	}

	renderTable(summaryTable)

	// Show per-domain delivery times for each time period that has them
	for _, stat := range response.Data {
		if len(stat.DeliveryTimes) == 0 {
			continue
		}

		fmt.Fprintf(h.writer, "\n")
		timePeriod := fmt.Sprintf("%s to %s",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))
		fmt.Fprintf(h.writer, "Per-Domain Delivery Times - %s:\n\n", timePeriod)

		domainTable := h.createBorderedTable()
		domainTable.Header("Recipient Domain", "Delivery Time")

		for _, dt := range stat.DeliveryTimes {
			domain := "Unknown"
			if dt.RecipientDomain != nil {
				domain = *dt.RecipientDomain
			}

			timeStr := "N/A"
			if dt.DeliveryTime != nil {
				timeStr = fmt.Sprintf("%.2fs", *dt.DeliveryTime)
				if *dt.DeliveryTime >= 60 {
					minutes := *dt.DeliveryTime / 60
					if minutes >= 60 {
						hours := minutes / 60
						timeStr = fmt.Sprintf("%.2fh", hours)
					} else {
						timeStr = fmt.Sprintf("%.2fm", minutes)
					}
				}
			}

			addTableRow(domainTable, []string{domain, timeStr})
		}

		renderTable(domainTable)
	}

	return nil
}

// Auth responses
func (h *tableHandler) HandleAuthLogin(success bool, profile string, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Profile", profile})
	addTableRow(table, []string{"Status", "Logged in"})

	renderTable(table)
	return nil
}

func (h *tableHandler) HandleAuthLogout(success bool, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Status", "Logged out"})

	renderTable(table)
	return nil
}

func (h *tableHandler) HandleAuthStatus(status *AuthStatus, config AuthConfig) error {
	if status == nil {
		fmt.Fprintf(h.writer, "No authentication status available\n")
		return nil
	}

	fmt.Fprintf(h.writer, "Authentication Status\n\n")

	// Main authentication details table
	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"Profile", status.Profile})
	addTableRow(table, []string{"API Key", status.APIKey})
	addTableRow(table, []string{"Valid", formatBooleanStatus(status.Valid)})

	renderTable(table)

	// Account details table if available
	if status.Account != nil {
		fmt.Fprintf(h.writer, "\nAccount Details:\n\n")

		accountTable := h.createBorderedTable()
		accountTable.Header("Field", "Value")

		addTableRow(accountTable, []string{"ID", formatUUID(status.Account.ID)})
		addTableRow(accountTable, []string{"Name", status.Account.Name})
		if status.Account.Website != nil {
			addTableRow(accountTable, []string{"Website", formatOptionalString(status.Account.Website)})
		}
		if status.Account.About != nil {
			addTableRow(accountTable, []string{"About", formatOptionalString(status.Account.About)})
		}
		addTableRow(accountTable, []string{"Created", formatTime(status.Account.CreatedAt)})
		addTableRow(accountTable, []string{"Updated", formatTime(status.Account.UpdatedAt)})

		renderTable(accountTable)

		// Email settings table
		if status.Account.TrackOpens != nil || status.Account.TrackClicks != nil ||
			status.Account.RejectBadRecipients != nil || status.Account.RejectMistypedRecipients != nil {
			fmt.Fprintf(h.writer, "\nEmail Settings:\n\n")

			settingsTable := h.createBorderedTable()
			settingsTable.Header("Setting", "Value")

			if status.Account.TrackOpens != nil {
				addTableRow(settingsTable, []string{"Track Opens", formatBooleanStatus(*status.Account.TrackOpens)})
			}
			if status.Account.TrackClicks != nil {
				addTableRow(settingsTable, []string{"Track Clicks", formatBooleanStatus(*status.Account.TrackClicks)})
			}
			if status.Account.RejectBadRecipients != nil {
				addTableRow(settingsTable, []string{"Reject Bad Recipients", formatBooleanStatus(*status.Account.RejectBadRecipients)})
			}
			if status.Account.RejectMistypedRecipients != nil {
				addTableRow(settingsTable, []string{"Reject Mistyped Recipients", formatBooleanStatus(*status.Account.RejectMistypedRecipients)})
			}

			renderTable(settingsTable)
		}

		// Data retention table
		if status.Account.MessageMetadataRetention != nil || status.Account.MessageDataRetention != nil {
			fmt.Fprintf(h.writer, "\nData Retention:\n\n")

			retentionTable := h.createBorderedTable()
			retentionTable.Header("Type", "Days")

			if status.Account.MessageMetadataRetention != nil {
				addTableRow(retentionTable, []string{"Message Metadata", formatInt(int(*status.Account.MessageMetadataRetention))})
			}
			if status.Account.MessageDataRetention != nil {
				addTableRow(retentionTable, []string{"Message Data", formatInt(int(*status.Account.MessageDataRetention))})
			}

			renderTable(retentionTable)
		}
	}

	return nil
}

func (h *tableHandler) HandleAuthSwitch(newProfile string, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	table := h.createBorderedTable()
	table.Header("Field", "Value")

	addTableRow(table, []string{"New Profile", newProfile})
	addTableRow(table, []string{"Status", "Profile switched"})

	renderTable(table)
	return nil
}

// Simple success and empty responses
func (h *tableHandler) HandleSimpleSuccess(message string) error {
	fmt.Fprintf(h.writer, "%s\n", message)
	return nil
}

func (h *tableHandler) HandleEmpty(message string) error {
	fmt.Fprintf(h.writer, "%s\n", message)
	return nil
}

// Table-specific utility functions

// createTable creates a properly configured table writer
func (h *tableHandler) createTable() *tablewriter.Table {
	return tablewriter.NewWriter(h.writer)
}

// createBorderedTable creates a table with borders for detailed views
func (h *tableHandler) createBorderedTable() *tablewriter.Table {
	table := tablewriter.NewWriter(h.writer)
	return table
}

// addTableRow adds a row to the table with proper formatting
func addTableRow(table *tablewriter.Table, row []string) {
	// Ensure all values are properly formatted and not nil
	formattedRow := make([]string, len(row))
	for i, cell := range row {
		if cell == "" {
			formattedRow[i] = "-"
		} else {
			formattedRow[i] = cell
		}
	}
	table.Append(formattedRow)
}

// renderTable renders the table with proper formatting
func renderTable(table *tablewriter.Table) {
	table.Render()
}
