package printer

import (
	"fmt"
	"maps"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
)

// The fields of the views of a segment, which the table, plain and CSV
// output of each view show in this order, except a table of segments, which
// shows segmentTableColumns.
var (
	// segmentFields are the fields of a segment as a list, create and update
	// return it.
	segmentFields = []string{
		"name", "id", "description", "state", "failure_reason", "member_count",
		"timezone", "last_evaluated_at", "created_at", "updated_at", "definition",
	}
	// segmentDetailFields are the fields of a segment as get returns it, with
	// the members a campaign to it alone would reach.
	segmentDetailFields = []string{
		"name", "id", "description", "state", "failure_reason", "member_count", "marketable_count",
		"timezone", "last_evaluated_at", "created_at", "updated_at", "definition",
	}
	// segmentTableColumns are the fields a table of segments shows.
	segmentTableColumns = []string{"name", "id", "state", "member_count", "updated_at"}
)

// segmentHeaders names the table column or row of each segment field.
var segmentHeaders = map[string]string{
	"name":              "Name",
	"id":                "ID",
	"description":       "Description",
	"state":             "State",
	"failure_reason":    "Failure Reason",
	"member_count":      "Members",
	"marketable_count":  "Marketable",
	"timezone":          "Timezone",
	"last_evaluated_at": "Last Evaluated",
	"created_at":        "Created",
	"updated_at":        "Updated",
	"definition":        "Definition",
}

// uncountedMarketable stands in table and plain output for a marketable count
// the API did not give: the segment is not ready, or its members could not
// be counted in time.
const uncountedMarketable = "unknown"

func (h *jsonHandler) HandleSegmentList(response *responses.PaginatedSegmentsResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleSegment(segment *responses.Segment, config SingleConfig) error {
	return h.printAPIResponse(segment)
}

func (h *jsonHandler) HandleSegmentDetail(segment *responses.SegmentDetail, config SingleConfig) error {
	return h.printAPIResponse(segment)
}

func (h *tableHandler) HandleSegmentList(response *responses.PaginatedSegmentsResponse, config ListConfig) error {
	return writeTableList(h, config, segmentsPage(response), "segments", segmentTableColumns, segmentHeaders, segmentDisplayFields)
}

func (h *tableHandler) HandleSingleSegment(segment *responses.Segment, config SingleConfig) error {
	if segment == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.renderSegment(segment, segmentFields, nil, config)
}

func (h *tableHandler) HandleSegmentDetail(segment *responses.SegmentDetail, config SingleConfig) error {
	if segment == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.renderSegment(&segment.Segment, segmentDetailFields, segmentDetailValues(segment), config)
}

// renderSegment writes the success message, then the segment's fields as
// field rows, with the values of the fields only its view has in extra.
func (h *tableHandler) renderSegment(segment *responses.Segment, fields []string, extra map[string]string, config SingleConfig) error {
	rows, err := segmentRows(segment, fields, extra)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", config.SuccessMessage); err != nil {
		return err
	}
	h.renderFieldTable(rows)
	return nil
}

func (h *plainHandler) HandleSegmentList(response *responses.PaginatedSegmentsResponse, config ListConfig) error {
	return writePlainList(h, config, segmentsPage(response), "segments", func(segment *responses.Segment) error {
		return h.writeSegment(segment, segmentFields, nil)
	})
}

func (h *plainHandler) HandleSingleSegment(segment *responses.Segment, config SingleConfig) error {
	if segment == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.writeSegment(segment, segmentFields, nil)
}

func (h *plainHandler) HandleSegmentDetail(segment *responses.SegmentDetail, config SingleConfig) error {
	if segment == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	if _, err := fmt.Fprintln(h.writer, config.SuccessMessage); err != nil {
		return err
	}
	return h.writeSegment(&segment.Segment, segmentDetailFields, segmentDetailValues(segment))
}

// writeSegment writes the label line of each of the segment's fields, with
// the values of the fields only its view has in extra.
func (h *plainHandler) writeSegment(segment *responses.Segment, fields []string, extra map[string]string) error {
	rows, err := segmentRows(segment, fields, extra)
	if err != nil {
		return err
	}
	return writeLabelLines(h.writer, "", rows)
}

func (h *csvHandler) HandleSegmentList(response *responses.PaginatedSegmentsResponse, config ListConfig) error {
	if response == nil {
		return nil
	}
	rows := make([]map[string]string, len(response.Data))
	for i := range response.Data {
		row, err := segmentCSVFields(&response.Data[i])
		if err != nil {
			return err
		}
		rows[i] = row
	}
	return h.writeFieldMaps(segmentFields, rows...)
}

func (h *csvHandler) HandleSingleSegment(segment *responses.Segment, config SingleConfig) error {
	if segment == nil {
		return nil
	}
	row, err := segmentCSVFields(segment)
	if err != nil {
		return err
	}
	return h.writeFieldMaps(segmentFields, row)
}

func (h *csvHandler) HandleSegmentDetail(segment *responses.SegmentDetail, config SingleConfig) error {
	if segment == nil {
		return nil
	}
	row, err := segmentCSVFields(&segment.Segment)
	if err != nil {
		return err
	}
	row["marketable_count"] = formatOptionalInt(segment.MarketableCount)
	return h.writeFieldMaps(segmentDetailFields, row)
}

// segmentRows returns a label and value row for each of fields of the
// segment, the definition as indented JSON, with the values of the fields
// only its view has in extra.
func segmentRows(segment *responses.Segment, fields []string, extra map[string]string) ([][]string, error) {
	definition, err := encodeJSON(segment.Definition, "  ")
	if err != nil {
		return nil, err
	}
	values := segmentDisplayFields(segment)
	values["definition"] = string(definition)
	maps.Copy(values, extra)
	return fieldRows(fields, segmentHeaders, values), nil
}

// segmentDetailValues maps the fields only a segment's detail view has to
// their table and plain value.
func segmentDetailValues(segment *responses.SegmentDetail) map[string]string {
	marketable := uncountedMarketable
	if segment.MarketableCount != nil {
		marketable = formatInt(*segment.MarketableCount)
	}
	return map[string]string{"marketable_count": marketable}
}

// segmentCSVFields maps each segment field to its CSV value, the definition
// as compact JSON.
func segmentCSVFields(segment *responses.Segment) (map[string]string, error) {
	definition, err := encodeJSON(segment.Definition, "")
	if err != nil {
		return nil, err
	}
	values := segmentDisplayFields(segment)
	maps.Copy(values, map[string]string{
		"last_evaluated_at": formatOptionalCSVTime(segment.LastEvaluatedAt),
		"created_at":        formatCSVTime(segment.CreatedAt),
		"updated_at":        formatCSVTime(segment.UpdatedAt),
		"definition":        string(definition),
	})
	return values, nil
}

// segmentDisplayFields maps each segment field but the definition to its
// table and plain value.
func segmentDisplayFields(segment *responses.Segment) map[string]string {
	return map[string]string{
		"name":              segment.Name,
		"id":                formatUUID(segment.ID),
		"description":       segment.Description,
		"state":             segment.State,
		"failure_reason":    formatOptionalString(segment.FailureReason),
		"member_count":      formatInt(segment.MemberCount),
		"timezone":          segment.Timezone,
		"last_evaluated_at": formatOptionalTime(segment.LastEvaluatedAt),
		"created_at":        formatTime(segment.CreatedAt),
		"updated_at":        formatTime(segment.UpdatedAt),
	}
}

// segmentsPage returns the page of a PaginatedSegmentsResponse, or nil for none.
func segmentsPage(response *responses.PaginatedSegmentsResponse) *listPage[responses.Segment] {
	if response == nil {
		return nil
	}
	return &listPage[responses.Segment]{items: response.Data, pagination: common.PaginationInfo(response.Pagination)}
}
