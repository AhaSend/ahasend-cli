package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// plainHandler handles plain text output formatting with complete type safety
type plainHandler struct {
	handlerBase
}

// GetFormat returns the format name
func (h *plainHandler) GetFormat() string {
	return "plain"
}

// Error handling
func (h *plainHandler) HandleError(err error) error {
	fmt.Fprintf(h.writer, "Error: %v\n", err)
	// Return the original error to ensure non-zero exit code
	return err
}

// Domain responses
func (h *plainHandler) HandleDomainList(response *responses.PaginatedDomainsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, domain := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Domain: %s\n", domain.Domain)
		fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(domain.ID))
		fmt.Fprintf(h.writer, "  Status: %s\n", formatDNSStatus(domain.DNSValid))
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(domain.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(domain.UpdatedAt))
		if domain.LastDNSCheckAt != nil {
			fmt.Fprintf(h.writer, "  Last DNS Check: %s\n", formatTimePtr(domain.LastDNSCheckAt))
		}
	}

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

func (h *plainHandler) HandleSingleDomain(domain *responses.Domain, config SingleConfig) error {
	if domain == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Domain: %s\n", domain.Domain)
	fmt.Fprintf(h.writer, "ID: %s\n", formatUUID(domain.ID))
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(domain.AccountID))
	fmt.Fprintf(h.writer, "DNS Status: %s\n", formatDNSStatus(domain.DNSValid))
	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(domain.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(domain.UpdatedAt))

	if domain.LastDNSCheckAt != nil {
		fmt.Fprintf(h.writer, "Last DNS Check: %s\n", formatTimePtr(domain.LastDNSCheckAt))
	} else {
		fmt.Fprintf(h.writer, "Last DNS Check: Never\n")
	}

	// Show DNS records if any
	if len(domain.DNSRecords) > 0 {
		fmt.Fprintf(h.writer, "\nDNS Records:\n")
		for i, record := range domain.DNSRecords {
			fmt.Fprintf(h.writer, "  %d. Type: %s, Host: %s, Content: %s, Required: %s, Propagated: %s\n",
				i+1, record.Type, record.Host, record.Content,
				formatBooleanStatus(record.Required), formatBooleanStatus(record.Propagated))
		}
	}

	return nil
}

// Message responses
func (h *plainHandler) HandleMessageList(response *responses.PaginatedMessagesResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, message := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Message: %s\n", formatUUID(message.ID))
		fmt.Fprintf(h.writer, "  From: %s\n", message.Sender)
		fmt.Fprintf(h.writer, "  To: %s\n", message.Recipient)
		fmt.Fprintf(h.writer, "  Subject: %s\n", message.Subject)
		fmt.Fprintf(h.writer, "  Status: %s\n", message.Status)
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(message.CreatedAt))
		if message.DeliveredAt != nil {
			fmt.Fprintf(h.writer, "  Delivered: %s\n", formatTimePtr(message.DeliveredAt))
		}
		fmt.Fprintf(h.writer, "  Opens: %d\n", message.OpenCount)
		fmt.Fprintf(h.writer, "  Clicks: %d\n", message.ClickCount)
	}

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

func (h *plainHandler) HandleSingleMessage(message *responses.Message, config SingleConfig) error {
	if message == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "ID: %s\n", formatUUID(message.ID))
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(message.AccountID))
	fmt.Fprintf(h.writer, "From: %s\n", message.Sender)
	fmt.Fprintf(h.writer, "To: %s\n", message.Recipient)
	fmt.Fprintf(h.writer, "Subject: %s\n", message.Subject)
	fmt.Fprintf(h.writer, "Status: %s\n", message.Status)
	fmt.Fprintf(h.writer, "Direction: %s\n", message.Direction)
	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(message.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(message.UpdatedAt))

	if message.DeliveredAt != nil {
		fmt.Fprintf(h.writer, "Delivered: %s\n", formatTimePtr(message.DeliveredAt))
	} else {
		fmt.Fprintf(h.writer, "Delivered: Not yet\n")
	}

	fmt.Fprintf(h.writer, "Opens: %d\n", message.OpenCount)
	fmt.Fprintf(h.writer, "Clicks: %d\n", message.ClickCount)
	fmt.Fprintf(h.writer, "Attempts: %d\n", message.NumAttempts)
	if message.BounceClassification != nil {
		fmt.Fprintf(h.writer, "Bounce Class: %s\n", formatOptionalString(message.BounceClassification))
	}
	fmt.Fprintf(h.writer, "Message ID: %s\n", message.MessageID)
	fmt.Fprintf(h.writer, "Domain ID: %s\n", formatUUID(message.DomainID))
	if len(message.Tags) > 0 {
		fmt.Fprintf(h.writer, "Tags: %s\n", formatStringSlice(message.Tags))
	}
	fmt.Fprintf(h.writer, "Retain Until: %s\n", formatTime(message.RetainUntil))

	return nil
}

func (h *plainHandler) HandleCreateMessage(response *responses.CreateMessageResponse, config CreateConfig) error {
	if response == nil {
		fmt.Fprintf(h.writer, "No message data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	// Show summary first
	fmt.Fprintf(h.writer, "Successfully sent %d messages\n", len(response.Data))

	// Show details for each message
	for i, messageData := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Message %d:\n", i+1)
		fmt.Fprintf(h.writer, "  ID: %s\n", formatOptionalString(messageData.ID))
		fmt.Fprintf(h.writer, "  Recipient: %s\n", messageData.Recipient.Email)
		fmt.Fprintf(h.writer, "  Status: %s\n", messageData.Status)
		if messageData.Error != nil {
			fmt.Fprintf(h.writer, "  Error: %s\n", *messageData.Error)
		}
	}

	return nil
}

func (h *plainHandler) HandleCancelMessage(response *CancelMessageResponse, config SimpleConfig) error {
	if response == nil {
		fmt.Fprintf(h.writer, "No cancellation data received\n")
		return nil
	}

	if response.Success {
		fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
		fmt.Fprintf(h.writer, "Message ID: %s\n", response.MessageID)
	} else {
		fmt.Fprintf(h.writer, "Message cancellation failed\n")
		fmt.Fprintf(h.writer, "Message ID: %s\n", response.MessageID)
		if response.Error != "" {
			fmt.Fprintf(h.writer, "Error: %s\n", response.Error)
		}
	}

	return nil
}

// Webhook responses
func (h *plainHandler) HandleWebhookList(response *responses.PaginatedWebhooksResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, webhook := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Webhook: %s\n", webhook.Name)
		fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(webhook.ID))
		fmt.Fprintf(h.writer, "  URL: %s\n", webhook.URL)
		fmt.Fprintf(h.writer, "  Enabled: %s\n", formatBooleanStatus(webhook.Enabled))
		fmt.Fprintf(h.writer, "  Scope: %s\n", webhook.Scope)
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(webhook.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(webhook.UpdatedAt))
		fmt.Fprintf(h.writer, "  Success Count: %d\n", webhook.SuccessCount)
		fmt.Fprintf(h.writer, "  Error Count: %d\n", webhook.ErrorCount)
		if webhook.LastRequestAt != nil {
			fmt.Fprintf(h.writer, "  Last Request: %s\n", formatTimePtr(webhook.LastRequestAt))
		}
		if len(webhook.Domains) > 0 {
			fmt.Fprintf(h.writer, "  Domains: %s\n", formatStringSlice(webhook.Domains))
		}
	}

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

func (h *plainHandler) HandleSingleWebhook(webhook *responses.Webhook, config SingleConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Webhook ID: %s\n", formatUUID(webhook.ID))
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(webhook.AccountID))
	fmt.Fprintf(h.writer, "Name: %s\n", webhook.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", webhook.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(webhook.Enabled))
	fmt.Fprintf(h.writer, "Scope: %s\n", webhook.Scope)
	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(webhook.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(webhook.UpdatedAt))

	// Statistics
	fmt.Fprintf(h.writer, "Success Count: %d\n", webhook.SuccessCount)
	fmt.Fprintf(h.writer, "Error Count: %d\n", webhook.ErrorCount)
	fmt.Fprintf(h.writer, "Errors Since Last Success: %d\n", webhook.ErrorsSinceLastSuccess)
	if webhook.LastRequestAt != nil {
		fmt.Fprintf(h.writer, "Last Request: %s\n", formatTimePtr(webhook.LastRequestAt))
	} else {
		fmt.Fprintf(h.writer, "Last Request: Never\n")
	}

	// Event subscriptions
	fmt.Fprintf(h.writer, "\nEvent Subscriptions:\n")
	fmt.Fprintf(h.writer, "  Reception: %s\n", formatBooleanStatus(webhook.OnReception))
	fmt.Fprintf(h.writer, "  Delivered: %s\n", formatBooleanStatus(webhook.OnDelivered))
	fmt.Fprintf(h.writer, "  Transient Error: %s\n", formatBooleanStatus(webhook.OnTransientError))
	fmt.Fprintf(h.writer, "  Failed: %s\n", formatBooleanStatus(webhook.OnFailed))
	fmt.Fprintf(h.writer, "  Bounced: %s\n", formatBooleanStatus(webhook.OnBounced))
	fmt.Fprintf(h.writer, "  Suppressed: %s\n", formatBooleanStatus(webhook.OnSuppressed))
	fmt.Fprintf(h.writer, "  Opened: %s\n", formatBooleanStatus(webhook.OnOpened))
	fmt.Fprintf(h.writer, "  Clicked: %s\n", formatBooleanStatus(webhook.OnClicked))
	fmt.Fprintf(h.writer, "  Suppression Created: %s\n", formatBooleanStatus(webhook.OnSuppressionCreated))
	fmt.Fprintf(h.writer, "  DNS Error: %s\n", formatBooleanStatus(webhook.OnDNSError))

	// Domain restrictions
	if len(webhook.Domains) > 0 {
		fmt.Fprintf(h.writer, "\nRestricted Domains: %s\n", formatStringSlice(webhook.Domains))
	} else {
		fmt.Fprintf(h.writer, "\nRestricted Domains: All domains\n")
	}

	// Security info (don't show actual secret)
	if webhook.Secret != "" {
		fmt.Fprintf(h.writer, "Secret: Configured\n")
	} else {
		fmt.Fprintf(h.writer, "Secret: Not configured\n")
	}

	return nil
}

func (h *plainHandler) HandleCreateWebhook(webhook *responses.Webhook, config CreateConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "No webhook data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Webhook ID: %s\n", formatUUID(webhook.ID))
	fmt.Fprintf(h.writer, "Name: %s\n", webhook.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", webhook.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(webhook.Enabled))
	fmt.Fprintf(h.writer, "Scope: %s\n", webhook.Scope)

	// Show configured events
	events := []string{}
	if webhook.OnReception {
		events = append(events, "reception")
	}
	if webhook.OnDelivered {
		events = append(events, "delivered")
	}
	if webhook.OnTransientError {
		events = append(events, "transient_error")
	}
	if webhook.OnFailed {
		events = append(events, "failed")
	}
	if webhook.OnBounced {
		events = append(events, "bounced")
	}
	if webhook.OnSuppressed {
		events = append(events, "suppressed")
	}
	if webhook.OnOpened {
		events = append(events, "opened")
	}
	if webhook.OnClicked {
		events = append(events, "clicked")
	}
	if webhook.OnSuppressionCreated {
		events = append(events, "suppression_created")
	}
	if webhook.OnDNSError {
		events = append(events, "dns_error")
	}

	if len(events) > 0 {
		fmt.Fprintf(h.writer, "Events: %s\n", formatStringSlice(events))
	} else {
		fmt.Fprintf(h.writer, "Events: None configured\n")
	}

	if len(webhook.Domains) > 0 {
		fmt.Fprintf(h.writer, "Domains: %s\n", formatStringSlice(webhook.Domains))
	}

	// Security info (show actual secret for creation)
	if webhook.Secret != "" {
		fmt.Fprintf(h.writer, "Secret: %s\n", webhook.Secret)
		fmt.Fprintf(h.writer, "\n⚠️  Store this secret securely. It will not be shown again.\n")
	}

	return nil
}

func (h *plainHandler) HandleUpdateWebhook(webhook *responses.Webhook, config UpdateConfig) error {
	if webhook == nil {
		fmt.Fprintf(h.writer, "No webhook data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Webhook ID: %s\n", formatUUID(webhook.ID))
	fmt.Fprintf(h.writer, "Name: %s\n", webhook.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", webhook.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(webhook.Enabled))
	fmt.Fprintf(h.writer, "Scope: %s\n", webhook.Scope)
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(webhook.UpdatedAt))

	// Show configured events
	events := []string{}
	if webhook.OnReception {
		events = append(events, "reception")
	}
	if webhook.OnDelivered {
		events = append(events, "delivered")
	}
	if webhook.OnTransientError {
		events = append(events, "transient_error")
	}
	if webhook.OnFailed {
		events = append(events, "failed")
	}
	if webhook.OnBounced {
		events = append(events, "bounced")
	}
	if webhook.OnSuppressed {
		events = append(events, "suppressed")
	}
	if webhook.OnOpened {
		events = append(events, "opened")
	}
	if webhook.OnClicked {
		events = append(events, "clicked")
	}
	if webhook.OnSuppressionCreated {
		events = append(events, "suppression_created")
	}
	if webhook.OnDNSError {
		events = append(events, "dns_error")
	}

	if len(events) > 0 {
		fmt.Fprintf(h.writer, "Events: %s\n", formatStringSlice(events))
	} else {
		fmt.Fprintf(h.writer, "Events: None configured\n")
	}

	if len(webhook.Domains) > 0 {
		fmt.Fprintf(h.writer, "Domains: %s\n", formatStringSlice(webhook.Domains))
	}

	return nil
}

func (h *plainHandler) HandleDeleteWebhook(success bool, config DeleteConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

func (h *plainHandler) HandleTriggerWebhook(webhookID string, events []string, config TriggerConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

// Route responses
func (h *plainHandler) HandleRouteList(response *responses.PaginatedRoutesResponse, config ListConfig) error {
	if response == nil || len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for _, route := range response.Data {
		fmt.Fprintf(h.writer, "Route ID: %s\n", formatUUID(route.ID))
		fmt.Fprintf(h.writer, "  Name: %s\n", route.Name)
		fmt.Fprintf(h.writer, "  URL: %s\n", route.URL)
		fmt.Fprintf(h.writer, "  Enabled: %s\n", formatBooleanStatus(route.Enabled))
		fmt.Fprintf(h.writer, "  Recipient Filter: %s\n", route.Recipient)
		fmt.Fprintf(h.writer, "  Include Attachments: %s\n", formatBooleanStatus(route.Attachments))
		fmt.Fprintf(h.writer, "  Include Headers: %s\n", formatBooleanStatus(route.Headers))
		fmt.Fprintf(h.writer, "  Group by Message ID: %s\n", formatBooleanStatus(route.GroupByMessageID))
		fmt.Fprintf(h.writer, "  Strip Replies: %s\n", formatBooleanStatus(route.StripReplies))
		fmt.Fprintf(h.writer, "  Successful calls: %d\n", route.SuccessCount)
		fmt.Fprintf(h.writer, "  Unsuccessful calls: %d\n", route.ErrorCount)
		fmt.Fprintf(h.writer, "  Errors since last success: %d\n", route.ErrorsSinceLastSuccess)
		if route.LastRequestAt != nil {
			fmt.Fprintf(h.writer, "  Last called at: %s\n", formatTime(*route.LastRequestAt))
		} else {
			fmt.Fprintf(h.writer, "  Last called at: Never\n")
		}
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(route.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(route.UpdatedAt))
		fmt.Fprintf(h.writer, "\n")
	}

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

func (h *plainHandler) HandleSingleRoute(route *responses.Route, config SingleConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Route ID: %s\n", formatUUID(route.ID))
	fmt.Fprintf(h.writer, "Name: %s\n", route.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", route.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(route.Enabled))
	fmt.Fprintf(h.writer, "Recipient Filter: %s\n", route.Recipient)

	fmt.Fprintf(h.writer, "\nProcessing Options:\n")
	fmt.Fprintf(h.writer, "  Include Attachments: %s\n", formatBooleanStatus(route.Attachments))
	fmt.Fprintf(h.writer, "  Include Headers: %s\n", formatBooleanStatus(route.Headers))
	fmt.Fprintf(h.writer, "  Group by Message ID: %s\n", formatBooleanStatus(route.GroupByMessageID))
	fmt.Fprintf(h.writer, "  Strip Replies: %s\n", formatBooleanStatus(route.StripReplies))

	fmt.Fprintf(h.writer, "\nTimestamps:\n")
	fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(route.CreatedAt))
	fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(route.UpdatedAt))

	return nil
}

func (h *plainHandler) HandleCreateRoute(route *responses.Route, config CreateConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "No route data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Route ID: %s\n", formatUUID(route.ID))
	fmt.Fprintf(h.writer, "Name: %s\n", route.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", route.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(route.Enabled))
	fmt.Fprintf(h.writer, "Recipient Filter: %s\n", route.Recipient)

	fmt.Fprintf(h.writer, "\nProcessing Configuration:\n")
	fmt.Fprintf(h.writer, "  Include Attachments: %s\n", formatBooleanStatus(route.Attachments))
	fmt.Fprintf(h.writer, "  Include Headers: %s\n", formatBooleanStatus(route.Headers))
	fmt.Fprintf(h.writer, "  Group by Message ID: %s\n", formatBooleanStatus(route.GroupByMessageID))
	fmt.Fprintf(h.writer, "  Strip Replies: %s\n", formatBooleanStatus(route.StripReplies))

	return nil
}

func (h *plainHandler) HandleUpdateRoute(route *responses.Route, config UpdateConfig) error {
	if route == nil {
		fmt.Fprintf(h.writer, "No route data received\n")
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Updated Route ID: %s\n", formatUUID(route.ID))
	fmt.Fprintf(h.writer, "Name: %s\n", route.Name)
	fmt.Fprintf(h.writer, "URL: %s\n", route.URL)
	fmt.Fprintf(h.writer, "Enabled: %s\n", formatBooleanStatus(route.Enabled))
	fmt.Fprintf(h.writer, "Recipient Filter: %s\n", route.Recipient)

	fmt.Fprintf(h.writer, "\nCurrent Configuration:\n")
	fmt.Fprintf(h.writer, "  Include Attachments: %s\n", formatBooleanStatus(route.Attachments))
	fmt.Fprintf(h.writer, "  Include Headers: %s\n", formatBooleanStatus(route.Headers))
	fmt.Fprintf(h.writer, "  Group by Message ID: %s\n", formatBooleanStatus(route.GroupByMessageID))
	fmt.Fprintf(h.writer, "  Strip Replies: %s\n", formatBooleanStatus(route.StripReplies))

	fmt.Fprintf(h.writer, "\nLast Updated: %s\n", formatTime(route.UpdatedAt))

	return nil
}

func (h *plainHandler) HandleDeleteRoute(success bool, config DeleteConfig) error {
	if !success {
		fmt.Fprintf(h.writer, "Failed to delete %s\n", config.ItemName)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

// Suppression responses
func (h *plainHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, suppression := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Email: %s\n", suppression.Email)
		fmt.Fprintf(h.writer, "  ID: %d\n", suppression.ID)
		if suppression.Domain != "" {
			fmt.Fprintf(h.writer, "  Domain: %s\n", suppression.Domain)
		}
		if suppression.Reason != "" {
			fmt.Fprintf(h.writer, "  Reason: %s\n", suppression.Reason)
		}
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(suppression.CreatedAt))
		fmt.Fprintf(h.writer, "  Expires: %s\n", formatTime(suppression.ExpiresAt))
	}

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\n")
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, "More suppressions available\n")
		} else {
			fmt.Fprintf(h.writer, "No more suppressions\n")
		}
	}

	return nil
}

func (h *plainHandler) HandleSingleSuppression(suppression *responses.Suppression, config SingleConfig) error {
	if suppression == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Email: %s\n", suppression.Email)
	fmt.Fprintf(h.writer, "ID: %d\n", suppression.ID)
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(suppression.AccountID))
	if suppression.Domain != "" {
		fmt.Fprintf(h.writer, "Domain: %s\n", suppression.Domain)
	}
	if suppression.Reason != "" {
		fmt.Fprintf(h.writer, "Reason: %s\n", suppression.Reason)
	}
	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(suppression.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(suppression.UpdatedAt))
	fmt.Fprintf(h.writer, "Expires: %s\n", formatTime(suppression.ExpiresAt))
	return nil
}

func (h *plainHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config CreateConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty("No suppressions created")
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, suppression := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Email: %s\n", suppression.Email)
		fmt.Fprintf(h.writer, "  ID: %d\n", suppression.ID)
		if suppression.Domain != "" {
			fmt.Fprintf(h.writer, "  Domain: %s\n", suppression.Domain)
		}
		if suppression.Reason != "" {
			fmt.Fprintf(h.writer, "  Reason: %s\n", suppression.Reason)
		}
		fmt.Fprintf(h.writer, "  Expires: %s\n", formatTime(suppression.ExpiresAt))
	}

	return nil
}

func (h *plainHandler) HandleDeleteSuppression(success bool, config DeleteConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

func (h *plainHandler) HandleWipeSuppression(count int, config WipeConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Wiped %d suppressions\n", count)
	return nil
}

func (h *plainHandler) HandleCheckSuppression(suppression *responses.Suppression, found bool, config CheckConfig) error {
	if found {
		fmt.Fprintf(h.writer, "%s\n", config.FoundMessage)
		if suppression != nil {
			fmt.Fprintf(h.writer, "Email: %s\n", suppression.Email)
			fmt.Fprintf(h.writer, "ID: %d\n", suppression.ID)
			if suppression.Domain != "" {
				fmt.Fprintf(h.writer, "Domain: %s\n", suppression.Domain)
			}
			if suppression.Reason != "" {
				fmt.Fprintf(h.writer, "Reason: %s\n", suppression.Reason)
			}
			fmt.Fprintf(h.writer, "Created: %s\n", formatTime(suppression.CreatedAt))
			fmt.Fprintf(h.writer, "Expires: %s\n", formatTime(suppression.ExpiresAt))
		}
	} else {
		fmt.Fprintf(h.writer, "%s\n", config.NotFoundMessage)
	}
	return nil
}

// SMTP responses
func (h *plainHandler) HandleSMTPList(response *responses.PaginatedSMTPCredentialsResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, credential := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Name: %s\n", credential.Name)
		fmt.Fprintf(h.writer, "  ID: %d\n", credential.ID)
		fmt.Fprintf(h.writer, "  Username: %s\n", credential.Username)
		fmt.Fprintf(h.writer, "  Scope: %s\n", credential.Scope)
		if len(credential.Domains) > 0 {
			fmt.Fprintf(h.writer, "  Domains: %s\n", formatStringSlice(credential.Domains))
		}
		fmt.Fprintf(h.writer, "  Sandbox: %s\n", formatBooleanStatus(credential.Sandbox))
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(credential.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(credential.UpdatedAt))
	}

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\n")
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, "More SMTP credentials available\n")
		} else {
			fmt.Fprintf(h.writer, "No more SMTP credentials\n")
		}
	}

	return nil
}

func (h *plainHandler) HandleSingleSMTP(credential *responses.SMTPCredential, config SingleConfig) error {
	if credential == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Name: %s\n", credential.Name)
	fmt.Fprintf(h.writer, "ID: %d\n", credential.ID)
	fmt.Fprintf(h.writer, "Username: %s\n", credential.Username)
	fmt.Fprintf(h.writer, "Password: %s\n", "[HIDDEN]") // Never show password
	fmt.Fprintf(h.writer, "Scope: %s\n", credential.Scope)
	if len(credential.Domains) > 0 {
		fmt.Fprintf(h.writer, "Domains: %s\n", formatStringSlice(credential.Domains))
	}
	fmt.Fprintf(h.writer, "Sandbox: %s\n", formatBooleanStatus(credential.Sandbox))
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(credential.AccountID))
	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(credential.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(credential.UpdatedAt))

	// Show SMTP connection settings
	fmt.Fprintf(h.writer, "\nSMTP Settings:\n")
	fmt.Fprintf(h.writer, "  Server: send.ahasend.com\n")
	fmt.Fprintf(h.writer, "  Port: 587 (STARTTLS) or 465 (SSL/TLS)\n")
	fmt.Fprintf(h.writer, "  Username: %s\n", credential.Username)
	fmt.Fprintf(h.writer, "  Password: [Use the password provided during creation]\n")

	return nil
}

func (h *plainHandler) HandleCreateSMTP(credential *responses.SMTPCredential, config CreateConfig) error {
	if credential == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Created SMTP Credential:\n")
	fmt.Fprintf(h.writer, "  Name: %s\n", credential.Name)
	fmt.Fprintf(h.writer, "  ID: %d\n", credential.ID)
	fmt.Fprintf(h.writer, "  Username: %s\n", credential.Username)
	// Show password only on creation
	if credential.Password != "" {
		fmt.Fprintf(h.writer, "  Password: %s\n", credential.Password)
		fmt.Fprintf(h.writer, "\n⚠️  IMPORTANT: Save this password now! It won't be shown again.\n")
	}
	fmt.Fprintf(h.writer, "  Scope: %s\n", credential.Scope)
	if len(credential.Domains) > 0 {
		fmt.Fprintf(h.writer, "  Domains: %s\n", formatStringSlice(credential.Domains))
	}
	fmt.Fprintf(h.writer, "  Sandbox: %s\n", formatBooleanStatus(credential.Sandbox))
	fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(credential.CreatedAt))

	// Show SMTP connection settings
	fmt.Fprintf(h.writer, "\nSMTP Settings:\n")
	fmt.Fprintf(h.writer, "  Server: send.ahasend.com\n")
	fmt.Fprintf(h.writer, "  Port: 587 (STARTTLS) or 465 (SSL/TLS)\n")
	fmt.Fprintf(h.writer, "  Username: %s\n", credential.Username)
	fmt.Fprintf(h.writer, "  Password: [Use the password shown above]\n")

	return nil
}

func (h *plainHandler) HandleDeleteSMTP(success bool, config DeleteConfig) error {
	if success {
		fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "Failed to delete %s\n", config.ItemName)
	}
	return nil
}

func (h *plainHandler) HandleSMTPSend(result *SMTPSendResult, config SMTPSendConfig) error {
	if result == nil {
		return h.HandleEmpty("No send result")
	}

	if result.TestMode {
		fmt.Fprintf(h.writer, "SMTP Test Result:\n")
		if result.Success {
			fmt.Fprintf(h.writer, "  Status: ✓ Connection successful\n")
			fmt.Fprintf(h.writer, "  Message: SMTP authentication and connection test passed\n")
		} else {
			fmt.Fprintf(h.writer, "  Status: ✗ Connection failed\n")
			if result.Error != "" {
				fmt.Fprintf(h.writer, "  Error: %s\n", result.Error)
			}
		}
	} else {
		if result.Success {
			fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
			if result.MessageID != "" {
				fmt.Fprintf(h.writer, "Message ID: %s\n", result.MessageID)
			}
		} else {
			fmt.Fprintf(h.writer, "Failed to send message\n")
			if result.Error != "" {
				fmt.Fprintf(h.writer, "Error: %s\n", result.Error)
			}
		}
	}

	return nil
}

// API Key responses
func (h *plainHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)

	for i, key := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Label: %s\n", key.Label)
		fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(key.ID))
		fmt.Fprintf(h.writer, "  Public Key: %s\n", key.PublicKey)
		if len(key.Scopes) > 0 {
			fmt.Fprintf(h.writer, "  Scopes: ")
			for j, scope := range key.Scopes {
				if j > 0 {
					fmt.Fprintf(h.writer, ", ")
				}
				fmt.Fprintf(h.writer, "%s", scope.Scope)
			}
			fmt.Fprintf(h.writer, "\n")
		}
		if key.LastUsedAt != nil {
			fmt.Fprintf(h.writer, "  Last Used: %s\n", formatTime(*key.LastUsedAt))
		} else {
			fmt.Fprintf(h.writer, "  Last Used: Never\n")
		}
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(key.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(key.UpdatedAt))
	}

	// Show pagination info if enabled
	if config.ShowPagination {
		fmt.Fprintf(h.writer, "\n")
		if response.Pagination.HasMore {
			fmt.Fprintf(h.writer, "More API keys available\n")
		} else {
			fmt.Fprintf(h.writer, "No more API keys\n")
		}
	}

	return nil
}

func (h *plainHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		fmt.Fprintf(h.writer, "%s\n", config.EmptyMessage)
		return nil
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Label: %s\n", key.Label)
	fmt.Fprintf(h.writer, "ID: %s\n", formatUUID(key.ID))
	fmt.Fprintf(h.writer, "Public Key: %s\n", key.PublicKey)
	fmt.Fprintf(h.writer, "Secret Key: %s\n", "[HIDDEN - Never displayed for security]")
	fmt.Fprintf(h.writer, "Account ID: %s\n", formatUUID(key.AccountID))

	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "Scopes:\n")
		for _, scope := range key.Scopes {
			fmt.Fprintf(h.writer, "  - %s", scope.Scope)
			if scope.DomainID != nil {
				fmt.Fprintf(h.writer, " (Domain: %s)", formatUUID(*scope.DomainID))
			}
			fmt.Fprintf(h.writer, "\n")
		}
	} else {
		fmt.Fprintf(h.writer, "Scopes: None\n")
	}

	if key.LastUsedAt != nil {
		fmt.Fprintf(h.writer, "Last Used: %s\n", formatTime(*key.LastUsedAt))
	} else {
		fmt.Fprintf(h.writer, "Last Used: Never\n")
	}

	fmt.Fprintf(h.writer, "Created: %s\n", formatTime(key.CreatedAt))
	fmt.Fprintf(h.writer, "Updated: %s\n", formatTime(key.UpdatedAt))

	return nil
}

func (h *plainHandler) HandleCreateAPIKey(key *responses.APIKey, config CreateConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Created API Key:\n")
	fmt.Fprintf(h.writer, "  Label: %s\n", key.Label)
	fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(key.ID))
	fmt.Fprintf(h.writer, "  Public Key: %s\n", key.PublicKey)

	// Show secret key only on creation
	if key.SecretKey != nil && *key.SecretKey != "" {
		fmt.Fprintf(h.writer, "  Secret Key: %s\n", *key.SecretKey)
		fmt.Fprintf(h.writer, "\n⚠️  IMPORTANT: Save this secret key now! It won't be shown again.\n")
	} else {
		fmt.Fprintf(h.writer, "  Secret Key: [Not provided in response]\n")
	}

	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "  Scopes:\n")
		for _, scope := range key.Scopes {
			fmt.Fprintf(h.writer, "    - %s", scope.Scope)
			if scope.DomainID != nil {
				fmt.Fprintf(h.writer, " (Domain: %s)", formatUUID(*scope.DomainID))
			}
			fmt.Fprintf(h.writer, "\n")
		}
	} else {
		fmt.Fprintf(h.writer, "  Scopes: None\n")
	}

	fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(key.CreatedAt))

	return nil
}

func (h *plainHandler) HandleUpdateAPIKey(key *responses.APIKey, config UpdateConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}

	fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage)

	fmt.Fprintf(h.writer, "Updated API Key:\n")
	fmt.Fprintf(h.writer, "  Label: %s\n", key.Label)
	fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(key.ID))
	fmt.Fprintf(h.writer, "  Public Key: %s\n", key.PublicKey)

	if len(key.Scopes) > 0 {
		fmt.Fprintf(h.writer, "  Scopes:\n")
		for _, scope := range key.Scopes {
			fmt.Fprintf(h.writer, "    - %s", scope.Scope)
			if scope.DomainID != nil {
				fmt.Fprintf(h.writer, " (Domain: %s)", formatUUID(*scope.DomainID))
			}
			fmt.Fprintf(h.writer, "\n")
		}
	} else {
		fmt.Fprintf(h.writer, "  Scopes: None\n")
	}

	fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(key.UpdatedAt))

	return nil
}

func (h *plainHandler) HandleDeleteAPIKey(success bool, config DeleteConfig) error {
	if success {
		fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	} else {
		fmt.Fprintf(h.writer, "Failed to delete %s\n", config.ItemName)
	}
	return nil
}

// Statistics responses
func (h *plainHandler) HandleDeliverabilityStats(response *responses.DeliverabilityStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No deliverability statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	for i, stat := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Time Period: %s to %s\n",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		fmt.Fprintf(h.writer, "  Reception Count: %s\n", formatInt(stat.ReceptionCount))
		fmt.Fprintf(h.writer, "  Delivered Count: %s\n", formatInt(stat.DeliveredCount))
		fmt.Fprintf(h.writer, "  Deferred Count: %s\n", formatInt(stat.DeferredCount))
		fmt.Fprintf(h.writer, "  Bounced Count: %s\n", formatInt(stat.BouncedCount))
		fmt.Fprintf(h.writer, "  Failed Count: %s\n", formatInt(stat.FailedCount))
		fmt.Fprintf(h.writer, "  Suppressed Count: %s\n", formatInt(stat.SuppressedCount))
		fmt.Fprintf(h.writer, "  Opened Count: %s\n", formatInt(stat.OpenedCount))
		fmt.Fprintf(h.writer, "  Clicked Count: %s\n", formatInt(stat.ClickedCount))

		// Calculate delivery rate if we have reception count
		if stat.ReceptionCount > 0 {
			deliveryRate := (float64(stat.DeliveredCount) / float64(stat.ReceptionCount)) * 100
			fmt.Fprintf(h.writer, "  Delivery Rate: %.2f%%\n", deliveryRate)
		}

		// Calculate open rate if we have delivered count
		if stat.DeliveredCount > 0 {
			openRate := (float64(stat.OpenedCount) / float64(stat.DeliveredCount)) * 100
			fmt.Fprintf(h.writer, "  Open Rate: %.2f%%\n", openRate)
		}
	}
	return nil
}

func (h *plainHandler) HandleBounceStats(response *responses.BounceStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No bounce statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	for i, stat := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Time Period: %s to %s\n",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		if len(stat.Bounces) == 0 {
			fmt.Fprintf(h.writer, "  No bounces recorded\n")
		} else {
			totalBounces := 0
			for _, bounce := range stat.Bounces {
				totalBounces += bounce.Count
			}

			fmt.Fprintf(h.writer, "  Total Bounces: %s\n", formatInt(totalBounces))
			fmt.Fprintf(h.writer, "  Bounce Classifications:\n")

			for _, bounce := range stat.Bounces {
				percentage := float64(0)
				if totalBounces > 0 {
					percentage = (float64(bounce.Count) / float64(totalBounces)) * 100
				}
				fmt.Fprintf(h.writer, "    %s: %s (%.1f%%)\n",
					bounce.Classification, formatInt(bounce.Count), percentage)
			}
		}
	}
	return nil
}

func (h *plainHandler) HandleDeliveryTimeStats(response *responses.DeliveryTimeStatisticsResponse, config StatsConfig) error {
	if len(response.Data) == 0 {
		fmt.Fprintf(h.writer, "No delivery time statistics found\n")
		return nil
	}

	if config.Title != "" {
		fmt.Fprintf(h.writer, "%s\n\n", config.Title)
	}

	for i, stat := range response.Data {
		if i > 0 {
			fmt.Fprintf(h.writer, "\n")
		}

		fmt.Fprintf(h.writer, "Time Period: %s to %s\n",
			formatTime(stat.FromTimestamp), formatTime(stat.ToTimestamp))

		fmt.Fprintf(h.writer, "  Delivered Count: %s\n", formatInt(stat.DeliveredCount))
		fmt.Fprintf(h.writer, "  Average Delivery Time: %.2f seconds\n", stat.AvgDeliveryTime)

		// Convert to human-readable format
		if stat.AvgDeliveryTime >= 60 {
			minutes := stat.AvgDeliveryTime / 60
			if minutes >= 60 {
				hours := minutes / 60
				fmt.Fprintf(h.writer, "  Average Delivery Time: %.2f hours\n", hours)
			} else {
				fmt.Fprintf(h.writer, "  Average Delivery Time: %.2f minutes\n", minutes)
			}
		}

		if len(stat.DeliveryTimes) > 0 {
			fmt.Fprintf(h.writer, "  Per-Domain Delivery Times:\n")
			for _, dt := range stat.DeliveryTimes {
				domain := "Unknown"
				if dt.RecipientDomain != nil {
					domain = *dt.RecipientDomain
				}

				time := "N/A"
				if dt.DeliveryTime != nil {
					time = fmt.Sprintf("%.2f seconds", *dt.DeliveryTime)
					// Convert to minutes/hours if needed
					if *dt.DeliveryTime >= 60 {
						minutes := *dt.DeliveryTime / 60
						if minutes >= 60 {
							hours := minutes / 60
							time = fmt.Sprintf("%.2f hours", hours)
						} else {
							time = fmt.Sprintf("%.2f minutes", minutes)
						}
					}
				}

				fmt.Fprintf(h.writer, "    %s: %s\n", domain, time)
			}
		}
	}
	return nil
}

// Auth responses
func (h *plainHandler) HandleAuthLogin(success bool, profile string, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Profile: %s\n", profile)
	return nil
}

func (h *plainHandler) HandleAuthLogout(success bool, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	return nil
}

func (h *plainHandler) HandleAuthStatus(status *AuthStatus, config AuthConfig) error {
	if status == nil {
		fmt.Fprintf(h.writer, "No authentication status available\n")
		return nil
	}

	fmt.Fprintf(h.writer, "Authentication Status\n")
	fmt.Fprintf(h.writer, "Profile: %s\n", status.Profile)
	fmt.Fprintf(h.writer, "API Key: %s\n", status.APIKey)
	fmt.Fprintf(h.writer, "Valid: %s\n", formatBooleanStatus(status.Valid))

	if status.Account != nil {
		fmt.Fprintf(h.writer, "\nAccount Details:\n")
		fmt.Fprintf(h.writer, "  ID: %s\n", formatUUID(status.Account.ID))
		fmt.Fprintf(h.writer, "  Name: %s\n", status.Account.Name)
		if status.Account.Website != nil {
			fmt.Fprintf(h.writer, "  Website: %s\n", formatOptionalString(status.Account.Website))
		}
		if status.Account.About != nil {
			fmt.Fprintf(h.writer, "  About: %s\n", formatOptionalString(status.Account.About))
		}
		fmt.Fprintf(h.writer, "  Created: %s\n", formatTime(status.Account.CreatedAt))
		fmt.Fprintf(h.writer, "  Updated: %s\n", formatTime(status.Account.UpdatedAt))

		// Email behavior settings
		if status.Account.TrackOpens != nil || status.Account.TrackClicks != nil ||
			status.Account.RejectBadRecipients != nil || status.Account.RejectMistypedRecipients != nil {
			fmt.Fprintf(h.writer, "\nEmail Settings:\n")
			if status.Account.TrackOpens != nil {
				fmt.Fprintf(h.writer, "  Track Opens: %s\n", formatBooleanStatus(*status.Account.TrackOpens))
			}
			if status.Account.TrackClicks != nil {
				fmt.Fprintf(h.writer, "  Track Clicks: %s\n", formatBooleanStatus(*status.Account.TrackClicks))
			}
			if status.Account.RejectBadRecipients != nil {
				fmt.Fprintf(h.writer, "  Reject Bad Recipients: %s\n", formatBooleanStatus(*status.Account.RejectBadRecipients))
			}
			if status.Account.RejectMistypedRecipients != nil {
				fmt.Fprintf(h.writer, "  Reject Mistyped Recipients: %s\n", formatBooleanStatus(*status.Account.RejectMistypedRecipients))
			}
		}

		// Data retention settings
		if status.Account.MessageMetadataRetention != nil || status.Account.MessageDataRetention != nil {
			fmt.Fprintf(h.writer, "\nData Retention:\n")
			if status.Account.MessageMetadataRetention != nil {
				fmt.Fprintf(h.writer, "  Message Metadata: %d days\n", *status.Account.MessageMetadataRetention)
			}
			if status.Account.MessageDataRetention != nil {
				fmt.Fprintf(h.writer, "  Message Data: %d days\n", *status.Account.MessageDataRetention)
			}
		}
	}

	return nil
}

func (h *plainHandler) HandleAuthSwitch(newProfile string, config AuthConfig) error {
	fmt.Fprintf(h.writer, "%s\n", config.SuccessMessage)
	fmt.Fprintf(h.writer, "Switched to profile: %s\n", newProfile)
	return nil
}

// Simple success and empty responses
func (h *plainHandler) HandleSimpleSuccess(message string) error {
	fmt.Fprintf(h.writer, "%s\n", message)
	return nil
}

func (h *plainHandler) HandleEmpty(message string) error {
	fmt.Fprintf(h.writer, "%s\n", message)
	return nil
}
