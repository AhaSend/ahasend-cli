package printer

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContactJSONPreservesListEnvelopeAndRecursiveAttributes(t *testing.T) {
	response := contactListFixture()
	var output bytes.Buffer
	handler := testHandler(t, "json", &output, os.Stderr)

	require.NoError(t, handler.HandleContactList(response, ListConfig{}))

	var actual map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &actual))
	assert.Equal(t, "list", actual["object"])
	pagination := actual["pagination"].(map[string]any)
	assert.Equal(t, true, pagination["has_more"])
	assert.Equal(t, "next-contact", pagination["next_cursor"])
	contacts := actual["data"].([]any)
	contact := contacts[0].(map[string]any)
	assert.Equal(t, "normalized@example.com", contact["email"])
	attributes := contact["attributes"].(map[string]any)
	assert.Equal(t, map[string]any{
		"empty": map[string]any{},
		"profile": map[string]any{
			"flags": []any{true, nil, "vip"},
			"score": float64(7),
		},
	}, attributes["additionalproperties"])
}

func TestContactJSONDecodesIntoSDKTypes(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleContactList(contactListFixture(), ListConfig{}))
		var decoded responses.PaginatedContactsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *contactListFixture(), decoded)
	})
	t.Run("single", func(t *testing.T) {
		contact := contactFixture()
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{}))
		var decoded responses.Contact
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, contact, decoded)
	})
	t.Run("batch", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, testHandler(t, "json", &output, os.Stderr).HandleBatchContacts(contactBatchFixture(), SingleConfig{}))
		var decoded responses.BatchUpsertContactsResponse
		require.NoError(t, json.Unmarshal(output.Bytes(), &decoded))
		assert.Equal(t, *contactBatchFixture(), decoded)
	})
}

func TestContactJSONPreservesEveryBatchResult(t *testing.T) {
	response := contactBatchFixture()
	var output bytes.Buffer
	handler := testHandler(t, "json", &output, os.Stderr)

	require.NoError(t, handler.HandleBatchContacts(response, SingleConfig{}))

	var actual map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &actual))
	assert.Equal(t, "list", actual["object"])
	assert.Equal(t, float64(1), actual["created"])
	assert.Equal(t, float64(1), actual["failed"])
	results := actual["data"].([]any)
	require.Len(t, results, 2)
	success := results[0].(map[string]any)
	assert.Equal(t, float64(0), success["position"])
	assert.Equal(t, "normalized@example.com", success["email"])
	assert.Equal(t, "created", success["outcome"])
	assert.NotContains(t, success, "reason")
	assert.Equal(t, "normalized@example.com", success["contact"].(map[string]any)["email"])
	failure := results[1].(map[string]any)
	assert.Equal(t, float64(1), failure["position"])
	assert.Equal(t, "bad@example.com", failure["email"])
	assert.Equal(t, "failed", failure["outcome"])
	assert.Equal(t, "email is suppressed", failure["reason"])
	assert.NotContains(t, failure, "contact")
}

func TestContactCSVPreservesFieldsAndStableAttributes(t *testing.T) {
	response := contactListFixture()
	var output bytes.Buffer
	handler := testHandler(t, "csv", &output, os.Stderr)

	require.NoError(t, handler.HandleContactList(response, ListConfig{}))
	headers, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	require.Equal(t, contactFields, headers)
	row := rows[0]
	assert.Equal(t, "normalized@example.com", row["email"])
	assert.Equal(t, "active", row["status"])
	assert.Equal(t, "false", row["unsubscribed"])
	assert.Equal(t,
		`{"additionalproperties":{"empty":{},"profile":{"flags":[true,null,"vip"],"score":7}},"segment":"customers"}`,
		row["attributes"])

	var attributes map[string]any
	require.NoError(t, json.Unmarshal([]byte(row["attributes"]), &attributes))
	assert.Equal(t, "customers", attributes["segment"])
}

func TestContactSingleCSVWritesOneRow(t *testing.T) {
	contact := contactFixture()
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{}))
	_, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, "normalized@example.com", rows[0]["email"])
}

func TestContactCSVPutsEachFieldOnOneLine(t *testing.T) {
	contact := contactFixture()
	contact.StatusReason = "bounced:\r\nmailbox\tfull"
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{}))
	_, rows := readCSV(t, output.String())
	require.Len(t, rows, 1)
	assert.Equal(t, "bounced: mailbox full", rows[0]["status_reason"])
}

func TestContactBatchCSVWritesOneRowPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "csv", &output, os.Stderr).HandleBatchContacts(contactBatchFixture(), SingleConfig{}))
	records, err := csv.NewReader(bytes.NewReader(output.Bytes())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	assert.Equal(t, []string{"position", "email", "contact_id", "outcome", "reason"}, records[0])
	assert.Equal(t, []string{"0", "normalized@example.com", "11111111-1111-4111-8111-111111111111", "created", ""}, records[1])
	assert.Equal(t, []string{"1", "bad@example.com", "", "failed", "email is suppressed"}, records[2])
}

func TestContactHumanFormatsPreserveCoreFields(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleContactList(contactListFixture(), ListConfig{
				SuccessMessage: "Contacts retrieved",
			}))
			text := output.String()
			assert.Contains(t, text, "normalized@example.com")
			assert.Contains(t, strings.ToLower(text), "active")
			assert.Contains(t, text, formatBooleanStatus(false))
			assert.Contains(t, text,
				`{"additionalproperties":{"empty":{},"profile":{"flags":[true,null,"vip"],"score":7}},"segment":"customers"}`)
		})
	}
}

func TestContactListTablePutsEmailFirst(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleContactList(contactListFixture(), ListConfig{}))
	assert.Equal(t, []string{"EMAIL", "ID", "STATUS", "UNSUBSCRIBED", "ATTRIBUTES"}, tableCells(output.String())[0])
	assert.Equal(t, []string{"normalized@example.com", "11111111-1111-4111-8111-111111111111"}, tableCells(output.String())[1][:2])
}

func TestContactDetailTableShowsEveryRow(t *testing.T) {
	useLocalTimeZone(t)
	contact := contactFixture()
	contact.Attributes = nil
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{SuccessMessage: "Contact"}))
	assert.Equal(t, [][]string{
		{"FIELD", "VALUE"},
		{"Email", "normalized@example.com"},
		{"ID", "11111111-1111-4111-8111-111111111111"},
		{"First Name", "Normal"},
		{"Last Name", "Contact"},
		{"Status", "active"},
		{"Status Reason", "confirmed"},
		{"Unsubscribed", "No"},
		{"Unsubscribed At", "-"},
		{"Attributes", "-"},
		{"Validation Status", "valid"},
		{"Last Validated At", "-"},
		{"Created", "2026-09-10 10:30:00 UTC+2"},
		{"Updated", "2026-09-10 11:30:00 UTC+2"},
	}, tableCells(output.String()))
}

func TestContactDetailPlainSkipsUnsetFields(t *testing.T) {
	useLocalTimeZone(t)
	contact := contactFixture()
	contact.FirstName = ""
	contact.Attributes = nil
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleSingleContact(&contact, SingleConfig{SuccessMessage: "Contact"}))
	assert.Equal(t, "Contact\n"+
		"Email: normalized@example.com\nID: 11111111-1111-4111-8111-111111111111\nLast Name: Contact\n"+
		"Status: active\nStatus Reason: confirmed\nUnsubscribed: No\nValidation Status: valid\n"+
		"Created: 2026-09-10 10:30:00 UTC+2\nUpdated: 2026-09-10 11:30:00 UTC+2\n", output.String())
}

func TestContactBatchTableShowsOneRowPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "table", &output, os.Stderr).HandleBatchContacts(contactBatchFixture(), SingleConfig{
		SuccessMessage: "Batch complete",
	}))
	assert.Contains(t, output.String(), "Created: 1  Updated: 0  Failed: 1")
	assert.Equal(t, [][]string{
		{"POSITION", "EMAIL", "CONTACT ID", "OUTCOME", "REASON"},
		{"0", "normalized@example.com", "11111111-1111-4111-8111-111111111111", "created", "-"},
		{"1", "bad@example.com", "-", "failed", "email is suppressed"},
	}, tableCells(output.String()))
}

func TestContactBatchPlainShowsOneBlockPerResult(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, testHandler(t, "plain", &output, os.Stderr).HandleBatchContacts(contactBatchFixture(), SingleConfig{
		SuccessMessage: "Batch complete",
	}))
	assert.Equal(t, "Batch complete\nCreated: 1\nUpdated: 0\nFailed: 1\n"+
		"\nResult 1:\n  Position: 0\n  Email: normalized@example.com\n"+
		"  Contact ID: 11111111-1111-4111-8111-111111111111\n  Outcome: created\n"+
		"\nResult 2:\n  Position: 1\n  Email: bad@example.com\n  Outcome: failed\n"+
		"  Reason: email is suppressed\n", output.String())
}

func TestContactListHumanFormatsEndWithCursorLines(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			response := contactListFixture()
			previous := "previous-contact"
			response.Pagination.PreviousCursor = &previous

			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleContactList(response, ListConfig{}))
			assert.True(t, strings.HasSuffix(output.String(), "--after next-contact\n--before previous-contact\n"), output.String())
		})
	}
}

func TestContactEmptyPageShowsMessageAndPreviousCursor(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			previous := "previous-contact"
			response := &responses.PaginatedContactsResponse{
				Object:     "list",
				Pagination: responses.ContactPagination{PreviousCursor: &previous},
			}

			var output bytes.Buffer
			require.NoError(t, testHandler(t, format, &output, os.Stderr).HandleContactList(response, ListConfig{EmptyMessage: "No contacts"}))
			assert.Equal(t, "No contacts\n--before previous-contact\n", output.String())
		})
	}
}

func TestContactSingleRendersInEveryFormat(t *testing.T) {
	for _, format := range GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			contact := contactFixture()
			var output bytes.Buffer
			handler := testHandler(t, format, &output, os.Stderr)
			require.NoError(t, handler.HandleSingleContact(&contact, SingleConfig{SuccessMessage: "Contact created"}))
			assert.Contains(t, output.String(), "normalized@example.com")
		})
	}
}

func contactListFixture() *responses.PaginatedContactsResponse {
	next := "next-contact"
	return &responses.PaginatedContactsResponse{
		Object: "list",
		Data:   []responses.Contact{contactFixture()},
		Pagination: responses.ContactPagination{
			HasMore:    true,
			NextCursor: &next,
		},
	}
}

func contactBatchFixture() *responses.BatchUpsertContactsResponse {
	contact := contactFixture()
	return &responses.BatchUpsertContactsResponse{
		Object:  "list",
		Created: 1,
		Failed:  1,
		Data: []responses.BatchContactResult{
			{Position: 0, Email: contact.Email, Outcome: "created", Contact: &contact},
			{Position: 1, Email: "bad@example.com", Outcome: "failed", Reason: "email is suppressed"},
		},
	}
}

// contactFixture holds attributes as JSON decodes them (float64 numbers, []any
// arrays), so JSON output decodes back into an equal contact.
func contactFixture() responses.Contact {
	createdAt := time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	return responses.Contact{
		Object:       "contact",
		ID:           uuid.MustParse("11111111-1111-4111-8111-111111111111"),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Email:        "normalized@example.com",
		FirstName:    "Normal",
		LastName:     "Contact",
		Status:       "active",
		StatusReason: "confirmed",
		Unsubscribed: false,
		Attributes: map[string]any{
			"segment": "customers",
			"additionalproperties": map[string]any{
				"profile": map[string]any{"score": float64(7), "flags": []any{true, nil, "vip"}},
				"empty":   map[string]any{},
			},
		},
		ValidationStatus: "valid",
	}
}

// tableCells returns the cells of each header and data line of a rendered
// table, in order, with the borders and padding removed.
func tableCells(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "│"), "│")
		cells := make([]string, len(parts))
		for i, part := range parts {
			cells[i] = strings.TrimSpace(part)
		}
		rows = append(rows, cells)
	}
	return rows
}
