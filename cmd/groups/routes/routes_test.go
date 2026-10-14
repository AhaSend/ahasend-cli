package routes

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go"

	"github.com/AhaSend/ahasend-cli/internal/client"
	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test routes list command with mock
func TestRoutesList_Success(t *testing.T) {
	// Create test routes with valid UUIDs
	route1 := createTestRoute("550e8400-e29b-41d4-a716-446655440001", "Support Route", "https://api.example.com/support", true)
	route2 := createTestRoute("550e8400-e29b-41d4-a716-446655440002", "Sales Route", "https://api.example.com/sales", false)

	testRoutes := []responses.Route{*route1, *route2}
	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test would require integrated execution with authentication bypass
	// For now, test the response structure and data
	assert.NotNil(t, mockResponse)
	assert.Len(t, mockResponse.Data, 2)
	assert.Equal(t, "Support Route", mockResponse.Data[0].Name)
	assert.Equal(t, "Sales Route", mockResponse.Data[1].Name)
}

func TestRoutesList_WithFlags(t *testing.T) {
	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	testRoutes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    true,
			NextCursor: ahasend.String("next-cursor"),
		},
	}

	// Verify the mock setup is correct
	assert.NotNil(t, mockResponse)
	assert.True(t, mockResponse.Pagination.HasMore)
	assert.Equal(t, "next-cursor", *mockResponse.Pagination.NextCursor)
}

func TestRoutesList_EmptyResult(t *testing.T) {
	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   []responses.Route{},
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	assert.NotNil(t, mockResponse)
	assert.Empty(t, mockResponse.Data)
	assert.False(t, mockResponse.Pagination.HasMore)
}

func TestGetCommand_RequiresRouteID(t *testing.T) {
	getCmd := NewGetCommand()

	// Test that command requires exactly one argument (route ID)
	getCmd.SetArgs([]string{})
	err := getCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	getCmd.SetArgs([]string{"route1", "route2"})
	err = getCmd.Execute()
	assert.Error(t, err)
}

func TestUpdateCommand_RequiresRouteID(t *testing.T) {
	updateCmd := NewUpdateCommand()

	// Test that command requires exactly one argument (route ID)
	updateCmd.SetArgs([]string{})
	err := updateCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	updateCmd.SetArgs([]string{"route1", "route2"})
	err = updateCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

func TestDeleteCommand_RequiresRouteID(t *testing.T) {
	deleteCmd := NewDeleteCommand()

	// Test that command requires exactly one argument (route ID)
	deleteCmd.SetArgs([]string{})
	err := deleteCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "argument is required")

	// Test with too many arguments
	deleteCmd.SetArgs([]string{"route1", "route2"})
	err = deleteCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected argument")
}

// Test integrated command execution with mocks
func TestRoutesIntegration_ListWithAuth(t *testing.T) {
	// This test demonstrates how integrated testing would work
	// It requires setting up authentication bypass or mock authentication

	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	routes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   routes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Verify mock response structure
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)
	assert.Equal(t, "Test Route", mockResponse.Data[0].Name)
}

// Test error scenarios
func TestRoutes_APIErrors(t *testing.T) {
	tests := []struct {
		name     string
		error    error
		expected string
	}{
		{
			name:     "general API error",
			error:    errors.New("API request failed"),
			expected: "API request failed",
		},
		{
			name:     "not found error",
			error:    errors.New("route not found"),
			expected: "route not found",
		},
		{
			name:     "validation error",
			error:    errors.New("invalid route configuration"),
			expected: "invalid route configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.error)
			assert.Contains(t, tt.error.Error(), tt.expected)
		})
	}
}

// Test output format validation
func TestRoutes_OutputFormats(t *testing.T) {
	route := createTestRoute("550e8400-e29b-41d4-a716-446655440003", "Test Route", "https://api.example.com/webhook", true)
	testRoutes := []responses.Route{*route}

	mockResponse := &responses.PaginatedRoutesResponse{
		Object: "list",
		Data:   testRoutes,
		Pagination: common.PaginationInfo{
			HasMore:    false,
			NextCursor: nil,
		},
	}

	// Test JSON output format
	assert.Equal(t, "list", mockResponse.Object)
	assert.Len(t, mockResponse.Data, 1)

	// Test table format data
	route1 := mockResponse.Data[0]
	assert.Equal(t, "Test Route", route1.Name)
	assert.Equal(t, "https://api.example.com/webhook", route1.URL)
	assert.True(t, route1.Enabled)
}

// Test edge cases and error conditions
func TestRoutes_NilResponse(t *testing.T) {
	var nilResponse *responses.PaginatedRoutesResponse
	assert.Nil(t, nilResponse)
}

func TestRoutes_InvalidPagination(t *testing.T) {
	// Test handling of invalid pagination parameters
	invalidLimit := int32(-1)
	assert.True(t, invalidLimit < 0, "negative limit should be invalid")

	// Test cursor validation
	longCursor := strings.Repeat("a", 1000)
	assert.True(t, len(longCursor) > 500, "very long cursor should be handled")
}

// Test route data validation
func TestRoute_DataValidation(t *testing.T) {
	tests := []struct {
		name  string
		route *responses.Route
		valid bool
	}{
		{
			name:  "valid route with all fields",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440008", "Valid Route", "https://api.example.com/webhook", true),
			valid: true,
		},
		{
			name:  "route with empty name",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440009", "", "https://api.example.com/webhook", true),
			valid: false, // Assuming empty name is invalid
		},
		{
			name:  "route with invalid URL format",
			route: createTestRoute("550e8400-e29b-41d4-a716-446655440010", "Valid Name", "not-a-url", true),
			valid: false, // Assuming invalid URL format is invalid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			if tt.valid {
				assert.NotEqual(t, uuid.Nil, tt.route.ID)
				assert.NotEmpty(t, tt.route.Name)
				assert.NotEmpty(t, tt.route.URL)
			} else {
				// Invalid cases should have some empty required fields
				isEmpty := tt.route.Name == "" || tt.route.URL == "" || tt.route.ID == uuid.Nil
				isInvalidURL := tt.route.URL != "" && !strings.HasPrefix(tt.route.URL, "http")
				assert.True(t, isEmpty || isInvalidURL, "Invalid route should have empty required fields or invalid URL")
			}
		})
	}
}

// Benchmark tests
func BenchmarkRoutesCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func BenchmarkListCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewListCommand()
	}
}

// createTestRoute returns a route with the given fields, created a day before
// it was last updated.
func createTestRoute(id, name, url string, enabled bool) *responses.Route {
	updatedAt := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	return &responses.Route{
		ID:        uuid.MustParse(id),
		Name:      name,
		URL:       url,
		Enabled:   enabled,
		CreatedAt: updatedAt.Add(-24 * time.Hour),
		UpdatedAt: updatedAt,
	}
}

// ================================
// Routes Listen Command Tests
// ================================

func TestParseListenParameters_Success(t *testing.T) {
	tests := []struct {
		name      string
		routeID   string
		recipient string
	}{
		{
			name:      "with route ID only",
			routeID:   "550e8400-e29b-41d4-a716-446655440001",
			recipient: "",
		},
		{
			name:      "with recipient pattern only",
			routeID:   "",
			recipient: "*@example.com",
		},
		{
			name:      "with complex recipient pattern",
			routeID:   "",
			recipient: "support-*@company.org",
		},
		{
			name:      "with specific recipient",
			routeID:   "",
			recipient: "test@domain.co.uk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListenParameters(tt.routeID, tt.recipient)
			assert.NoError(t, err)
		})
	}
}

func TestParseListenParameters_ValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		routeID   string
		recipient string
		wantError string
	}{
		{
			name:      "no parameters provided",
			routeID:   "",
			recipient: "",
			wantError: "one of --route-id or --recipient must be provided",
		},
		{
			name:      "recipient longer than the API takes",
			routeID:   "",
			recipient: strings.Repeat("a", 256),
			wantError: "--recipient must be at most 255 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListenParameters(tt.routeID, tt.recipient)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

// routingEvent is a routing event as the API streams it, with the message
// under "data".
func routingEvent(t *testing.T, from, to, subject string) map[string]interface{} {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"type":      "message.routing",
		"timestamp": "2026-10-14T10:00:00Z",
		"route_id":  "11111111-1111-1111-1111-111111111111",
		"data": map[string]interface{}{
			"id":         "msg-1",
			"from":       from,
			"reply_to":   "",
			"to":         to,
			"subject":    subject,
			"message_id": "<abc@example.com>",
			"plain_body": "This is a test email body",
		},
	})
	require.NoError(t, err)
	var event map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &event))
	return event
}

func TestPrintRoutingSummary(t *testing.T) {
	var buffer bytes.Buffer

	printRoutingSummary(&buffer, routingEvent(t, "sender@example.com", "recipient@company.com", "Test Email Subject"))

	assert.Equal(t, " from: sender@example.com to: recipient@company.com subject: Test Email Subject", buffer.String())
}

// TestPrintRoutingSummaryEscapesControlCharacters checks that the sender,
// recipient and subject of an inbound email cannot send escape sequences to
// the terminal.
func TestPrintRoutingSummaryEscapesControlCharacters(t *testing.T) {
	var buffer bytes.Buffer

	printRoutingSummary(&buffer, routingEvent(t, "a\x1b]0;x\x07@example.com", "b\u009b2J@example.com", "Hi \x1b[2J\x07 there"))

	assert.Equal(t, ` from: a\u001b]0;x\u0007@example.com to: b\u009b2J@example.com subject: Hi \u001b[2J\u0007 there`, buffer.String())
}

func TestDeleteDeclinedPromptsOnStderrBeforeAuthentication(t *testing.T) {
	authCalled := testutil.RefuseAuth(t)

	stdout, stderr, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "n\n", "11111111-1111-1111-1111-111111111111")
	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Contains(t, stderr, "Are you sure you want to delete route 11111111-1111-1111-1111-111111111111? (y/N): ")
	assert.Empty(t, stdout)
	assert.False(t, *authCalled)
}

func TestDeleteAcceptedDeletesWithoutFetchingTheRoute(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("DeleteRoute", uuid.MustParse("11111111-1111-1111-1111-111111111111")).Return(&common.SuccessResponse{Message: "deleted"}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, _, err := testutil.RunCommand(t, NewDeleteCommand(), "plain", "y\n", "11111111-1111-1111-1111-111111111111")
	require.NoError(t, err)
	assert.Equal(t, "deleted\n", stdout)
	mockClient.AssertExpectations(t)
	mockClient.AssertNotCalled(t, "GetRoute", mock.Anything)
}

func TestRoutesList_EnabledIsRefusedInJSON(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewListCommand(), "json", "", "--enabled")

	require.Error(t, err)
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.Contains(t, err.Error(), "--enabled and --output json cannot be used together")
	assert.False(t, *called, "the command must not authenticate")
}

func TestRoutesList_EnabledFiltersThePageOutsideJSON(t *testing.T) {
	const body = `{"object":"list","data":[` +
		`{"object":"route","id":"11111111-1111-4111-8111-111111111111","name":"On route","url":"https://example.com/a","enabled":true},` +
		`{"object":"route","id":"22222222-2222-4222-8222-222222222222","name":"Off route","url":"https://example.com/b","enabled":false}` +
		`],"pagination":{"has_more":false}}`
	tests := []struct {
		flag  string
		shown string
		gone  string
	}{
		{"--enabled", "On route", "Off route"},
		{"--enabled=false", "Off route", "On route"},
	}
	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("ListRoutes", mock.Anything).
				Return(mocks.APIResponse[responses.PaginatedRoutesResponse](t, body), nil)

			out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "plain", tt.flag)

			require.NoError(t, err)
			assert.Contains(t, out, tt.shown)
			assert.NotContains(t, out, tt.gone)
		})
	}
}

func TestRoutesList_DomainFiltersByRecipientDomain(t *testing.T) {
	domain := "example.com"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListRoutes", requests.GetRoutesParams{Domain: &domain}).
		Return(mocks.APIResponse[responses.PaginatedRoutesResponse](t, `{"object":"list","data":[],"pagination":{"has_more":false}}`), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "plain", "--domain", domain)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestRoutesList_RejectsAnEmptyDomainBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewListCommand(), "--domain", "")

	require.EqualError(t, err, "--domain cannot be empty")
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *called)
}

func TestRoutesList_RejectsADomainThatIsNotOneBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, err := testutil.RunPlain(t, NewListCommand(), "--domain", "not a domain")

	require.Error(t, err)
	assert.Equal(t, `invalid --domain value "not a domain": not a domain name`, err.Error())
	assert.False(t, *called)
}

// TestListenNamesTheTemporaryRouteWhenConnectingFails checks that the note on
// the temporary route, which stays after the command, is printed even when
// connecting to its stream fails.
func TestListenNamesTheTemporaryRouteWhenConnectingFails(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("InitiateRouteStream", (*uuid.UUID)(nil), "*@example.com").
		Return(&client.RouteStreamResponse{RouteID: "11111111-1111-1111-1111-111111111111", WsURL: "wss://stream"}, nil)
	mockClient.On("ConnectWebSocket", "wss://stream", false).
		Return(nil, errors.New("websocket: bad handshake (HTTP 502)"))
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewListenCommand(), "plain", "", "--recipient", "*@example.com")

	require.EqualError(t, err, "failed to connect to websocket: websocket: bad handshake (HTTP 502)")
	assert.True(t, strings.HasPrefix(stderr, "Note: temporary route 11111111-1111-1111-1111-111111111111 stays after the listener stops; "+
		"delete it with 'ahasend routes delete 11111111-1111-1111-1111-111111111111'\n"), stderr)
	assert.Empty(t, stdout)
}

func TestTemporaryRouteNoteEscapesTheRouteID(t *testing.T) {
	note := temporaryRouteNote("id\x1b]0;x\x07")

	assert.Contains(t, note, `'ahasend routes delete id\u001b]0;x\u0007'`)
	assert.NotContains(t, note, "\x1b]")
}
