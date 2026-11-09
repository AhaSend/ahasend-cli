package printer

import (
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// segmentMemberFields are the fields of each member of a segment, which the
// table, plain and CSV output show in this order.
var segmentMemberFields = []string{"email", "contact_id", "entered_at"}

// segmentMemberHeaders names the table column or plain label of each member
// field.
var segmentMemberHeaders = map[string]string{
	"email":      "Email",
	"contact_id": "Contact ID",
	"entered_at": "Entered",
}

func (h *jsonHandler) HandleSegmentMemberList(response *responses.PaginatedSegmentContactsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleSegmentMemberList(response *responses.PaginatedSegmentContactsResponse, config ListConfig) error {
	return writeTableList(h, config, segmentMembersPage(response), "segment members", segmentMemberFields,
		segmentMemberHeaders, segmentMemberDisplayFields)
}

func (h *plainHandler) HandleSegmentMemberList(response *responses.PaginatedSegmentContactsResponse, config ListConfig) error {
	return writePlainList(h, config, segmentMembersPage(response), "segment members", func(member *responses.SegmentContact) error {
		return h.writeFieldLines(segmentMemberFields, segmentMemberHeaders, segmentMemberDisplayFields(member))
	})
}

func (h *csvHandler) HandleSegmentMemberList(response *responses.PaginatedSegmentContactsResponse, config ListConfig) error {
	return writeCSVList(h, segmentMembersPage(response), segmentMemberFields, segmentMemberCSVFields)
}

// segmentMemberCSVFields maps each member field to its CSV value.
func segmentMemberCSVFields(member *responses.SegmentContact) map[string]string {
	values := segmentMemberDisplayFields(member)
	values["entered_at"] = formatCSVTime(member.EnteredAt)
	return values
}

// segmentMemberDisplayFields maps each member field to its table and plain
// value.
func segmentMemberDisplayFields(member *responses.SegmentContact) map[string]string {
	return map[string]string{
		"email":      member.Email,
		"contact_id": formatUUID(member.ContactID),
		"entered_at": formatTime(member.EnteredAt),
	}
}

// segmentMembersPage returns the page of a PaginatedSegmentContactsResponse,
// or nil for none.
func segmentMembersPage(response *responses.PaginatedSegmentContactsResponse) *listPage[responses.SegmentContact] {
	if response == nil {
		return nil
	}
	return &listPage[responses.SegmentContact]{items: response.Data, pagination: common.PaginationInfo(response.Pagination)}
}
