package printer

import (
	"maps"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// campaignRecipientFields are the fields of each recipient of a campaign,
// which plain and CSV output show in this order.
var campaignRecipientFields = []string{
	"email", "contact_id", "list_name", "status", "bounce_classification",
	"bounce_reason", "bounce_message", "attempts", "created_at", "updated_at",
}

// campaignRecipientTableColumns are the fields a table of recipients shows.
var campaignRecipientTableColumns = []string{
	"email", "contact_id", "list_name", "status", "bounce_classification", "attempts",
}

// campaignRecipientHeaders names the table column or row of each recipient
// field.
var campaignRecipientHeaders = map[string]string{
	"email":                 "Email",
	"contact_id":            "Contact ID",
	"list_name":             "List",
	"status":                "Status",
	"bounce_classification": "Bounce Classification",
	"bounce_reason":         "Bounce Reason",
	"bounce_message":        "Bounce Message",
	"attempts":              "Attempts",
	"created_at":            "Created",
	"updated_at":            "Updated",
}

func (h *jsonHandler) HandleCampaignRecipientList(response *responses.PaginatedCampaignRecipientsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleCampaignRecipientList(response *responses.PaginatedCampaignRecipientsResponse, config ListConfig) error {
	return writeTableList(h, config, campaignRecipientsPage(response), "recipients",
		campaignRecipientTableColumns, campaignRecipientHeaders, campaignRecipientDisplayFields)
}

func (h *plainHandler) HandleCampaignRecipientList(response *responses.PaginatedCampaignRecipientsResponse, config ListConfig) error {
	return writePlainList(h, config, campaignRecipientsPage(response), "recipients", func(recipient *responses.CampaignRecipient) error {
		return h.writeFieldLines(campaignRecipientFields, campaignRecipientHeaders, campaignRecipientDisplayFields(recipient))
	})
}

func (h *csvHandler) HandleCampaignRecipientList(response *responses.PaginatedCampaignRecipientsResponse, config ListConfig) error {
	return writeCSVList(h, campaignRecipientsPage(response), campaignRecipientFields, campaignRecipientCSVFields)
}

// campaignRecipientCSVFields maps each recipient field to its CSV value.
func campaignRecipientCSVFields(recipient *responses.CampaignRecipient) map[string]string {
	values := campaignRecipientDisplayFields(recipient)
	maps.Copy(values, map[string]string{
		"created_at": formatCSVTime(recipient.CreatedAt),
		"updated_at": formatCSVTime(recipient.UpdatedAt),
	})
	return values
}

// campaignRecipientDisplayFields maps each recipient field to its table and
// plain value.
func campaignRecipientDisplayFields(recipient *responses.CampaignRecipient) map[string]string {
	return map[string]string{
		"email":                 recipient.Email,
		"contact_id":            formatUUID(recipient.ContactID),
		"list_name":             recipient.ListName,
		"status":                recipient.Status,
		"bounce_classification": formatOptionalString(recipient.BounceClassification),
		"bounce_reason":         formatOptionalString(recipient.BounceReason),
		"bounce_message":        formatOptionalString(recipient.BounceMessage),
		"attempts":              formatInt(recipient.Attempts),
		"created_at":            formatTime(recipient.CreatedAt),
		"updated_at":            formatTime(recipient.UpdatedAt),
	}
}

// campaignRecipientsPage returns the page of a PaginatedCampaignRecipientsResponse, or nil for none.
func campaignRecipientsPage(response *responses.PaginatedCampaignRecipientsResponse) *listPage[responses.CampaignRecipient] {
	if response == nil {
		return nil
	}
	return &listPage[responses.CampaignRecipient]{items: response.Data, pagination: response.Pagination}
}
