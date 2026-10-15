package domains

import (
	"strconv"
	"strings"
	"testing"
	"time"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Sending type, DKIM selector and pause state -----------------------------------

// newPausedDomain returns a verified marketing domain paused for its bounce
// rate. It is verified so that verify and check-dns print no DNS help.
func newPausedDomain() *responses.Domain {
	pausedAt := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	reason := responses.DomainPauseReasonBounceRate
	return &responses.Domain{
		Object:      "domain",
		ID:          uuid.MustParse("55555555-5555-5555-5555-555555555555"),
		Domain:      "news.example.com",
		DNSValid:    true,
		CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		SendingType: common.DomainSendingTypeMarketing,
		Paused:      true,
		PausedAt:    &pausedAt,
		PauseReason: &reason,
	}
}

func TestCreateCommand_SendsSendingTypeAndDKIMSelector(t *testing.T) {
	sendingType, selector := "marketing", "news"
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateDomain", requests.CreateDomainRequest{
		Domain:       "news.example.com",
		SendingType:  &sendingType,
		DKIMSelector: &selector,
	}).Return(newPausedDomain(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "table", "news.example.com",
		"--sending-type", "marketing", "--dkim-selector", "news", "--no-dns-help")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestCreateCommand_OmitsUnsetSendingTypeAndDKIMSelector(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateDomain", requests.CreateDomainRequest{Domain: "example.com"}).
		Return(newPausedDomain(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewCreateCommand(), "table", "example.com", "--no-dns-help")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestEditCommand_SendsSendingTypeAndDKIMSelector(t *testing.T) {
	sendingType, selector := "transactional", "s2026"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateDomain", "example.com", requests.UpdateDomainRequest{
		SendingType:  &sendingType,
		DKIMSelector: &selector,
	}).Return(newPausedDomain(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "table", "example.com",
		"--sending-type", "transactional", "--dkim-selector", "s2026")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestEditCommand_SendsSubdomains(t *testing.T) {
	tracking, returnPath, subscription, media := "track", "bounce", "prefs", "cdn"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateDomain", "example.com", requests.UpdateDomainRequest{
		TrackingSubdomain:     &tracking,
		ReturnPathSubdomain:   &returnPath,
		SubscriptionSubdomain: &subscription,
		MediaSubdomain:        &media,
	}).Return(newPausedDomain(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "table", "example.com",
		"--tracking-subdomain", "track", "--return-path-subdomain", "bounce",
		"--subscription-subdomain", "prefs", "--media-subdomain", "cdn")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestEditCommand_EmptyDKIMSelectorIsSent(t *testing.T) {
	empty := ""
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateDomain", "example.com", requests.UpdateDomainRequest{DKIMSelector: &empty}).
		Return(newPausedDomain(), nil)

	_, err := testutil.RunWithClient(t, mockClient, NewUpdateCommand(), "table", "example.com", "--dkim-selector", "")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListCommand_SendsSendingType(t *testing.T) {
	sendingType := "marketing"
	mockClient := &mocks.MockClient{}
	mockClient.On("ListDomains", requests.GetDomainsParams{SendingType: &sendingType}).
		Return(&responses.PaginatedDomainsResponse{Data: []responses.Domain{*newPausedDomain()}}, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "table", "--sending-type", "marketing")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestListCommand_StatusSendsDNSValidFilterAndPrintsListAsReturned(t *testing.T) {
	body := `{"object":"list","data":[{"domain":"example.com","future_field":1.50}],"pagination":{"has_more":false}}`
	tests := []struct {
		status   string
		dnsValid bool
	}{
		{"verified", true},
		{"pending", false},
		{"failed", false},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("ListDomains", requests.GetDomainsParams{DNSValid: &tt.dnsValid}).
				Return(mocks.APIResponse[responses.PaginatedDomainsResponse](t, body), nil)

			out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "json", "--status", tt.status)

			require.NoError(t, err)
			assert.Equal(t, testutil.IndentedJSON(t, body), out)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestListCommand_EmptySendingTypeSendsNoFilter(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListDomains", requests.GetDomainsParams{}).
		Return(&responses.PaginatedDomainsResponse{Data: []responses.Domain{*newPausedDomain()}}, nil)

	_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "table", "--sending-type", "")
	require.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDomainFlagsValidatedBeforeAuthentication(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		args    []string
		message string
	}{
		{"create empty sending type", NewCreateCommand(), []string{"example.com", "--sending-type", ""},
			"--sending-type cannot be empty"},
		{"update empty sending type", NewUpdateCommand(), []string{"example.com", "--sending-type", ""},
			"--sending-type cannot be empty"},
		{"update no settings", NewUpdateCommand(), []string{"example.com"},
			"at least one of --tracking-subdomain, --return-path-subdomain, --subscription-subdomain, --media-subdomain, --dkim-rotation-interval, --dkim-selector or --sending-type must be provided"},
		{"update selector of two labels", NewUpdateCommand(), []string{"example.com", "--dkim-selector", "a.b"},
			`invalid --dkim-selector value "a.b": must be one DNS label of 1-62 letters, digits and hyphens, without a leading or trailing hyphen`},
		{"create selector with a leading hyphen", NewCreateCommand(), []string{"example.com", "--dkim-selector", "-news"},
			`invalid --dkim-selector value "-news": must be one DNS label of 1-62 letters, digits and hyphens, without a leading or trailing hyphen`},
		{"create selector of 63 characters", NewCreateCommand(), []string{"example.com", "--dkim-selector", strings.Repeat("a", 63)},
			`invalid --dkim-selector value "` + strings.Repeat("a", 63) + `": must be one DNS label of 1-62 letters, digits and hyphens, without a leading or trailing hyphen`},
		{"list unknown status", NewListCommand(), []string{"--status", "active"},
			"--status must be one of: verified, pending, failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			authCalled := testutil.RefuseAuth(t)
			_, err := testutil.RunPlain(t, test.cmd, test.args...)
			require.Error(t, err)
			assert.Equal(t, test.message, err.Error())
			assert.False(t, *authCalled)
		})
	}
}

// TestUpdateLeavesTheRotationIntervalToTheAPI checks that any number of
// days reaches the API, which states no range and decides itself.
func TestUpdateLeavesTheRotationIntervalToTheAPI(t *testing.T) {
	for _, days := range []int{7, 30, 180, 365} {
		t.Run(strconv.Itoa(days), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("UpdateDomain", "example.com", mock.MatchedBy(func(req requests.UpdateDomainRequest) bool {
				return req.DKIMRotationIntervalDays != nil && *req.DKIMRotationIntervalDays == days
			})).Return(&responses.Domain{Domain: "example.com"}, nil)
			testutil.InjectClient(t, mockClient)

			_, err := testutil.RunPlain(t, NewUpdateCommand(), "example.com", "--dkim-rotation-interval", strconv.Itoa(days))
			require.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

// TestDomainArgumentValidatedBeforePromptAndAuthentication checks that every
// command that puts the domain argument in a path rejects one that cannot be
// a path segment before it prompts or authenticates.
func TestDomainArgumentValidatedBeforePromptAndAuthentication(t *testing.T) {
	commands := map[string]struct {
		newCommand func() *cobra.Command
		flags      []string
	}{
		"get":       {NewGetCommand, nil},
		"delete":    {NewDeleteCommand, nil},
		"verify":    {NewVerifyCommand, nil},
		"check-dns": {NewCheckDNSCommand, nil},
		"update":    {NewUpdateCommand, []string{"--sending-type", "marketing"}},
	}
	values := map[string]string{"": "domain cannot be empty", "..": `invalid domain value "..": not a domain`, ".": `invalid domain value ".": not a domain`}
	for name, command := range commands {
		for value, message := range values {
			t.Run(name+" "+value, func(t *testing.T) {
				authCalled := testutil.RefuseAuth(t)

				args := append([]string{value}, command.flags...)
				_, stderr, err := testutil.RunCommand(t, command.newCommand(), "plain", "y\n", args...)
				require.Error(t, err)
				assert.Equal(t, message, err.Error())
				assert.Equal(t, 4, clierrors.GetExitCode(err))
				assert.NotContains(t, stderr, "Are you sure")
				assert.False(t, *authCalled)
			})
		}
	}
}

// TestListCommand_SendsLimitAndCursors checks that the page flags reach the
// API as limit, after and before, and that the deprecated --cursor is
// --after.
func TestListCommand_SendsLimitAndCursors(t *testing.T) {
	limit, cursor := int32(10), "next-page"
	tests := map[string]struct {
		args []string
		want common.PaginationParams
	}{
		"after":  {[]string{"--after", cursor}, common.PaginationParams{Limit: &limit, After: &cursor}},
		"before": {[]string{"--before", cursor}, common.PaginationParams{Limit: &limit, Before: &cursor}},
		"cursor": {[]string{"--cursor", cursor}, common.PaginationParams{Limit: &limit, After: &cursor}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("ListDomains", requests.GetDomainsParams{PaginationParams: test.want}).
				Return(&responses.PaginatedDomainsResponse{Data: []responses.Domain{*newPausedDomain()}}, nil)

			_, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "table", append([]string{"--limit", "10"}, test.args...)...)
			require.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDomainCommands_TableShowsSendingTypeAndPause(t *testing.T) {
	domain := newPausedDomain()
	tests := []struct {
		name  string
		setup func(*mocks.MockClient)
		cmd   *cobra.Command
		args  []string
	}{
		{
			name:  "get",
			setup: func(m *mocks.MockClient) { m.On("GetDomain", domain.Domain).Return(domain, nil) },
			cmd:   NewGetCommand(),
			args:  []string{domain.Domain},
		},
		{
			name: "create",
			setup: func(m *mocks.MockClient) {
				m.On("CreateDomain", requests.CreateDomainRequest{Domain: domain.Domain}).Return(domain, nil)
			},
			cmd:  NewCreateCommand(),
			args: []string{domain.Domain, "--no-dns-help"},
		},
		{
			name: "edit",
			setup: func(m *mocks.MockClient) {
				m.On("UpdateDomain", domain.Domain, requests.UpdateDomainRequest{SendingType: &domain.SendingType}).Return(domain, nil)
			},
			cmd:  NewUpdateCommand(),
			args: []string{domain.Domain, "--sending-type", domain.SendingType},
		},
		{
			name:  "verify",
			setup: func(m *mocks.MockClient) { m.On("GetDomain", domain.Domain).Return(domain, nil) },
			cmd:   NewVerifyCommand(),
			args:  []string{domain.Domain},
		},
		{
			name:  "check-dns",
			setup: func(m *mocks.MockClient) { m.On("CheckDomainDNS", domain.Domain).Return(domain, nil) },
			cmd:   NewCheckDNSCommand(),
			args:  []string{domain.Domain},
		},
		{
			name: "list",
			setup: func(m *mocks.MockClient) {
				m.On("ListDomains", requests.GetDomainsParams{}).
					Return(&responses.PaginatedDomainsResponse{Data: []responses.Domain{*domain}}, nil)
			},
			cmd:  NewListCommand(),
			args: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			tt.setup(mockClient)

			out, err := testutil.RunWithClient(t, mockClient, tt.cmd, "table", tt.args...)
			require.NoError(t, err)
			assert.Contains(t, out, "marketing")
			assert.Contains(t, out, "bounce_rate")
			assert.Contains(t, out, formatLocal(*domain.PausedAt))
			mockClient.AssertExpectations(t)
		})
	}
}

// formatLocal formats t the way the table printer does.
func formatLocal(t time.Time) string {
	return t.Local().Format("2006-01-02 15:04:05")
}

// Benchmark tests
func BenchmarkDomainsCommand_Creation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewCommand()
	}
}

func TestListCommand_TablePutsTheDomainThenTheID(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("ListDomains", requests.GetDomainsParams{}).
		Return(&responses.PaginatedDomainsResponse{Data: []responses.Domain{*newPausedDomain()}}, nil)

	out, err := testutil.RunWithClient(t, mockClient, NewListCommand(), "table")
	require.NoError(t, err)
	assert.Regexp(t, `(?m)^│\s+DOMAIN\s+│\s+ID\s+│`, out)
}
