package printer

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// segmentDefinitionFixture returns a definition with a condition, a group
// and a count, the shape of the definition in the segments help.
func segmentDefinitionFixture() common.SegmentDefinition {
	plan, country := "pro", "DE"
	return common.SegmentDefinition{
		SchemaVersion: common.SegmentSchemaVersion,
		Match:         common.SegmentMatchAll,
		Conditions: []common.SegmentRule{
			{Condition: &common.SegmentCondition{
				Field: "attributes.plan", Op: common.SegmentOpEqual, Value: &common.SegmentValue{String: &plan},
			}},
			{Group: &common.SegmentGroup{Match: common.SegmentMatchAny, Conditions: []common.SegmentCondition{
				{Field: "attributes.country", Op: common.SegmentOpEqual, Value: &common.SegmentValue{String: &country}},
				{Field: common.SegmentFieldCampaignCount, Op: common.SegmentOpOpenedAtLeast, Value: &common.SegmentValue{
					Count: &common.SegmentCampaignCount{Count: 3, Since: "90 days ago"},
				}},
			}}},
		},
	}
}

// segmentDefinitionJSON is segmentDefinitionFixture as compact JSON.
const segmentDefinitionJSON = `{"schema_version":1,"match":"all","conditions":[` +
	`{"field":"attributes.plan","op":"equal","value":"pro"},` +
	`{"match":"any","conditions":[{"field":"attributes.country","op":"equal","value":"DE"},` +
	`{"field":"campaign_count","op":"opened_at_least","value":{"count":3,"since":"90 days ago"}}]}]}`

// segmentFixture returns a segment with every field set.
func segmentFixture() responses.Segment {
	createdAt := time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	evaluatedAt := createdAt.Add(30 * time.Minute)
	reason := "the members could not be found in time"
	return responses.Segment{
		Object:          "segment",
		ID:              uuid.MustParse("33333333-3333-4333-8333-333333333333"),
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt.Add(time.Hour),
		Name:            "Engaged pro customers",
		Description:     "Pro plan, opened lately",
		Definition:      segmentDefinitionFixture(),
		Timezone:        "Europe/Berlin",
		State:           common.SegmentStateFailed,
		FailureReason:   &reason,
		MemberCount:     42,
		LastEvaluatedAt: &evaluatedAt,
	}
}

func segmentDetailFixture() responses.SegmentDetail {
	marketable := 40
	return responses.SegmentDetail{Segment: segmentFixture(), MarketableCount: &marketable}
}

func segmentsPageFixture() *responses.PaginatedSegmentsResponse {
	next, previous := "next-segment", "previous-segment"
	return &responses.PaginatedSegmentsResponse{
		Object:     "list",
		Data:       []responses.Segment{segmentFixture()},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next, PreviousCursor: &previous},
	}
}

func segmentMembersPageFixture() *responses.PaginatedSegmentContactsResponse {
	next := "next-member"
	return &responses.PaginatedSegmentContactsResponse{
		Object: "list",
		Data: []responses.SegmentContact{{
			Object:    "segment_contact",
			SegmentID: uuid.MustParse("33333333-3333-4333-8333-333333333333"),
			ContactID: uuid.MustParse("11111111-1111-4111-8111-111111111111"),
			Email:     "person@example.com",
			EnteredAt: time.Date(2026, time.September, 10, 9, 0, 0, 0, time.UTC),
		}},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next},
	}
}

// segmentPreviewFixture returns a preview of count contacts, or of a count
// the API did not give for nil, with two sample contacts.
func segmentPreviewFixture(count *int) *responses.SegmentPreview {
	return &responses.SegmentPreview{
		Object: "segment_preview",
		Count:  count,
		Contacts: []responses.SegmentPreviewContact{
			{ContactID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Email: "a@example.com"},
			{ContactID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), Email: "b@example.com"},
		},
	}
}

func TestSegmentJSONDecodesIntoSDKTypes(t *testing.T) {
	count := 12
	segment, detail := segmentFixture(), segmentDetailFixture()
	tests := map[string]struct {
		render  func(ResponseHandler) error
		decoded any
		want    any
	}{
		"list": {func(h ResponseHandler) error { return h.HandleSegmentList(segmentsPageFixture(), ListConfig{}) },
			&responses.PaginatedSegmentsResponse{}, segmentsPageFixture()},
		"single": {func(h ResponseHandler) error { return h.HandleSingleSegment(&segment, SingleConfig{}) },
			&responses.Segment{}, &segment},
		"detail": {func(h ResponseHandler) error { return h.HandleSegmentDetail(&detail, SingleConfig{}) },
			&responses.SegmentDetail{}, &detail},
		"preview": {func(h ResponseHandler) error {
			return h.HandleSegmentPreview(segmentPreviewFixture(&count), SingleConfig{})
		},
			&responses.SegmentPreview{}, segmentPreviewFixture(&count)},
		"members": {func(h ResponseHandler) error {
			return h.HandleSegmentMemberList(segmentMembersPageFixture(), ListConfig{})
		}, &responses.PaginatedSegmentContactsResponse{}, segmentMembersPageFixture()},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, test.render(testHandler(t, "json", &output, os.Stderr)))
			require.NoError(t, json.Unmarshal(output.Bytes(), test.decoded))
			assert.Equal(t, test.want, test.decoded)
		})
	}
}

func TestSegmentTableListShowsColumnsAndCursorLines(t *testing.T) {
	useLocalTimeZone(t)
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleSegmentList(segmentsPageFixture(), ListConfig{SuccessMessage: "Segments"}))
	text := output.String()
	assert.Equal(t, [][]string{
		{"NAME", "ID", "STATE", "MEMBERS", "UPDATED"},
		{"Engaged pro customers", "33333333-3333-4333-8333-333333333333", "failed", "42", "2026-09-10 11:30:00 UTC+2"},
	}, tableCells(text))
	assert.True(t, strings.HasSuffix(text, "\nShowing 1 segment (more available)\n--after next-segment\n--before previous-segment\n"), text)
}

func TestSegmentPlainShowsEveryFieldAndTheDefinitionAsIndentedJSON(t *testing.T) {
	useLocalTimeZone(t)
	segment := segmentFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleSegment(&segment, SingleConfig{SuccessMessage: "Segment"}))
	definition := `{
  "schema_version": 1,
  "match": "all",
  "conditions": [
    {
      "field": "attributes.plan",
      "op": "equal",
      "value": "pro"
    },
    {
      "match": "any",
      "conditions": [
        {
          "field": "attributes.country",
          "op": "equal",
          "value": "DE"
        },
        {
          "field": "campaign_count",
          "op": "opened_at_least",
          "value": {
            "count": 3,
            "since": "90 days ago"
          }
        }
      ]
    }
  ]
}`
	// Plain output starts each further line of a value under its first line.
	definition = strings.ReplaceAll(definition, "\n", "\n"+strings.Repeat(" ", len("Definition: ")))
	assert.Equal(t, "Segment\n"+
		"Name: Engaged pro customers\nID: 33333333-3333-4333-8333-333333333333\n"+
		"Description: Pro plan, opened lately\nState: failed\n"+
		"Failure Reason: the members could not be found in time\nMembers: 42\nTimezone: Europe/Berlin\n"+
		"Last Evaluated: 2026-09-10 11:00:00 UTC+2\nCreated: 2026-09-10 10:30:00 UTC+2\nUpdated: 2026-09-10 11:30:00 UTC+2\n"+
		"Definition: "+definition+"\n", output.String())
}

func TestSegmentCSVWritesTheDefinitionAsCompactJSON(t *testing.T) {
	segment := segmentFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleSegment(&segment, SingleConfig{}))
	headers, rows := readCSV(t, output.String())
	assert.Equal(t, segmentFields, headers)
	require.Len(t, rows, 1)
	assert.Equal(t, segmentDefinitionJSON, rows[0]["definition"])
	assert.Equal(t, "2026-09-10T09:00:00Z", rows[0]["last_evaluated_at"])
	assert.Equal(t, "42", rows[0]["member_count"])
}

// TestSegmentWithoutAFailureOrAnEvaluationLeavesThemOut checks that a ready
// segment's unset failure reason, and the last evaluation of one whose first
// build has not ended, are missing values: "-" in a table, left out of plain
// output and empty in CSV.
func TestSegmentWithoutAFailureOrAnEvaluationLeavesThemOut(t *testing.T) {
	segment := segmentFixture()
	segment.State = common.SegmentStateBuilding
	segment.FailureReason = nil
	segment.LastEvaluatedAt = nil
	render := func(h ResponseHandler) error { return h.HandleSingleSegment(&segment, SingleConfig{}) }

	table := tableCells(renderWith(t, "table", render))
	assert.Contains(t, table, []string{"Failure Reason", "-"})
	assert.Contains(t, table, []string{"Last Evaluated", "-"})
	plain := renderWith(t, "plain", render)
	assert.NotContains(t, plain, "Failure Reason")
	assert.NotContains(t, plain, "Last Evaluated")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Empty(t, rows[0]["failure_reason"])
	assert.Empty(t, rows[0]["last_evaluated_at"])
}

func TestSegmentDetailShowsTheMarketableCountAfterTheMembers(t *testing.T) {
	segment := segmentDetailFixture()
	render := func(h ResponseHandler) error { return h.HandleSegmentDetail(&segment, SingleConfig{}) }

	assert.Contains(t, renderWith(t, "plain", render), "\nMembers: 42\nMarketable: 40\nTimezone: Europe/Berlin\n")
	headers, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, segmentDetailFields, headers)
	assert.Equal(t, "40", rows[0]["marketable_count"])
}

func TestSegmentDetailShowsUnknownForANullMarketableCount(t *testing.T) {
	segment := segmentDetailFixture()
	segment.MarketableCount = nil
	render := func(h ResponseHandler) error { return h.HandleSegmentDetail(&segment, SingleConfig{}) }

	assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Marketable", "unknown"})
	assert.Contains(t, renderWith(t, "plain", render), "\nMarketable: unknown\n")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Empty(t, rows[0]["marketable_count"])
}

func TestSegmentPreviewShowsTheCountThenTheSampleContacts(t *testing.T) {
	count := 12
	render := func(h ResponseHandler) error {
		return h.HandleSegmentPreview(segmentPreviewFixture(&count), SingleConfig{})
	}

	table := renderWith(t, "table", render)
	assert.True(t, strings.HasPrefix(table, "Count: 12\n\n┌"), table)
	assert.Equal(t, [][]string{
		{"EMAIL", "CONTACT ID"},
		{"a@example.com", "11111111-1111-4111-8111-111111111111"},
		{"b@example.com", "22222222-2222-4222-8222-222222222222"},
	}, tableCells(table))
	assert.Equal(t, "Count: 12\n"+
		"\nEmail: a@example.com\nContact ID: 11111111-1111-4111-8111-111111111111\n"+
		"\nEmail: b@example.com\nContact ID: 22222222-2222-4222-8222-222222222222\n", renderWith(t, "plain", render))
}

func TestSegmentPreviewWithANullCountSaysItWasNotCountedInTime(t *testing.T) {
	render := func(h ResponseHandler) error {
		return h.HandleSegmentPreview(segmentPreviewFixture(nil), SingleConfig{})
	}

	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			output := renderWith(t, format, render)
			assert.True(t, strings.HasPrefix(output, "Count: not counted in time\n"), output)
			assert.Contains(t, output, "a@example.com")
			assert.Contains(t, output, "b@example.com")
		})
	}
}

func TestSegmentPreviewCSVHasARowPerSampleContactWithTheCount(t *testing.T) {
	count := 12
	for name, test := range map[string]struct {
		count *int
		want  string
	}{
		"counted":   {&count, "12"},
		"uncounted": {nil, ""},
	} {
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "csv", func(h ResponseHandler) error {
				return h.HandleSegmentPreview(segmentPreviewFixture(test.count), SingleConfig{})
			})
			assert.Equal(t, "count,email,contact_id\n"+
				test.want+",a@example.com,11111111-1111-4111-8111-111111111111\n"+
				test.want+",b@example.com,22222222-2222-4222-8222-222222222222\n", output)
		})
	}
}

func TestSegmentPreviewWithoutContactsShowsTheCountAlone(t *testing.T) {
	count := 0
	preview := &responses.SegmentPreview{Object: "segment_preview", Count: &count, Contacts: []responses.SegmentPreviewContact{}}
	render := func(h ResponseHandler) error { return h.HandleSegmentPreview(preview, SingleConfig{}) }

	assert.Equal(t, "Count: 0\n", renderWith(t, "table", render))
	assert.Equal(t, "Count: 0\n", renderWith(t, "plain", render))
	assert.Equal(t, "count,email,contact_id\n0,,\n", renderWith(t, "csv", render))
}

func TestSegmentMembersShowEmailContactIDAndEntryTime(t *testing.T) {
	useLocalTimeZone(t)
	render := func(h ResponseHandler) error {
		return h.HandleSegmentMemberList(segmentMembersPageFixture(), ListConfig{SuccessMessage: "Members"})
	}

	table := renderWith(t, "table", render)
	assert.Equal(t, [][]string{
		{"EMAIL", "CONTACT ID", "ENTERED"},
		{"person@example.com", "11111111-1111-4111-8111-111111111111", "2026-09-10 11:00:00 UTC+2"},
	}, tableCells(table))
	assert.True(t, strings.HasSuffix(table, "\nShowing 1 segment member (more available)\n--after next-member\n"), table)
	assert.Equal(t, "email,contact_id,entered_at\nperson@example.com,11111111-1111-4111-8111-111111111111,2026-09-10T09:00:00Z\n",
		renderWith(t, "csv", render))
}

func TestSegmentEmptyPagesShowTheEmptyMessage(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			segments := renderWith(t, format, func(h ResponseHandler) error {
				return h.HandleSegmentList(&responses.PaginatedSegmentsResponse{Object: "list"}, ListConfig{EmptyMessage: "No segments"})
			})
			assert.Equal(t, "No segments\n", segments)
			members := renderWith(t, format, func(h ResponseHandler) error {
				return h.HandleSegmentMemberList(&responses.PaginatedSegmentContactsResponse{Object: "list"}, ListConfig{EmptyMessage: "No members"})
			})
			assert.Equal(t, "No members\n", members)
		})
	}
}
