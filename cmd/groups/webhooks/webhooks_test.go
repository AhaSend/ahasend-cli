package webhooks

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetCommand_RequiresWebhookID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test with no arguments - should fail
	err := getCmd.Args(getCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments - should fail
	err = getCmd.Args(getCmd, []string{"webhook1", "webhook2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")

	// Test with exactly one argument - should pass
	err = getCmd.Args(getCmd, []string{"valid-webhook-id"})
	assert.NoError(t, err)
}

// Benchmark tests
func BenchmarkWebhooksCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

// Helper functions for creating test data

func createTestWebhook(idStr, name, url string, enabled bool) responses.Webhook {
	// Parse string ID to UUID
	id, err := uuid.Parse(idStr)
	if err != nil {
		// If invalid UUID, create a new one
		id = uuid.New()
	}

	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-1 * time.Hour)
	scope := "account"

	return responses.Webhook{
		ID:        id,
		Name:      name,
		URL:       url,
		Enabled:   enabled,
		Scope:     scope,
		Domains:   []string{"example.com", "test.com"},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

// Test output format validation
func TestWebhooksList_OutputFormats(t *testing.T) {
	webhook := createTestWebhook(uuid.New().String(), "Test Webhook", "https://example.com/webhook", true)
	testWebhooks := []responses.Webhook{webhook}

	mockResponse := &responses.PaginatedWebhooksResponse{
		Object: "list",
		Data:   testWebhooks,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test JSON output format
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)

	// Test table format data
	webhook1 := mockResponse.Data[0]
	assert.Equal(t, "Test Webhook", webhook1.Name)
	assert.Equal(t, "https://example.com/webhook", webhook1.URL)
	assert.True(t, webhook1.Enabled)
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete webhook 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesWithoutFetchingTheWebhook(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteWebhook", uuid.MustParse("11111111-1111-1111-1111-111111111111")).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "GetWebhook", mock.Anything)
}

func TestWebhooksList_EnabledIsFilteredByTheAPI(t *testing.T) {
	// The API's page is printed as it came, so a webhook the API returned
	// is shown whatever its status.
	const body = `{"object":"list","data":[{"object":"webhook","id":"11111111-1111-4111-8111-111111111111","name":"Hook","url":"https://example.com/hook","enabled":false}],"pagination":{"has_more":false}}`
	enabled := true
	mockClient := &mocks.MockClient{}
	mockClient.On("ListWebhooks", common.PaginationParams{}, &enabled).
		Return(mocks.APIResponse[responses.PaginatedWebhooksResponse](t, body), nil)

	for _, format := range []string{"json", "plain"} {
		t.Run(format, func(t *testing.T) {
			out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), format, "--enabled")

			require.NoError(t, err)
			if format == "json" {
				assert.Equal(t, testutil.IndentedJSON(t, body), out)
			} else {
				assert.Contains(t, out, "Hook")
			}
		})
	}
	mockClient.AssertExpectations(t)
}

func TestTemporaryWebhookNoteEscapesTheWebhookID(t *testing.T) {
	note := temporaryWebhookNote("id\x1b]0;x\x07")

	assert.Contains(t, note, `'ahasend webhooks delete id\u001b]0;x\u0007'`)
	assert.NotContains(t, note, "\x1b]")
}
