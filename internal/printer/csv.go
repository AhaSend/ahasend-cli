package printer

import (
	"encoding/csv"
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// csvHandler handles CSV output formatting with complete type safety
type csvHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *csvHandler) GetFormat() string {
	return "csv"
}

// Error handling
func (h *csvHandler) HandleError(err error) error {
	fmt.Fprintf(h.writer, "Error: %v\n", err)
	// Return the original error to ensure non-zero exit code
	return err
}

// Domain responses
func (h *csvHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for the first domain to determine headers
	firstDomain := response.Data[0]
	fieldMap := map[string]string{
		"domain":            firstDomain.Domain,
		"id":                formatUUID(firstDomain.ID),
		"account_id":        formatUUID(firstDomain.AccountID),
		"dns_valid":         fmt.Sprintf("%t", firstDomain.DNSValid),
		"status":            formatDNSStatus(firstDomain.DNSValid),
		"created_at":        formatTime(firstDomain.CreatedAt),
		"updated_at":        formatTime(firstDomain.UpdatedAt),
		"last_dns_check_at": formatTimePtr(firstDomain.LastDNSCheckAt),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers
		headers = []string{"domain", "dns_valid", "created_at", "updated_at", "last_dns_check_at", "id"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, domain := range response.Data {
		domainFieldMap := map[string]string{
			"domain":            domain.Domain,
			"id":                formatUUID(domain.ID),
			"account_id":        formatUUID(domain.AccountID),
			"dns_valid":         fmt.Sprintf("%t", domain.DNSValid),
			"status":            formatDNSStatus(domain.DNSValid),
			"created_at":        formatTime(domain.CreatedAt),
			"updated_at":        formatTime(domain.UpdatedAt),
			"last_dns_check_at": formatTimePtr(domain.LastDNSCheckAt),
		}

		row := convertToCSVRow(domainFieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map
	fieldMap := map[string]string{
		"domain":            domain.Domain,
		"id":                formatUUID(domain.ID),
		"account_id":        formatUUID(domain.AccountID),
		"dns_valid":         fmt.Sprintf("%t", domain.DNSValid),
		"status":            formatDNSStatus(domain.DNSValid),
		"created_at":        formatTime(domain.CreatedAt),
		"updated_at":        formatTime(domain.UpdatedAt),
		"last_dns_check_at": formatTimePtr(domain.LastDNSCheckAt),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers for single domain
		headers = []string{"domain", "id", "account_id", "dns_valid", "created_at", "updated_at", "last_dns_check_at"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

// Message responses
func (h *csvHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for the first message to determine headers
	firstMessage := response.Data[0]
	fieldMap := map[string]string{
		"id":           formatUUID(firstMessage.ID),
		"sender":       firstMessage.Sender,
		"recipient":    firstMessage.Recipient,
		"subject":      firstMessage.Subject,
		"status":       firstMessage.Status,
		"created":      formatTime(firstMessage.CreatedAt),
		"delivered":    formatTimePtr(firstMessage.DeliveredAt),
		"opens":        formatInt(int(firstMessage.OpenCount)),
		"clicks":       formatInt(int(firstMessage.ClickCount)),
		"message_id":   firstMessage.MessageID,
		"direction":    firstMessage.Direction,
		"domain_id":    formatUUID(firstMessage.DomainID),
		"attempts":     formatInt(int(firstMessage.NumAttempts)),
		"tags":         formatStringSlice(firstMessage.Tags),
		"bounce_class": formatOptionalString(firstMessage.BounceClassification),
		"retain_until": formatTime(firstMessage.RetainUntil),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers
		headers = []string{"id", "sender", "recipient", "subject", "status", "created", "delivered", "opens", "clicks"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, message := range response.Data {
		messageFieldMap := map[string]string{
			"id":           formatUUID(message.ID),
			"sender":       message.Sender,
			"recipient":    message.Recipient,
			"subject":      message.Subject,
			"status":       message.Status,
			"created":      formatTime(message.CreatedAt),
			"delivered":    formatTimePtr(message.DeliveredAt),
			"opens":        formatInt(int(message.OpenCount)),
			"clicks":       formatInt(int(message.ClickCount)),
			"message_id":   message.MessageID,
			"direction":    message.Direction,
			"domain_id":    formatUUID(message.DomainID),
			"attempts":     formatInt(int(message.NumAttempts)),
			"tags":         formatStringSlice(message.Tags),
			"bounce_class": formatOptionalString(message.BounceClassification),
			"retain_until": formatTime(message.RetainUntil),
		}

		row := convertToCSVRow(messageFieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map
	fieldMap := map[string]string{
		"id":           formatUUID(message.ID),
		"account_id":   formatUUID(message.AccountID),
		"sender":       message.Sender,
		"recipient":    message.Recipient,
		"subject":      message.Subject,
		"status":       message.Status,
		"direction":    message.Direction,
		"created":      formatTime(message.CreatedAt),
		"updated":      formatTime(message.UpdatedAt),
		"delivered":    formatTimePtr(message.DeliveredAt),
		"opens":        formatInt(int(message.OpenCount)),
		"clicks":       formatInt(int(message.ClickCount)),
		"attempts":     formatInt(int(message.NumAttempts)),
		"bounce_class": formatOptionalString(message.BounceClassification),
		"message_id":   message.MessageID,
		"domain_id":    formatUUID(message.DomainID),
		"tags":         formatStringSlice(message.Tags),
		"retain_until": formatTime(message.RetainUntil),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers for single message
		headers = []string{"id", "account_id", "sender", "recipient", "subject", "status", "direction", "created", "updated", "delivered", "opens", "clicks", "attempts", "bounce_class", "message_id", "domain_id", "tags", "retain_until"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error {
	if response == nil || len(response.Data) == 0 {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Write headers
	headers := []string{"message_id", "recipient", "status", "error"}
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, messageData := range response.Data {
		errorMsg := ""
		if messageData.Error != nil {
			errorMsg = *messageData.Error
		}

		row := []string{
			formatOptionalString(messageData.ID),
			messageData.Recipient.Email,
			messageData.Status,
			errorMsg,
		}

		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error {
	if response == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Write headers
	headers := []string{"message_id", "success", "error"}
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := []string{
		response.MessageID,
		fmt.Sprintf("%t", response.Success),
		response.Error,
	}

	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

// Webhook responses
func (h *csvHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for the first webhook to determine headers
	firstWebhook := response.Data[0]
	fieldMap := map[string]string{
		"name":       firstWebhook.Name,
		"id":         formatUUID(firstWebhook.ID),
		"account_id": formatUUID(firstWebhook.AccountID),
		"url":        firstWebhook.URL,
		"enabled":    formatBooleanStatus(firstWebhook.Enabled),
		"events":     formatWebhookEvents(&firstWebhook),
		"secret":     formatWebhookSecret(firstWebhook.Secret),
		"scope":      firstWebhook.Scope,
		"domains":    formatStringSlice(firstWebhook.Domains),
		"created_at": formatTime(firstWebhook.CreatedAt),
		"updated_at": formatTime(firstWebhook.UpdatedAt),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers
		headers = []string{"name", "id", "url", "enabled", "events", "created_at", "updated_at"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, webhook := range response.Data {
		webhookFieldMap := map[string]string{
			"name":       webhook.Name,
			"id":         formatUUID(webhook.ID),
			"account_id": formatUUID(webhook.AccountID),
			"url":        webhook.URL,
			"enabled":    formatBooleanStatus(webhook.Enabled),
			"events":     formatWebhookEvents(&webhook),
			"secret":     formatWebhookSecret(webhook.Secret),
			"scope":      webhook.Scope,
			"domains":    formatStringSlice(webhook.Domains),
			"created_at": formatTime(webhook.CreatedAt),
			"updated_at": formatTime(webhook.UpdatedAt),
		}

		row := convertToCSVRow(webhookFieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map
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

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers for single webhook
		headers = []string{"name", "id", "account_id", "url", "enabled", "events", "secret", "scope", "domains", "created_at", "updated_at"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map - for creation, include the actual secret
	fieldMap := map[string]string{
		"name":       webhook.Name,
		"id":         formatUUID(webhook.ID),
		"account_id": formatUUID(webhook.AccountID),
		"url":        webhook.URL,
		"enabled":    formatBooleanStatus(webhook.Enabled),
		"events":     formatWebhookEvents(webhook),
		"secret":     formatWebhookSecretCreation(webhook.Secret),
		"scope":      webhook.Scope,
		"domains":    formatStringSlice(webhook.Domains),
		"created_at": formatTime(webhook.CreatedAt),
	}

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers for created webhook
		headers = []string{"name", "id", "url", "enabled", "events", "secret", "scope", "domains", "created_at"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error {
	if webhook == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map
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

	// Get headers respecting field order
	var headers []string
	if len(config.FieldOrder) > 0 {
		headers = getCSVHeaders(fieldMap, config.FieldOrder)
	} else {
		// Default headers for updated webhook
		headers = []string{"name", "id", "url", "enabled", "events", "secret", "scope", "domains", "created_at", "updated_at"}
	}

	// Write headers
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data row
	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleDeleteWebhook(success bool, config DeleteConfig) error {
	// CSV format doesn't output data for delete operations
	// Success/failure is handled via exit codes and error messages
	return nil
}

func (h *csvHandler) HandleTriggerWebhook(webhookID string, events []string, config TriggerConfig) error {
	// CSV format doesn't output data for trigger operations
	// Success/failure is handled via exit codes and error messages
	return nil
}

// Route responses
func (h *csvHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	if response == nil || len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Define default field order if not specified
	defaultFields := []string{"id", "name", "url", "enabled", "recipient", "attachments", "headers", "group_by_message_id", "strip_replies", "created_at", "updated_at"}
	fieldOrder := config.FieldOrder
	if len(fieldOrder) == 0 {
		fieldOrder = defaultFields
	}

	// Create header row
	headers := []string{}
	for _, field := range fieldOrder {
		switch field {
		case "id":
			headers = append(headers, "id")
		case "name":
			headers = append(headers, "name")
		case "url":
			headers = append(headers, "url")
		case "enabled":
			headers = append(headers, "enabled")
		case "recipient":
			headers = append(headers, "recipient")
		case "attachments":
			headers = append(headers, "attachments")
		case "headers":
			headers = append(headers, "headers")
		case "group_by_message_id":
			headers = append(headers, "group_by_message_id")
		case "strip_replies":
			headers = append(headers, "strip_replies")
		case "created_at":
			headers = append(headers, "created_at")
		case "updated_at":
			headers = append(headers, "updated_at")
		}
	}
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, route := range response.Data {
		fieldMap := map[string]string{
			"id":                  formatUUID(route.ID),
			"name":                route.Name,
			"url":                 route.URL,
			"enabled":             fmt.Sprintf("%t", route.Enabled),
			"recipient":           route.Recipient,
			"attachments":         fmt.Sprintf("%t", route.Attachments),
			"headers":             fmt.Sprintf("%t", route.Headers),
			"group_by_message_id": fmt.Sprintf("%t", route.GroupByMessageID),
			"strip_replies":       fmt.Sprintf("%t", route.StripReplies),
			"created_at":          route.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at":          route.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}

		row := convertToCSVRow(fieldMap, fieldOrder)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for single route
	fieldMap := map[string]string{
		"id":                  formatUUID(route.ID),
		"name":                route.Name,
		"url":                 route.URL,
		"enabled":             fmt.Sprintf("%t", route.Enabled),
		"recipient":           route.Recipient,
		"attachments":         fmt.Sprintf("%t", route.Attachments),
		"headers":             fmt.Sprintf("%t", route.Headers),
		"group_by_message_id": fmt.Sprintf("%t", route.GroupByMessageID),
		"strip_replies":       fmt.Sprintf("%t", route.StripReplies),
		"created_at":          route.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":          route.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, config.FieldOrder)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleCreateRoute(route *responses.Route, config CreateConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for created route
	fieldMap := map[string]string{
		"id":                  formatUUID(route.ID),
		"name":                route.Name,
		"url":                 route.URL,
		"enabled":             fmt.Sprintf("%t", route.Enabled),
		"recipient":           route.Recipient,
		"attachments":         fmt.Sprintf("%t", route.Attachments),
		"headers":             fmt.Sprintf("%t", route.Headers),
		"group_by_message_id": fmt.Sprintf("%t", route.GroupByMessageID),
		"strip_replies":       fmt.Sprintf("%t", route.StripReplies),
		"created_at":          route.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, config.FieldOrder)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleUpdateRoute(route *responses.Route, config UpdateConfig) error {
	if route == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for updated route
	fieldMap := map[string]string{
		"id":                  formatUUID(route.ID),
		"name":                route.Name,
		"url":                 route.URL,
		"enabled":             fmt.Sprintf("%t", route.Enabled),
		"recipient":           route.Recipient,
		"attachments":         fmt.Sprintf("%t", route.Attachments),
		"headers":             fmt.Sprintf("%t", route.Headers),
		"group_by_message_id": fmt.Sprintf("%t", route.GroupByMessageID),
		"strip_replies":       fmt.Sprintf("%t", route.StripReplies),
		"updated_at":          route.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, config.FieldOrder)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleDeleteRoute(success bool, config DeleteConfig) error {
	if !success {
		return nil // No CSV output for failed operations
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create simple success record
	headers := []string{"status", "action", "item_type"}
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := []string{"success", "deleted", config.ItemName}
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleTriggerRoute(routeID string, config TriggerConfig) error {
	// CSV format doesn't output data for trigger operations
	// Success/failure is handled via exit codes and error messages
	return nil
}

// Suppression responses
func (h *csvHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for the first suppression to determine headers
	firstSuppression := response.Data[0]
	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", firstSuppression.ID),
		"email":      firstSuppression.Email,
		"domain":     firstSuppression.Domain,
		"reason":     firstSuppression.Reason,
		"account_id": formatUUID(firstSuppression.AccountID),
		"created_at": formatTime(firstSuppression.CreatedAt),
		"updated_at": formatTime(firstSuppression.UpdatedAt),
		"expires_at": formatTime(firstSuppression.ExpiresAt),
	}

	// Get headers respecting field order
	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, suppression := range response.Data {
		suppressionFieldMap := map[string]string{
			"id":         fmt.Sprintf("%d", suppression.ID),
			"email":      suppression.Email,
			"domain":     suppression.Domain,
			"reason":     suppression.Reason,
			"account_id": formatUUID(suppression.AccountID),
			"created_at": formatTime(suppression.CreatedAt),
			"updated_at": formatTime(suppression.UpdatedAt),
			"expires_at": formatTime(suppression.ExpiresAt),
		}

		row := convertToCSVRow(suppressionFieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error {
	if suppression == nil {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", suppression.ID),
		"email":      suppression.Email,
		"domain":     suppression.Domain,
		"reason":     suppression.Reason,
		"account_id": formatUUID(suppression.AccountID),
		"created_at": formatTime(suppression.CreatedAt),
		"updated_at": formatTime(suppression.UpdatedAt),
		"expires_at": formatTime(suppression.ExpiresAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error {
	if response == nil || len(response.Data) == 0 {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Create field map for the first suppression to determine headers
	firstSuppression := response.Data[0]
	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", firstSuppression.ID),
		"email":      firstSuppression.Email,
		"domain":     firstSuppression.Domain,
		"reason":     firstSuppression.Reason,
		"account_id": formatUUID(firstSuppression.AccountID),
		"created_at": formatTime(firstSuppression.CreatedAt),
		"updated_at": formatTime(firstSuppression.UpdatedAt),
		"expires_at": formatTime(firstSuppression.ExpiresAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write data rows
	for _, suppression := range response.Data {
		suppressionFieldMap := map[string]string{
			"id":         fmt.Sprintf("%d", suppression.ID),
			"email":      suppression.Email,
			"domain":     suppression.Domain,
			"reason":     suppression.Reason,
			"account_id": formatUUID(suppression.AccountID),
			"created_at": formatTime(suppression.CreatedAt),
			"updated_at": formatTime(suppression.UpdatedAt),
			"expires_at": formatTime(suppression.ExpiresAt),
		}

		row := convertToCSVRow(suppressionFieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleDeleteSuppression(success bool, config DeleteConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

func (h *csvHandler) HandleWipeSuppression(count int, config WipeConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Wiped %d suppressions\n", count)
	return nil
}

func (h *csvHandler) HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error {
	if found && suppression != nil {
		writer := h.createCSVWriter()
		defer flushCSVWriter(writer)

		fieldMap := map[string]string{
			"id":         fmt.Sprintf("%d", suppression.ID),
			"email":      suppression.Email,
			"domain":     suppression.Domain,
			"reason":     suppression.Reason,
			"account_id": formatUUID(suppression.AccountID),
			"created_at": formatTime(suppression.CreatedAt),
			"updated_at": formatTime(suppression.UpdatedAt),
			"expires_at": formatTime(suppression.ExpiresAt),
		}

		headers := getCSVHeaders(fieldMap, config.FieldOrder)
		if err := writeCSVHeaders(writer, headers); err != nil {
			return err
		}

		row := convertToCSVRow(fieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}
	// For CSV, we don't output "not found" messages since CSV is for data only
	return nil
}

// SMTP responses
func (h *csvHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// First credential to establish headers
	firstCred := response.Data[0]
	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", firstCred.ID),
		"name":       firstCred.Name,
		"username":   firstCred.Username,
		"scope":      firstCred.Scope,
		"domains":    formatStringSlice(firstCred.Domains),
		"sandbox":    fmt.Sprintf("%t", firstCred.Sandbox),
		"account_id": formatUUID(firstCred.AccountID),
		"created_at": formatTime(firstCred.CreatedAt),
		"updated_at": formatTime(firstCred.UpdatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	// Write all credentials
	for _, credential := range response.Data {
		fieldMap := map[string]string{
			"id":         fmt.Sprintf("%d", credential.ID),
			"name":       credential.Name,
			"username":   credential.Username,
			"scope":      credential.Scope,
			"domains":    formatStringSlice(credential.Domains),
			"sandbox":    fmt.Sprintf("%t", credential.Sandbox),
			"account_id": formatUUID(credential.AccountID),
			"created_at": formatTime(credential.CreatedAt),
			"updated_at": formatTime(credential.UpdatedAt),
		}

		row := convertToCSVRow(fieldMap, headers)
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

func (h *csvHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		return nil // No CSV output for nil credential
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", credential.ID),
		"name":       credential.Name,
		"username":   credential.Username,
		"password":   "[HIDDEN]", // Never show password
		"scope":      credential.Scope,
		"domains":    formatStringSlice(credential.Domains),
		"sandbox":    fmt.Sprintf("%t", credential.Sandbox),
		"account_id": formatUUID(credential.AccountID),
		"created_at": formatTime(credential.CreatedAt),
		"updated_at": formatTime(credential.UpdatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error {
	if credential == nil {
		return nil
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Include password in CSV output for create operation since it's the only time it's available
	passwordValue := "[Not provided]"
	if credential.Password != "" {
		passwordValue = credential.Password
	}

	fieldMap := map[string]string{
		"id":         fmt.Sprintf("%d", credential.ID),
		"name":       credential.Name,
		"username":   credential.Username,
		"password":   passwordValue,
		"scope":      credential.Scope,
		"domains":    formatStringSlice(credential.Domains),
		"sandbox":    fmt.Sprintf("%t", credential.Sandbox),
		"account_id": formatUUID(credential.AccountID),
		"created_at": formatTime(credential.CreatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	row := convertToCSVRow(fieldMap, headers)
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleDeleteSMTP(success bool, config DeleteConfig) error {
	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	headers := []string{"operation", "item_type", "success", "status"}
	if err := writeCSVHeaders(writer, headers); err != nil {
		return err
	}

	status := "deleted"
	if !success {
		status = "failed"
	}

	row := []string{
		"delete",
		"smtp_credential",
		fmt.Sprintf("%t", success),
		status,
	}
	if err := writeCSVRow(writer, row); err != nil {
		return err
	}

	return nil
}

func (h *csvHandler) HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error {
	if result == nil {
		return nil
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	if result.TestMode {
		headers := []string{"test_mode", "success", "error"}
		if err := writeCSVHeaders(writer, headers); err != nil {
			return err
		}

		row := []string{
			"true",
			fmt.Sprintf("%t", result.Success),
			result.Error,
		}
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	} else {
		headers := []string{"success", "message_id", "error"}
		if err := writeCSVHeaders(writer, headers); err != nil {
			return err
		}

		row := []string{
			fmt.Sprintf("%t", result.Success),
			result.MessageID,
			result.Error,
		}
		if err := writeCSVRow(writer, row); err != nil {
			return err
		}
	}

	return nil
}

// API Key responses
func (h *csvHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty results
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// First API key to establish headers
	firstKey := response.Data[0]

	// Format scopes for the first key
	scopes := ""
	if len(firstKey.Scopes) > 0 {
		scopeNames := make([]string, len(firstKey.Scopes))
		for i, scope := range firstKey.Scopes {
			scopeNames[i] = scope.Scope
		}
		scopes = formatStringSlice(scopeNames)
	}

	lastUsed := ""
	if firstKey.LastUsedAt != nil {
		lastUsed = formatTime(*firstKey.LastUsedAt)
	}

	fieldMap := map[string]string{
		"id":           formatUUID(firstKey.ID),
		"label":        firstKey.Label,
		"public_key":   firstKey.PublicKey,
		"scopes":       scopes,
		"last_used_at": lastUsed,
		"account_id":   formatUUID(firstKey.AccountID),
		"created_at":   formatTime(firstKey.CreatedAt),
		"updated_at":   formatTime(firstKey.UpdatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	writeCSVHeaders(writer, headers)

	// Write all API keys
	for _, key := range response.Data {
		// Format scopes
		scopes := ""
		if len(key.Scopes) > 0 {
			scopeNames := make([]string, len(key.Scopes))
			for i, scope := range key.Scopes {
				scopeNames[i] = scope.Scope
			}
			scopes = formatStringSlice(scopeNames)
		}

		lastUsed := ""
		if key.LastUsedAt != nil {
			lastUsed = formatTime(*key.LastUsedAt)
		}

		fieldMap := map[string]string{
			"id":           formatUUID(key.ID),
			"label":        key.Label,
			"public_key":   key.PublicKey,
			"scopes":       scopes,
			"last_used_at": lastUsed,
			"account_id":   formatUUID(key.AccountID),
			"created_at":   formatTime(key.CreatedAt),
			"updated_at":   formatTime(key.UpdatedAt),
		}

		row := convertToCSVRow(fieldMap, headers)
		writeCSVRow(writer, row)
	}

	return nil
}

func (h *csvHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		return nil // No CSV output for nil key
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Format scopes
	scopes := ""
	if len(key.Scopes) > 0 {
		scopeNames := make([]string, len(key.Scopes))
		for i, scope := range key.Scopes {
			scopeNames[i] = scope.Scope
		}
		scopes = formatStringSlice(scopeNames)
	}

	lastUsed := ""
	if key.LastUsedAt != nil {
		lastUsed = formatTime(*key.LastUsedAt)
	}

	fieldMap := map[string]string{
		"id":           formatUUID(key.ID),
		"label":        key.Label,
		"public_key":   key.PublicKey,
		"secret_key":   "[HIDDEN]", // Never show secret key
		"scopes":       scopes,
		"last_used_at": lastUsed,
		"account_id":   formatUUID(key.AccountID),
		"created_at":   formatTime(key.CreatedAt),
		"updated_at":   formatTime(key.UpdatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, headers)
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error {
	if key == nil {
		return nil
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Include secret key in CSV output for create operation since it's the only time it's available
	secretKey := "[Not provided]"
	if key.SecretKey != nil && *key.SecretKey != "" {
		secretKey = *key.SecretKey
	}

	// Format scopes
	scopes := ""
	if len(key.Scopes) > 0 {
		scopeNames := make([]string, len(key.Scopes))
		for i, scope := range key.Scopes {
			scopeNames[i] = scope.Scope
		}
		scopes = formatStringSlice(scopeNames)
	}

	fieldMap := map[string]string{
		"id":         formatUUID(key.ID),
		"label":      key.Label,
		"public_key": key.PublicKey,
		"secret_key": secretKey,
		"scopes":     scopes,
		"account_id": formatUUID(key.AccountID),
		"created_at": formatTime(key.CreatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, headers)
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error {
	if key == nil {
		return nil
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Format scopes
	scopes := ""
	if len(key.Scopes) > 0 {
		scopeNames := make([]string, len(key.Scopes))
		for i, scope := range key.Scopes {
			scopeNames[i] = scope.Scope
		}
		scopes = formatStringSlice(scopeNames)
	}

	fieldMap := map[string]string{
		"id":         formatUUID(key.ID),
		"label":      key.Label,
		"public_key": key.PublicKey,
		"scopes":     scopes,
		"account_id": formatUUID(key.AccountID),
		"created_at": formatTime(key.CreatedAt),
		"updated_at": formatTime(key.UpdatedAt),
	}

	headers := getCSVHeaders(fieldMap, config.FieldOrder)
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, headers)
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleDeleteAPIKey(success bool, config DeleteConfig) error {
	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	headers := []string{"operation", "item_type", "success", "status"}
	writeCSVHeaders(writer, headers)

	status := "deleted"
	if !success {
		status = "failed"
	}

	row := []string{
		"delete",
		"api_key",
		fmt.Sprintf("%t", success),
		status,
	}
	writeCSVRow(writer, row)

	return nil
}

// Statistics responses
func (h *csvHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Define field mapping
	defaultFields := []string{
		"from_timestamp", "to_timestamp", "reception_count", "delivered_count",
		"deferred_count", "bounced_count", "failed_count", "suppressed_count",
		"opened_count", "clicked_count", "delivery_rate", "open_rate",
	}

	fieldOrder := defaultFields
	if len(config.FieldOrder) > 0 {
		fieldOrder = config.FieldOrder
	}

	// Write headers
	writeCSVHeaders(writer, fieldOrder)

	// Write data rows
	for _, stat := range response.Data {
		// Calculate rates
		deliveryRate := ""
		if stat.ReceptionCount > 0 {
			rate := (float64(stat.DeliveredCount) / float64(stat.ReceptionCount)) * 100
			deliveryRate = fmt.Sprintf("%.2f", rate)
		}

		openRate := ""
		if stat.DeliveredCount > 0 {
			rate := (float64(stat.OpenedCount) / float64(stat.DeliveredCount)) * 100
			openRate = fmt.Sprintf("%.2f", rate)
		}

		fieldMap := map[string]string{
			"from_timestamp":   formatTime(stat.FromTimestamp),
			"to_timestamp":     formatTime(stat.ToTimestamp),
			"reception_count":  formatInt(stat.ReceptionCount),
			"delivered_count":  formatInt(stat.DeliveredCount),
			"deferred_count":   formatInt(stat.DeferredCount),
			"bounced_count":    formatInt(stat.BouncedCount),
			"failed_count":     formatInt(stat.FailedCount),
			"suppressed_count": formatInt(stat.SuppressedCount),
			"opened_count":     formatInt(stat.OpenedCount),
			"clicked_count":    formatInt(stat.ClickedCount),
			"delivery_rate":    deliveryRate,
			"open_rate":        openRate,
		}

		row := convertToCSVRow(fieldMap, fieldOrder)
		writeCSVRow(writer, row)
	}

	return nil
}

func (h *csvHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Use field order or defaults for bounce data
	defaultFields := []string{"from_timestamp", "to_timestamp", "classification", "count", "percentage"}
	fieldOrder := defaultFields
	if len(config.FieldOrder) > 0 {
		fieldOrder = config.FieldOrder
	}

	// Write headers
	writeCSVHeaders(writer, fieldOrder)

	// Flatten bounce data - each classification gets its own row
	for _, stat := range response.Data {
		totalBounces := 0
		for _, bounce := range stat.Bounces {
			totalBounces += bounce.Count
		}

		for _, bounce := range stat.Bounces {
			percentage := float64(0)
			if totalBounces > 0 {
				percentage = (float64(bounce.Count) / float64(totalBounces)) * 100
			}

			fieldMap := map[string]string{
				"from_timestamp": formatTime(stat.FromTimestamp),
				"to_timestamp":   formatTime(stat.ToTimestamp),
				"classification": bounce.Classification,
				"count":          formatInt(bounce.Count),
				"percentage":     fmt.Sprintf("%.1f", percentage),
			}

			row := convertToCSVRow(fieldMap, fieldOrder)
			writeCSVRow(writer, row)
		}
	}

	return nil
}

func (h *csvHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		return nil // No CSV output for empty data
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Define field mapping - include per-domain data
	defaultFields := []string{
		"from_timestamp", "to_timestamp", "delivered_count",
		"avg_delivery_time", "recipient_domain", "domain_delivery_time",
	}

	fieldOrder := defaultFields
	if len(config.FieldOrder) > 0 {
		fieldOrder = config.FieldOrder
	}

	// Write headers
	writeCSVHeaders(writer, fieldOrder)

	// Write data rows - flatten per-domain data
	for _, stat := range response.Data {
		baseFields := map[string]string{
			"from_timestamp":    formatTime(stat.FromTimestamp),
			"to_timestamp":      formatTime(stat.ToTimestamp),
			"delivered_count":   formatInt(stat.DeliveredCount),
			"avg_delivery_time": formatFloat64(stat.AvgDeliveryTime),
		}

		if len(stat.DeliveryTimes) == 0 {
			// Just output the main statistics
			fieldMap := make(map[string]string)
			for k, v := range baseFields {
				fieldMap[k] = v
			}
			fieldMap["recipient_domain"] = ""
			fieldMap["domain_delivery_time"] = ""

			row := convertToCSVRow(fieldMap, fieldOrder)
			writeCSVRow(writer, row)
		} else {
			// Output one row per domain
			for _, dt := range stat.DeliveryTimes {
				fieldMap := make(map[string]string)
				for k, v := range baseFields {
					fieldMap[k] = v
				}

				domain := ""
				if dt.RecipientDomain != nil {
					domain = *dt.RecipientDomain
				}

				domainTime := ""
				if dt.DeliveryTime != nil {
					domainTime = formatFloat64(*dt.DeliveryTime)
				}

				fieldMap["recipient_domain"] = domain
				fieldMap["domain_delivery_time"] = domainTime

				row := convertToCSVRow(fieldMap, fieldOrder)
				writeCSVRow(writer, row)
			}
		}
	}

	return nil
}

// Auth responses
func (h *csvHandler) HandleAuthLogin(success bool, profile string, config AuthConfig) error {
	fieldMap := map[string]string{
		"profile": profile,
		"status":  "logged_in",
		"success": formatBooleanStatus(success),
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	headers := getCSVHeaders(fieldMap, []string{"profile", "status", "success"})
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, []string{"profile", "status", "success"})
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleAuthLogout(success bool, config AuthConfig) error {
	fieldMap := map[string]string{
		"status":  "logged_out",
		"success": formatBooleanStatus(success),
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	headers := getCSVHeaders(fieldMap, []string{"status", "success"})
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, []string{"status", "success"})
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleAuthStatus(status *AuthStatus, config AuthConfig) error {
	if status == nil {
		return nil // No CSV output for empty status
	}

	// Create comprehensive field map with all available data
	fieldMap := map[string]string{
		"profile": status.Profile,
		"api_key": status.APIKey,
		"valid":   formatBooleanStatus(status.Valid),
	}

	// Add account fields if available
	if status.Account != nil {
		fieldMap["account_id"] = formatUUID(status.Account.ID)
		fieldMap["account_name"] = status.Account.Name
		fieldMap["website"] = formatOptionalString(status.Account.Website)
		fieldMap["about"] = formatOptionalString(status.Account.About)
		fieldMap["created_at"] = formatTime(status.Account.CreatedAt)
		fieldMap["updated_at"] = formatTime(status.Account.UpdatedAt)

		// Email settings
		if status.Account.TrackOpens != nil {
			fieldMap["track_opens"] = formatBooleanStatus(*status.Account.TrackOpens)
		}
		if status.Account.TrackClicks != nil {
			fieldMap["track_clicks"] = formatBooleanStatus(*status.Account.TrackClicks)
		}
		if status.Account.RejectBadRecipients != nil {
			fieldMap["reject_bad_recipients"] = formatBooleanStatus(*status.Account.RejectBadRecipients)
		}
		if status.Account.RejectMistypedRecipients != nil {
			fieldMap["reject_mistyped_recipients"] = formatBooleanStatus(*status.Account.RejectMistypedRecipients)
		}

		// Data retention settings
		if status.Account.MessageMetadataRetention != nil {
			fieldMap["message_metadata_retention"] = formatInt(int(*status.Account.MessageMetadataRetention))
		}
		if status.Account.MessageDataRetention != nil {
			fieldMap["message_data_retention"] = formatInt(int(*status.Account.MessageDataRetention))
		}
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	// Use a predefined field order for consistency
	fieldOrder := []string{
		"profile", "api_key", "valid", "account_id", "account_name",
		"website", "about", "created_at", "updated_at", "track_opens",
		"track_clicks", "reject_bad_recipients", "reject_mistyped_recipients",
		"message_metadata_retention", "message_data_retention",
	}

	headers := getCSVHeaders(fieldMap, fieldOrder)
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, fieldOrder)
	writeCSVRow(writer, row)

	return nil
}

func (h *csvHandler) HandleAuthSwitch(newProfile string, config AuthConfig) error {
	fieldMap := map[string]string{
		"new_profile": newProfile,
		"status":      "profile_switched",
		"operation":   "switch",
	}

	writer := h.createCSVWriter()
	defer flushCSVWriter(writer)

	headers := getCSVHeaders(fieldMap, []string{"new_profile", "status", "operation"})
	writeCSVHeaders(writer, headers)

	row := convertToCSVRow(fieldMap, []string{"new_profile", "status", "operation"})
	writeCSVRow(writer, row)

	return nil
}

// Simple success and empty responses
func (h *csvHandler) HandleSimpleSuccess(message string) error {
	// CSV format doesn't typically output success messages
	return nil
}

func (h *csvHandler) HandleEmpty(message string) error {
	// CSV format doesn't output empty messages
	return nil
}

// CSV-specific utility functions

// createCSVWriter creates and configures a CSV writer
func (h *csvHandler) createCSVWriter() *csv.Writer {
	writer := csv.NewWriter(h.writer)
	writer.Comma = ','
	return writer
}

// writeCSVHeaders writes the header row for CSV output
func writeCSVHeaders(writer *csv.Writer, headers []string) error {
	return writer.Write(headers)
}

// writeCSVRow writes a data row to CSV with proper escaping
func writeCSVRow(writer *csv.Writer, row []string) error {
	// Ensure all values are properly formatted
	formattedRow := make([]string, len(row))
	for i, cell := range row {
		formattedRow[i] = escapeCSVField(cell)
	}
	return writer.Write(formattedRow)
}

// escapeCSVField ensures proper CSV escaping for field values
func escapeCSVField(field string) string {
	// The csv.Writer handles most escaping, but we can clean up the data
	if field == "" {
		return ""
	}

	// Clean up any existing quotes or special characters
	field = strings.ReplaceAll(field, "\r", "")
	field = strings.ReplaceAll(field, "\n", " ")
	field = strings.ReplaceAll(field, "\t", " ")

	return field
}

// flushCSVWriter ensures all data is written to the output
func flushCSVWriter(writer *csv.Writer) {
	writer.Flush()
}

// convertToCSVRow converts a map of field values to an ordered CSV row
func convertToCSVRow(fieldMap map[string]string, fieldOrder []string) []string {
	row := make([]string, 0, len(fieldMap))

	if len(fieldOrder) == 0 {
		// No specific order - use map iteration order
		for _, value := range fieldMap {
			row = append(row, value)
		}
		return row
	}

	// Use specified field order
	for _, fieldName := range fieldOrder {
		if value, exists := fieldMap[fieldName]; exists {
			row = append(row, value)
		} else {
			row = append(row, "")
		}
	}

	return row
}

// getCSVHeaders returns the header row for CSV based on field order
func getCSVHeaders(fieldMap map[string]string, fieldOrder []string) []string {
	if len(fieldOrder) == 0 {
		// No specific order - extract keys from map
		headers := make([]string, 0, len(fieldMap))
		for key := range fieldMap {
			headers = append(headers, key)
		}
		return headers
	}

	// Use specified field order
	return fieldOrder
}
