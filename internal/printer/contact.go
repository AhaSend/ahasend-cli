package printer

import (
	"fmt"
	"maps"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// contactFields are the fields of each view of a contact, which the table,
// plain and CSV output show in this order, except a table of contacts, which
// shows contactTableColumns.
var contactFields = []string{
	"email", "id", "first_name", "last_name", "status", "status_reason",
	"unsubscribed", "unsubscribed_at", "attributes", "validation_status",
	"last_validated_at", "created_at", "updated_at",
}

// contactTableColumns are the fields a table of contacts shows.
var contactTableColumns = []string{"email", "id", "status", "unsubscribed", "attributes"}

// contactHeaders names the table column or row of each contact field.
var contactHeaders = map[string]string{
	"email":             "Email",
	"id":                "ID",
	"first_name":        "First Name",
	"last_name":         "Last Name",
	"status":            "Status",
	"status_reason":     "Status Reason",
	"unsubscribed":      "Unsubscribed",
	"unsubscribed_at":   "Unsubscribed At",
	"attributes":        "Attributes",
	"validation_status": "Validation Status",
	"last_validated_at": "Last Validated At",
	"created_at":        "Created",
	"updated_at":        "Updated",
}

func (h *jsonHandler) HandleContactList(response *responses.PaginatedContactsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleContact(contact *responses.Contact, config SingleConfig) error {
	return h.printAPIResponse(contact)
}

func (h *jsonHandler) HandleBatchContacts(response *responses.BatchUpsertContactsResponse, config SingleConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleContactList(response *responses.PaginatedContactsResponse, config ListConfig) error {
	page := contactsPage(response)
	if page == nil || len(page.items) == 0 {
		return writeEmptyPage(h.writer, config.EmptyMessage, page.cursors())
	}

	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	rows := make([]map[string]string, len(page.items))
	for i := range page.items {
		values, err := contactDisplayFields(&page.items[i])
		if err != nil {
			return err
		}
		rows[i] = values
	}
	h.writeFieldTable(contactTableColumns, contactHeaders, rows...)

	return writeListFooter(h.writer, len(page.items), "contacts", page.pagination)
}

func (h *tableHandler) HandleSingleContact(contact *responses.Contact, config SingleConfig) error {
	if contact == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	return h.renderContactDetails(contact)
}

func (h *tableHandler) HandleBatchContacts(response *responses.BatchUpsertContactsResponse, config SingleConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nCreated: %d  Updated: %d  Failed: %d\n\n",
		config.SuccessMessage, response.Created, response.Updated, response.Failed); err != nil {
		return err
	}
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		rows[i] = contactBatchRow(&response.Data[i])
	}
	h.renderTable(batchResultHeaders, rows...)
	return nil
}

func (h *tableHandler) renderContactDetails(contact *responses.Contact) error {
	values, err := contactDisplayFields(contact)
	if err != nil {
		return err
	}
	h.renderFieldTable(fieldRows(contactFields, contactHeaders, values))
	return nil
}

func (h *plainHandler) HandleContactList(response *responses.PaginatedContactsResponse, config ListConfig) error {
	return writePlainList(h, config, contactsPage(response), "contacts", h.renderContactDetails)
}

func (h *plainHandler) HandleSingleContact(contact *responses.Contact, config SingleConfig) error {
	if contact == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.renderContactDetails(contact)
}

func (h *plainHandler) HandleBatchContacts(response *responses.BatchUpsertContactsResponse, config SingleConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nCreated: %d\nUpdated: %d\nFailed: %d\n",
		config.SuccessMessage, response.Created, response.Updated, response.Failed); err != nil {
		return err
	}
	for i := range response.Data {
		if err := writeBatchResult(h.writer, i+1, contactBatchRow(&response.Data[i])); err != nil {
			return err
		}
	}
	return nil
}

func (h *plainHandler) renderContactDetails(contact *responses.Contact) error {
	values, err := contactDisplayFields(contact)
	if err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", fieldRows(contactFields, contactHeaders, values))
}

func (h *csvHandler) HandleContactList(response *responses.PaginatedContactsResponse, config ListConfig) error {
	if response == nil {
		return nil
	}
	return h.writeContactsCSV(response.Data)
}

func (h *csvHandler) HandleSingleContact(contact *responses.Contact, config SingleConfig) error {
	if contact == nil {
		return nil
	}
	return h.writeContactsCSV([]responses.Contact{*contact})
}

func (h *csvHandler) HandleBatchContacts(response *responses.BatchUpsertContactsResponse, config SingleConfig) error {
	if response == nil {
		return nil
	}

	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		rows[i] = contactBatchRow(&response.Data[i])
	}
	return h.writeRows(batchResultCSVHeaders, rows...)
}

func (h *csvHandler) writeContactsCSV(contacts []responses.Contact) error {
	rows := make([]map[string]string, len(contacts))
	for i := range contacts {
		fields, err := contactCSVFields(&contacts[i])
		if err != nil {
			return err
		}
		rows[i] = fields
	}
	return h.writeFieldMaps(contactFields, rows...)
}

// contactCSVFields maps each contact field to its CSV value.
func contactCSVFields(contact *responses.Contact) (map[string]string, error) {
	values, err := contactDisplayFields(contact)
	if err != nil {
		return nil, err
	}
	maps.Copy(values, map[string]string{
		"unsubscribed":      formatBool(contact.Unsubscribed),
		"unsubscribed_at":   formatOptionalCSVTime(contact.UnsubscribedAt),
		"last_validated_at": formatOptionalCSVTime(contact.LastValidatedAt),
		"created_at":        formatCSVTime(contact.CreatedAt),
		"updated_at":        formatCSVTime(contact.UpdatedAt),
	})
	return values, nil
}

// contactBatchRow returns a batch result's position, email, contact ID,
// outcome and reason. The contact ID is empty when the entry failed.
func contactBatchRow(result *responses.BatchContactResult) []string {
	contactID := ""
	if result.Contact != nil {
		contactID = formatUUID(result.Contact.ID)
	}
	return []string{
		formatInt(result.Position), result.Email, contactID,
		result.Outcome, result.Reason,
	}
}

// contactDisplayFields maps each contact field to its table and plain value.
func contactDisplayFields(contact *responses.Contact) (map[string]string, error) {
	attributes, err := stableJSON(contact.Attributes)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"email":             contact.Email,
		"id":                formatUUID(contact.ID),
		"first_name":        contact.FirstName,
		"last_name":         contact.LastName,
		"status":            contact.Status,
		"status_reason":     contact.StatusReason,
		"unsubscribed":      formatBooleanStatus(contact.Unsubscribed),
		"unsubscribed_at":   formatOptionalTime(contact.UnsubscribedAt),
		"attributes":        attributes,
		"validation_status": contact.ValidationStatus,
		"last_validated_at": formatOptionalTime(contact.LastValidatedAt),
		"created_at":        formatTime(contact.CreatedAt),
		"updated_at":        formatTime(contact.UpdatedAt),
	}, nil
}

// contactsPage returns the page of a contacts response, or nil for none.
func contactsPage(response *responses.PaginatedContactsResponse) *listPage[responses.Contact] {
	if response == nil {
		return nil
	}
	return &listPage[responses.Contact]{items: response.Data, pagination: common.PaginationInfo(response.Pagination)}
}
