package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const (
	intCampaignID         = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	intCampaignListID     = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	intCampaignTemplateID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

// campaignNotReadyBody is the 400 body the API answers a send or test of a
// campaign with issues.
const campaignNotReadyBody = `{"message":"campaign is not ready to send","issues":[` +
	`{"code":"sender_missing","message":"Add a sender"},` +
	`{"code":"lists_missing","message":"Choose at least one list"}]}`

// CampaignsIntegrationTestSuite drives the campaigns commands and the nested
// campaign templates group end to end through cmd.NewRootCmdForTesting
// against a MockClient wired in via the auth resolver.
type CampaignsIntegrationTestSuite struct {
	commandSuite
}

func fixedCampaign() *responses.Campaign {
	subject := "October news"
	fromName := "Example"
	editor := "html"
	ready := true
	return &responses.Campaign{
		CampaignListItem: responses.CampaignListItem{
			Object:    "campaign",
			ID:        uuid.MustParse(intCampaignID),
			CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
			Name:      "October",
			Subject:   &subject,
			From:      &common.SenderAddress{Email: "news@example.com", Name: &fromName},
			Status:    common.CampaignStatusDraft,
		},
		ListIDs: []uuid.UUID{uuid.MustParse(intCampaignListID)},
		Editor:  &editor,
		Content: &responses.TemplateContent{HTML: "<p>News {{ unsubscribe_link }}</p>"},
		Ready:   &ready,
		Issues:  []responses.CampaignIssue{},
	}
}

func fixedCampaignsPage() *responses.PaginatedCampaignsResponse {
	next := "next-campaigns"
	return &responses.PaginatedCampaignsResponse{
		Object:     "list",
		Data:       []responses.CampaignListItem{fixedCampaign().CampaignListItem},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next},
	}
}

func fixedCampaignTemplatesPage() *responses.PaginatedCampaignTemplatesResponse {
	return &responses.PaginatedCampaignTemplatesResponse{
		Object: "list",
		Data: []responses.CampaignTemplate{{
			Object: "campaign_template",
			ID:     uuid.MustParse(intCampaignTemplateID),
			Name:   "Newsletter",
			Editor: "advanced",
		}},
	}
}

// TestCampaignWorkflow creates, gets, lists and deletes a campaign.
func (suite *CampaignsIntegrationTestSuite) TestCampaignWorkflow() {
	htmlPath := filepath.Join(suite.T().TempDir(), "october.html")
	html := "<p>News {{ unsubscribe_link }}</p>"
	suite.Require().NoError(os.WriteFile(htmlPath, []byte(html), 0o600))
	subject := "October news"
	fromName := "Example"

	suite.mockClient.On("CreateCampaign", requests.CreateCampaignRequest{
		Name:    "October",
		Subject: &subject,
		From:    &common.SenderAddress{Email: "news@example.com", Name: &fromName},
		ListIDs: []uuid.UUID{uuid.MustParse(intCampaignListID)},
		Content: &requests.TemplateContentInput{HTML: &html},
	}).Return(fixedCampaign(), nil)
	suite.mockClient.On("GetCampaign", uuid.MustParse(intCampaignID)).Return(fixedCampaign(), nil)
	suite.mockClient.On("ListCampaigns", requests.ListCampaignsParams{}).Return(fixedCampaignsPage(), nil)
	suite.mockClient.On("DeleteCampaign", uuid.MustParse(intCampaignID)).Return(&common.SuccessResponse{}, nil)
	called := testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "campaigns", "create", "--name", "October", "--subject", subject,
		"--from", "news@example.com", "--from-name", fromName, "--list-id", intCampaignListID, "--html-file", htmlPath)
	suite.Zero(code)
	suite.Contains(out, "Campaign created successfully")
	suite.Contains(out, intCampaignID)

	out, _, code = execRoot("plain", "", "campaigns", "get", intCampaignID)
	suite.Zero(code)
	suite.Contains(out, "Lists: "+intCampaignListID+"\n")
	suite.Contains(out, "Ready: Yes\n")

	out, _, code = execRoot("table", "", "campaigns", "list")
	suite.Zero(code)
	suite.Contains(out, "October")
	suite.True(strings.HasSuffix(out, "--after next-campaigns\n"), out)

	out, stderr, code := execRoot("plain", "", "campaigns", "delete", intCampaignID, "--force")
	suite.Zero(code)
	suite.Equal("Campaign '"+intCampaignID+"' deleted successfully\n", out)
	suite.Empty(stderr)

	suite.True(*called)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestNestedGroup runs one command of the campaigns templates group.
func (suite *CampaignsIntegrationTestSuite) TestNestedGroup() {
	suite.mockClient.On("ListCampaignTemplates", requests.ListCampaignTemplatesParams{}).Return(fixedCampaignTemplatesPage(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("table", "", "campaigns", "templates", "list")
	suite.Zero(code)
	suite.Contains(out, "Newsletter")
	suite.Contains(out, intCampaignTemplateID)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestSendPromptsOnStderr sends a campaign after the confirmation prompt.
func (suite *CampaignsIntegrationTestSuite) TestSendPromptsOnStderr() {
	campaign := fixedCampaign()
	campaign.Status = common.CampaignStatusValidating
	suite.mockClient.On("SendCampaign", uuid.MustParse(intCampaignID), requests.SendCampaignRequest{}, testutil.IdempotencyKeyArg("send-1")).
		Return(&responses.CampaignSend{Campaign: *campaign, Warnings: []string{"Check the design"}}, nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, stderr, code := execRoot("json", "y\n", "campaigns", "send", intCampaignID, "--idempotency-key", "send-1")
	suite.Zero(code)
	suite.Contains(stderr, "Are you sure you want to send campaign "+intCampaignID)
	var send responses.CampaignSend
	suite.Require().NoError(json.Unmarshal([]byte(out), &send))
	suite.Equal([]string{"Check the design"}, send.Warnings)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestCampaignNotReady checks that a refused send lists each issue with no
// usage text outside JSON mode, prints the raw API body in JSON mode, and
// exits 4 in every mode.
func (suite *CampaignsIntegrationTestSuite) TestCampaignNotReady() {
	for _, format := range []string{"table", "plain", "csv", "json"} {
		suite.Run(format, func() {
			mockClient := &mocks.MockClient{}
			mockClient.On("SendCampaign", uuid.MustParse(intCampaignID), requests.SendCampaignRequest{}).Return(nil, &api.APIError{
				StatusCode: 400,
				Message:    "campaign is not ready to send",
				Raw:        []byte(campaignNotReadyBody),
			})
			testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot(format, "", "campaigns", "send", intCampaignID, "--force")
			if format == "json" {
				suite.JSONEq(campaignNotReadyBody, out)
				suite.Empty(stderr)
			} else {
				suite.Empty(out)
				suite.Equal("Error: failed to send campaign: campaign is not ready to send\n"+
					"sender_missing: Add a sender\nlists_missing: Choose at least one list\n", stderr)
			}
			suite.NotContains(out, "Usage:")
			suite.NotContains(stderr, "Usage:")
			suite.Equal(4, code)
			mockClient.AssertExpectations(suite.T())
		})
	}
}

// TestValidationFailsBeforeAuth checks that local validation errors never
// reach the auth resolver.
func (suite *CampaignsIntegrationTestSuite) TestValidationFailsBeforeAuth() {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"get bad campaign ID", []string{"campaigns", "get", "nope"}, `invalid campaign ID value "nope": not a UUID`},
		{"update without fields", []string{"campaigns", "update", intCampaignID}, "at least one of --name"},
		{"test without to", []string{"campaigns", "test", intCampaignID}, "--to is required"},
		{"recipients without filter", []string{"campaigns", "recipients", intCampaignID}, "one of --status or --bounce-classification"},
		{"templates update without fields", []string{"campaigns", "templates", "update", intCampaignTemplateID}, "at least one of --name"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			called := testutil.InjectClient(suite.T(), &mocks.MockClient{})
			out, stderr, code := execRoot("plain", "", tc.args...)
			suite.Empty(out)
			suite.Contains(stderr, tc.message)
			suite.NotContains(stderr, "Usage:")
			suite.NotZero(code)
			suite.False(*called)
		})
	}
}

// TestJSONRoundTrip checks that JSON output decodes into the SDK types
// unchanged.
func (suite *CampaignsIntegrationTestSuite) TestJSONRoundTrip() {
	suite.mockClient.On("GetCampaign", uuid.MustParse(intCampaignID)).Return(fixedCampaign(), nil)
	suite.mockClient.On("ListCampaignTemplates", requests.ListCampaignTemplatesParams{}).Return(fixedCampaignTemplatesPage(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "campaigns", "get", intCampaignID)
	suite.Zero(code)
	var campaign responses.Campaign
	suite.Require().NoError(json.Unmarshal([]byte(out), &campaign))
	suite.Equal(*fixedCampaign(), campaign)
	suite.Contains(out, "<p>News {{ unsubscribe_link }}</p>")

	out, _, code = execRoot("json", "", "campaigns", "templates", "list")
	suite.Zero(code)
	var templates responses.PaginatedCampaignTemplatesResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &templates))
	suite.Equal(*fixedCampaignTemplatesPage(), templates)

	suite.mockClient.AssertExpectations(suite.T())
}

// TestAPIErrorPassThrough checks that JSON mode prints the API's error body
// unchanged and that every mode exits nonzero.
func (suite *CampaignsIntegrationTestSuite) TestAPIErrorPassThrough() {
	raw := `{"message":"campaign is not a draft"}`
	for _, format := range []string{"json", "table", "plain"} {
		suite.Run(format, func() {
			mockClient := &mocks.MockClient{}
			mockClient.On("UnscheduleCampaign", uuid.MustParse(intCampaignID), testutil.IdempotencyKeyArg("k")).Return(nil, &api.APIError{
				StatusCode: 409,
				Message:    "campaign is not a draft",
				Raw:        []byte(raw),
			})
			testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot(format, "", "campaigns", "unschedule", intCampaignID, "--idempotency-key", "k")
			if format == "json" {
				suite.JSONEq(raw, out)
			} else {
				suite.Empty(out)
				suite.Contains(stderr, "Error:")
			}
			suite.NotZero(code)
			mockClient.AssertExpectations(suite.T())
		})
	}
}

func TestCampaignsIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(CampaignsIntegrationTestSuite))
}
