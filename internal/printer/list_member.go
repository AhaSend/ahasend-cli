package printer

import (
	"fmt"
	"maps"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// A membership has three views, each with the fields its endpoint returns.
// Table, plain and CSV output show the fields of a view in this order, except
// a table of memberships, which shows the view's table columns.
var (
	// listMemberFields are the fields of a member of lists members list, whose
	// contact the API embeds, with its names.
	listMemberFields = []string{
		"email", "contact_id", "first_name", "last_name", "list_id", "subscription_status",
		"subscribed_at", "unsubscribed_at", "created_at", "updated_at",
	}
	listMemberTableColumns = []string{
		"email", "contact_id", "first_name", "last_name", "subscription_status", "subscribed_at",
	}

	// contactMembershipFields are the fields of a membership of contacts
	// memberships, whose list the API embeds, with its name.
	contactMembershipFields = []string{
		"list_name", "list_id", "email", "contact_id", "subscription_status",
		"subscribed_at", "unsubscribed_at", "created_at", "updated_at",
	}
	contactMembershipTableColumns = []string{
		"list_name", "list_id", "subscription_status", "subscribed_at", "unsubscribed_at",
	}

	// membershipFields are the fields of the membership lists members set
	// writes, which embeds neither the list nor the contact.
	membershipFields = []string{
		"email", "contact_id", "list_id", "subscription_status",
		"subscribed_at", "unsubscribed_at", "created_at", "updated_at",
	}
)

// listMemberHeaders names the table column or row of each membership field.
var listMemberHeaders = map[string]string{
	"email":               "Email",
	"contact_id":          "Contact ID",
	"list_name":           "List",
	"list_id":             "List ID",
	"subscription_status": "Subscription Status",
	"subscribed_at":       "Subscribed At",
	"unsubscribed_at":     "Unsubscribed At",
	"created_at":          "Created",
	"updated_at":          "Updated",
	"first_name":          "First Name",
	"last_name":           "Last Name",
}

func (h *jsonHandler) HandleListMemberList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleContactMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleListMember(member *responses.ListContact, config SingleConfig) error {
	return h.printAPIResponse(member)
}

func (h *jsonHandler) HandleBatchListMembers(response *responses.BatchAddListContactsResponse, config SingleConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleListMemberList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, config, listMemberTableColumns)
}

func (h *tableHandler) HandleContactMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, config, contactMembershipTableColumns)
}

// writeMembershipList writes a page of memberships as a table of columns.
func (h *tableHandler) writeMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig, columns []string) error {
	return writeTableList(h, config, listMembersPage(response), config.ItemName, columns, listMemberHeaders, listMemberDisplayFields)
}

func (h *tableHandler) HandleSingleListMember(member *responses.ListContact, config SingleConfig) error {
	if member == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(fieldRows(membershipFields, listMemberHeaders, listMemberDisplayFields(member)))
	return nil
}

func (h *tableHandler) HandleBatchListMembers(response *responses.BatchAddListContactsResponse, config SingleConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nAdded: %d  Skipped: %d  Failed: %d\n\n",
		config.SuccessMessage, response.Added, response.Skipped, response.Failed); err != nil {
		return err
	}
	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		rows[i] = listMemberBatchRow(&response.Data[i])
	}
	h.renderTable(batchResultHeaders, rows...)
	return nil
}

func (h *plainHandler) HandleListMemberList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, config, listMemberFields)
}

func (h *plainHandler) HandleContactMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, config, contactMembershipFields)
}

// writeMembershipList writes a page of memberships, each with fields.
func (h *plainHandler) writeMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig, fields []string) error {
	return writePlainList(h, config, listMembersPage(response), config.ItemName, func(member *responses.ListContact) error {
		return h.writeFieldLines(fields, listMemberHeaders, listMemberDisplayFields(member))
	})
}

func (h *plainHandler) HandleSingleListMember(member *responses.ListContact, config SingleConfig) error {
	if member == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", fieldRows(membershipFields, listMemberHeaders, listMemberDisplayFields(member)))
}

func (h *plainHandler) HandleBatchListMembers(response *responses.BatchAddListContactsResponse, config SingleConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.EmptyMessage)
	}

	if _, err := fmt.Fprintf(h.writer, "%s\nAdded: %d\nSkipped: %d\nFailed: %d\n",
		config.SuccessMessage, response.Added, response.Skipped, response.Failed); err != nil {
		return err
	}
	for i := range response.Data {
		if err := writeBatchResult(h.writer, i+1, listMemberBatchRow(&response.Data[i])); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleListMemberList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, listMemberFields)
}

func (h *csvHandler) HandleContactMembershipList(response *responses.PaginatedListContactsResponse, config ListConfig) error {
	return h.writeMembershipList(response, contactMembershipFields)
}

// writeMembershipList writes a page of memberships as rows of fields.
func (h *csvHandler) writeMembershipList(response *responses.PaginatedListContactsResponse, fields []string) error {
	return writeCSVList(h, listMembersPage(response), fields, listMemberCSVFields)
}

func (h *csvHandler) HandleSingleListMember(member *responses.ListContact, config SingleConfig) error {
	if member == nil {
		return nil
	}
	return h.writeFieldMaps(membershipFields, listMemberCSVFields(member))
}

func (h *csvHandler) HandleBatchListMembers(response *responses.BatchAddListContactsResponse, config SingleConfig) error {
	if response == nil {
		return nil
	}

	rows := make([][]string, len(response.Data))
	for i := range response.Data {
		rows[i] = listMemberBatchRow(&response.Data[i])
	}
	return h.writeRows(batchResultCSVHeaders, rows...)
}

// listMemberCSVFields maps each membership field to its CSV value.
func listMemberCSVFields(member *responses.ListContact) map[string]string {
	values := listMemberDisplayFields(member)
	maps.Copy(values, map[string]string{
		"subscribed_at":   formatOptionalCSVTime(member.SubscribedAt),
		"unsubscribed_at": formatOptionalCSVTime(member.UnsubscribedAt),
		"created_at":      formatCSVTime(member.CreatedAt),
		"updated_at":      formatCSVTime(member.UpdatedAt),
	})
	return values
}

// listMemberDisplayFields maps each membership field to its table and plain
// value.
func listMemberDisplayFields(member *responses.ListContact) map[string]string {
	values := map[string]string{
		"email":               member.Email,
		"contact_id":          formatUUID(member.ContactID),
		"list_id":             formatUUID(member.ListID),
		"subscription_status": member.SubscriptionStatus,
		"subscribed_at":       formatOptionalTime(member.SubscribedAt),
		"unsubscribed_at":     formatOptionalTime(member.UnsubscribedAt),
		"created_at":          formatTime(member.CreatedAt),
		"updated_at":          formatTime(member.UpdatedAt),
	}
	if member.List != nil {
		values["list_name"] = member.List.Name
	}
	if member.Contact != nil {
		values["first_name"] = member.Contact.FirstName
		values["last_name"] = member.Contact.LastName
	}
	return values
}

// listMemberBatchRow returns a batch result's position, email, contact ID,
// outcome and reason. The contact ID is the ID of the contact the API
// resolved, which the result's membership holds, or else the ID the member
// was given by; it is empty for a member given by an email the API resolved
// to no contact.
func listMemberBatchRow(result *responses.BatchListContactResult) []string {
	contactID := formatOptionalUUID(result.ID)
	if result.Membership != nil {
		contactID = formatUUID(result.Membership.ContactID)
	}
	return []string{
		formatInt(result.Position), result.Email, contactID,
		result.Outcome, result.Reason,
	}
}

// listMembersPage returns the page of a PaginatedListContactsResponse, or nil for none.
func listMembersPage(response *responses.PaginatedListContactsResponse) *listPage[responses.ListContact] {
	if response == nil {
		return nil
	}
	return &listPage[responses.ListContact]{items: response.Data, pagination: common.PaginationInfo(response.Pagination)}
}
