package suppressions

import (
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCommands_CSVIncludesProtected(t *testing.T) {
	suppression := responses.Suppression{
		Object:    "suppression",
		ID:        uuid.MustParse("66666666-6666-6666-6666-666666666666"),
		Email:     "user@example.com",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		ExpiresAt: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Protected: true,
	}
	page := &responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{suppression}}
	tests := []struct {
		name  string
		setup func(*mocks.MockClient)
		cmd   func() *cobra.Command
		args  []string
	}{
		{"list", func(m *mocks.MockClient) {
			m.On("ListSuppressions", mock.AnythingOfType("requests.GetSuppressionsParams")).Return(page, nil)
		}, NewListCommand, nil},
		{"check", func(m *mocks.MockClient) {
			m.On("ListSuppressions", mock.AnythingOfType("requests.GetSuppressionsParams")).Return(page, nil)
		}, NewCheckCommand, []string{"user@example.com"}},
		{"create", func(m *mocks.MockClient) {
			m.On("CreateSuppression", mock.AnythingOfType("requests.CreateSuppressionRequest")).
				Return(&responses.CreateSuppressionResponse{Data: []responses.Suppression{suppression}}, nil)
		}, NewCreateCommand, []string{"user@example.com", "--expires", "30d"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			tt.setup(mockClient)

			out, err := testutil.RunWithClient(t, mockClient, tt.cmd(), "csv", tt.args...)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(out), "\n")
			require.Len(t, lines, 2, out)
			headers := strings.Split(lines[0], ",")
			values := strings.Split(lines[1], ",")
			require.Contains(t, headers, "protected")
			for i, header := range headers {
				if header == "protected" {
					assert.Equal(t, "true", values[i])
				}
			}
			mockClient.AssertExpectations(t)
		})
	}
}

func TestListCommand_TablePutsTheEmailThenTheID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListSuppressions", mock.AnythingOfType("requests.GetSuppressionsParams")).
		Return(&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{{
			ID: uuid.MustParse("66666666-6666-6666-6666-666666666666"), Email: "user@example.com",
		}}}, nil)

	out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "table")
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^│\s+EMAIL\s+│\s+ID\s+│`, out)
	assert.Contains(t, out, "66666666-6666-6666-6666-666666666666")
}
