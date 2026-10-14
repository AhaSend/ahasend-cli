package printer

import (
	"fmt"
	"maps"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// listFields are the fields of each view of a contact list, which the
// table, plain and CSV output show in this order, except a table of lists,
// which shows listTableColumns.
var listFields = []string{"name", "id", "description", "tags", "contact_count", "created_at", "updated_at"}

// listTableColumns are the fields a table of lists shows.
var listTableColumns = []string{"name", "id", "tags", "contact_count", "updated_at"}

// listHeaders names the table column or row of each list field.
var listHeaders = map[string]string{
	"name":          "Name",
	"id":            "ID",
	"description":   "Description",
	"tags":          "Tags",
	"contact_count": "Contacts",
	"created_at":    "Created",
	"updated_at":    "Updated",
}

func (h *jsonHandler) HandleListsList(response *responses.PaginatedContactListsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleList(list *responses.ContactList, config SingleConfig) error {
	return h.printAPIResponse(list)
}

func (h *tableHandler) HandleListsList(response *responses.PaginatedContactListsResponse, config ListConfig) error {
	return writeTableList(h, config, listsPage(response), "lists", listTableColumns, listHeaders, listDisplayFields)
}

func (h *tableHandler) HandleSingleList(list *responses.ContactList, config SingleConfig) error {
	if list == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(listDetailRows(list))
	return nil
}

func (h *plainHandler) HandleListsList(response *responses.PaginatedContactListsResponse, config ListConfig) error {
	return writePlainList(h, config, listsPage(response), "lists", func(list *responses.ContactList) error {
		return writeLabelLines(h.writer, "", listDetailRows(list))
	})
}

func (h *plainHandler) HandleSingleList(list *responses.ContactList, config SingleConfig) error {
	if list == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", listDetailRows(list))
}

func (h *csvHandler) HandleListsList(response *responses.PaginatedContactListsResponse, config ListConfig) error {
	return writeCSVList(h, listsPage(response), listFields, listCSVFields)
}

func (h *csvHandler) HandleSingleList(list *responses.ContactList, config SingleConfig) error {
	if list == nil {
		return nil
	}
	return h.writeFieldMaps(listFields, listCSVFields(list))
}

// listCSVFields maps each list field to its CSV value.
func listCSVFields(list *responses.ContactList) map[string]string {
	values := listDisplayFields(list)
	maps.Copy(values, map[string]string{
		"created_at": formatCSVTime(list.CreatedAt),
		"updated_at": formatCSVTime(list.UpdatedAt),
	})
	return values
}

// listDisplayFields maps each list field to its table and plain value.
func listDisplayFields(list *responses.ContactList) map[string]string {
	return map[string]string{
		"name":          list.Name,
		"id":            formatUUID(list.ID),
		"description":   list.Description,
		"tags":          formatTags(list.Tags),
		"contact_count": formatInt(list.ContactCount),
		"created_at":    formatTime(list.CreatedAt),
		"updated_at":    formatTime(list.UpdatedAt),
	}
}

// listDetailRows returns the rows of a list's detail view.
func listDetailRows(list *responses.ContactList) [][]string {
	return fieldRows(listFields, listHeaders, listDisplayFields(list))
}

// listsPage returns the page of a PaginatedContactListsResponse, or nil for none.
func listsPage(response *responses.PaginatedContactListsResponse) *listPage[responses.ContactList] {
	if response == nil {
		return nil
	}
	return &listPage[responses.ContactList]{items: response.Data, pagination: common.PaginationInfo(response.Pagination)}
}
