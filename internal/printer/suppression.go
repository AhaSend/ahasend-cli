package printer

import (
	"fmt"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// suppressionFields are the fields of each view of a suppression, which the
// table, plain and CSV output show in this order.
var suppressionFields = []string{"email", "id", "domain", "reason", "created_at", "expires_at", "protected"}

// suppressionTableColumns are the fields a table of suppressions shows: all
// of them, as they fit.
var suppressionTableColumns = suppressionFields

// suppressionHeaders names the table column or row of each suppression field.
var suppressionHeaders = map[string]string{
	"email":      "Email",
	"id":         "ID",
	"domain":     "Domain",
	"reason":     "Reason",
	"created_at": "Created",
	"expires_at": "Expires",
	"protected":  "Protected",
}

func (h *jsonHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config ResultConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleCheckSuppression(response *responses.PaginatedSuppressionsResponse, config CheckConfig) error {
	return h.printAPIResponse(response)
}

func (h *tableHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "suppressions", suppressionTableColumns, suppressionHeaders, suppressionDisplayFields)
}

func (h *tableHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config ResultConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty("No suppressions created")
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderSuppressionTable(response.Data)
	return nil
}

// HandleCheckSuppression shows every suppression a check found, one row
// per suppression, as a list does.
func (h *tableHandler) HandleCheckSuppression(response *responses.PaginatedSuppressionsResponse, config CheckConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.NotFoundMessage)
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.FoundMessage); err != nil {
		return err
	}
	h.renderSuppressionTable(response.Data)
	return nil
}

// renderSuppressionTable renders one row per suppression.
func (h *tableHandler) renderSuppressionTable(suppressions []responses.Suppression) {
	h.writeFieldTable(suppressionTableColumns, suppressionHeaders, fieldMapsOf(suppressions, suppressionDisplayFields)...)
}

func (h *plainHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "suppressions", func(suppression *responses.Suppression) error {
		return writeLabelLines(h.writer, "", suppressionDetailRows(suppression))
	})
}

func (h *plainHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config ResultConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty("No suppressions created")
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.writeSuppressionItems(response.Data)
}

// HandleCheckSuppression shows every suppression a check found, as a list
// does.
func (h *plainHandler) HandleCheckSuppression(response *responses.PaginatedSuppressionsResponse, config CheckConfig) error {
	if response == nil || len(response.Data) == 0 {
		return h.HandleEmpty(config.NotFoundMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.FoundMessage); err != nil {
		return err
	}
	return h.writeSuppressionItems(response.Data)
}

// writeSuppressionItems writes each suppression's fields, with a blank line
// between suppressions.
func (h *plainHandler) writeSuppressionItems(suppressions []responses.Suppression) error {
	for i := range suppressions {
		if err := h.writeItemSeparator(i); err != nil {
			return err
		}
		if err := writeLabelLines(h.writer, "", suppressionDetailRows(&suppressions[i])); err != nil {
			return err
		}
	}
	return nil
}

func (h *csvHandler) HandleSuppressionList(response *responses.PaginatedSuppressionsResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), suppressionFields, suppressionCSVFields)
}

func (h *csvHandler) HandleCreateSuppression(response *responses.CreateSuppressionResponse, config ResultConfig) error {
	if response == nil {
		return nil
	}
	return h.writeSuppressionsCSV(response.Data)
}

// HandleCheckSuppression writes every suppression a check found, and
// nothing when the address is not suppressed, since CSV holds data only.
func (h *csvHandler) HandleCheckSuppression(response *responses.PaginatedSuppressionsResponse, config CheckConfig) error {
	if response == nil {
		return h.writeSuppressionsCSV(nil)
	}
	return h.writeSuppressionsCSV(response.Data)
}

func (h *csvHandler) writeSuppressionsCSV(suppressions []responses.Suppression) error {
	return h.writeFieldMaps(suppressionFields, fieldMapsOf(suppressions, suppressionCSVFields)...)
}

// suppressionCSVFields maps each suppression field to its CSV value.
func suppressionCSVFields(suppression *responses.Suppression) map[string]string {
	return map[string]string{
		"id":         formatUUID(suppression.ID),
		"email":      suppression.Email,
		"domain":     suppression.Domain,
		"reason":     suppression.Reason,
		"created_at": formatCSVTime(suppression.CreatedAt),
		"expires_at": formatCSVTime(suppression.ExpiresAt),
		"protected":  formatBool(suppression.Protected),
	}
}

// suppressionDisplayFields maps each suppression field to its table value.
func suppressionDisplayFields(suppression *responses.Suppression) map[string]string {
	return map[string]string{
		"email":      suppression.Email,
		"id":         formatUUID(suppression.ID),
		"domain":     suppression.Domain,
		"reason":     suppression.Reason,
		"created_at": formatTime(suppression.CreatedAt),
		"expires_at": formatTime(suppression.ExpiresAt),
		"protected":  formatBooleanStatus(suppression.Protected),
	}
}

// suppressionDetailRows returns the rows of a suppression's detail view.
func suppressionDetailRows(suppression *responses.Suppression) [][]string {
	return fieldRows(suppressionFields, suppressionHeaders, suppressionDisplayFields(suppression))
}
