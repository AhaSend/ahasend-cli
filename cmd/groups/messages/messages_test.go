package messages

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Benchmark tests
// Template ID ---------------------------------------------------------------------

const (
	testAccountID  = "11111111-1111-1111-1111-111111111111"
	testTemplateID = "77777777-7777-7777-7777-777777777777"
)

func newTestMessage(id string, templateID *uuid.UUID) responses.Message {
	return responses.Message{
		Object:     "message",
		ID:         uuid.MustParse(id),
		Sender:     "noreply@example.com",
		Recipient:  "user@example.com",
		Subject:    "Welcome",
		Status:     "Delivered",
		Direction:  "outbound",
		CreatedAt:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TemplateID: templateID,
	}
}

func newListMockClient() *mocks.MockClient {
	templateID := uuid.MustParse(testTemplateID)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(testAccountID)
	mockClient.On("GetMessages", mock.Anything).Return(&responses.PaginatedMessagesResponse{
		Data: []responses.Message{
			newTestMessage("88888888-8888-8888-8888-888888888888", &templateID),
			newTestMessage("99999999-9999-9999-9999-999999999999", nil),
		},
	}, nil)
	return mockClient
}

func TestListCommand_TableShowsTheSummaryColumns(t *testing.T) {
	out, err := testutil.RunWithClient(t, newListMockClient(), NewListCommand(), "table")
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^│\s+SUBJECT\s+│\s+ID\s+│\s+FROM\s+│\s+TO\s+│\s+STATUS\s+│\s+CREATED\s+│$`, out)
}

func TestListCommand_CSVHasTemplateIDColumn(t *testing.T) {
	out, err := testutil.RunWithClient(t, newListMockClient(), NewListCommand(), "csv")
	require.NoError(t, err)

	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	column := -1
	for i, header := range records[0] {
		if header == "template_id" {
			column = i
		}
	}
	require.NotEqual(t, -1, column, "csv header must have template_id")
	assert.Equal(t, testTemplateID, records[1][column])
	assert.Empty(t, records[2][column], "an inline send has no template ID")
}

func TestListCommand_PlainShowsTemplateIDWhenSet(t *testing.T) {
	out, err := testutil.RunWithClient(t, newListMockClient(), NewListCommand(), "plain")
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(out, "Template ID: "+testTemplateID))
	assert.Equal(t, 1, strings.Count(out, "Template ID"))
}

func TestListCommand_ShowDetailsAddsTheDetailFieldsInEveryFormat(t *testing.T) {
	tests := map[string]string{"table": "DIRECTION", "plain": "Direction: outbound", "csv": ",direction,"}
	for format, detail := range tests {
		t.Run(format, func(t *testing.T) {
			out, err := testutil.RunWithClient(t, newListMockClient(), NewListCommand(), format)
			require.NoError(t, err)
			assert.NotContains(t, out, detail)

			out, err = testutil.RunWithClient(t, newListMockClient(), NewListCommand(), format, "--show-details")
			require.NoError(t, err)
			assert.Contains(t, out, detail)
		})
	}
}

func TestGetCommand_ShowsTemplateIDWhenSet(t *testing.T) {
	templateID := uuid.MustParse(testTemplateID)
	message := newTestMessage("88888888-8888-8888-8888-888888888888", &templateID)

	for _, format := range []string{"table", "plain"} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetMessage", message.ID.String()).Return(&message, nil)

			out, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), format, message.ID.String())
			require.NoError(t, err)
			assert.Contains(t, out, "Template ID")
			assert.Contains(t, out, testTemplateID)
		})
	}
}

func TestGetCommand_ShowsTemplateIDAsMissingForInlineSend(t *testing.T) {
	message := newTestMessage("99999999-9999-9999-9999-999999999999", nil)

	for format, check := range map[string]func(t *testing.T, out string){
		"table": func(t *testing.T, out string) { assert.Regexp(t, `│ Template ID +│ - +│`, out) },
		"plain": func(t *testing.T, out string) { assert.NotContains(t, out, "Template ID") },
	} {
		t.Run(format, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetMessage", message.ID.String()).Return(&message, nil)

			out, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), format, message.ID.String())
			require.NoError(t, err)
			assert.Contains(t, out, message.ID.String())
			check(t, out)
		})
	}
}

func TestGetCommand_JSONIncludesTemplateID(t *testing.T) {
	templateID := uuid.MustParse(testTemplateID)
	message := newTestMessage("88888888-8888-8888-8888-888888888888", &templateID)
	mockClient := &mocks.MockClient{}
	mockClient.On("GetMessage", message.ID.String()).Return(&message, nil)

	out, err := testutil.RunWithClient(t, mockClient, NewGetCommand(), "json", message.ID.String())
	require.NoError(t, err)
	assert.Contains(t, out, `"template_id": "`+testTemplateID+`"`)
}

func BenchmarkMessagesCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}
