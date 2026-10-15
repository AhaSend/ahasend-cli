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
	intTemplateID        = "88888888-8888-4888-8888-888888888888"
	intTemplateVersionID = "99999999-9999-4999-8999-999999999999"
)

// TemplatesIntegrationTestSuite drives the templates commands, their draft and
// versions groups, and template sends end to end through
// cmd.NewRootCmdForTesting against a MockClient wired in via the auth resolver.
type TemplatesIntegrationTestSuite struct {
	commandSuite
}

func fixedTemplate() *responses.Template {
	fromName := "Example"
	return &responses.Template{
		Object:    "template",
		ID:        uuid.MustParse(intTemplateID),
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Name:      "Welcome",
		Subject:   "Hello {{ first_name }}",
		Variables: []responses.TemplateVariable{{Name: "first_name", Required: true}},
		From:      &common.SenderAddress{Email: "hello@example.com", Name: &fromName},
		Editor:    "html",
		Content:   &responses.TemplateContent{HTML: "<p>Hello {{ first_name }}</p>"},
	}
}

func fixedTemplatesPage() *responses.PaginatedTemplatesResponse {
	next := "next-templates"
	template := fixedTemplate()
	template.Content = nil
	return &responses.PaginatedTemplatesResponse{
		Object:     "list",
		Data:       []responses.Template{*template},
		Pagination: common.PaginationInfo{HasMore: true, NextCursor: &next},
	}
}

func fixedTemplateVersions() *responses.TemplateVersionsResponse {
	return &responses.TemplateVersionsResponse{
		Object: "list",
		Data: []responses.TemplateVersion{{
			Object:      "template_version",
			ID:          uuid.MustParse(intTemplateVersionID),
			Version:     1,
			PublishedAt: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		}},
	}
}

// TestTemplateWorkflow creates, gets, lists and deletes a template.
func (suite *TemplatesIntegrationTestSuite) TestTemplateWorkflow() {
	htmlPath := filepath.Join(suite.T().TempDir(), "welcome.html")
	suite.Require().NoError(os.WriteFile(htmlPath, []byte("<p>Hello {{ first_name }}</p>"), 0o600))
	html := "<p>Hello {{ first_name }}</p>"
	subject := "Hello {{ first_name }}"

	suite.mockClient.On("CreateTemplate", requests.CreateTemplateRequest{
		Name:    "Welcome",
		Subject: &subject,
		Content: &requests.TemplateContentInput{HTML: &html},
		Publish: true,
	}).Return(fixedTemplate(), nil)
	suite.mockClient.On("GetTemplate", uuid.MustParse(intTemplateID)).Return(fixedTemplate(), nil)
	suite.mockClient.On("GetTemplates", requests.GetTemplatesParams{}).Return(fixedTemplatesPage(), nil)
	suite.mockClient.On("DeleteTemplate", uuid.MustParse(intTemplateID)).Return(&common.SuccessResponse{}, nil)
	called := testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "templates", "create", "--name", "Welcome",
		"--subject", subject, "--html-file", htmlPath, "--publish")
	suite.Zero(code)
	suite.Contains(out, "Template created successfully")
	suite.Contains(out, intTemplateID)

	out, _, code = execRoot("plain", "", "templates", "get", intTemplateID)
	suite.Zero(code)
	suite.Contains(out, "Variables: first_name (required)\n")
	suite.Contains(out, "HTML:\n  <p>Hello {{ first_name }}</p>\n")

	out, _, code = execRoot("table", "", "templates", "list")
	suite.Zero(code)
	suite.Contains(out, "Welcome")
	suite.True(strings.HasSuffix(out, "--after next-templates\n"), out)

	out, stderr, code := execRoot("plain", "", "templates", "delete", intTemplateID, "--force")
	suite.Zero(code)
	suite.Equal("Template '"+intTemplateID+"' deleted successfully\n", out)
	suite.Empty(stderr)

	suite.True(*called)
	suite.mockClient.AssertExpectations(suite.T())
}

// TestNestedGroups runs one command of the draft and versions groups.
func (suite *TemplatesIntegrationTestSuite) TestNestedGroups() {
	suite.mockClient.On("DiscardTemplateDraft", uuid.MustParse(intTemplateID)).Return(fixedTemplate(), nil)
	suite.mockClient.On("GetTemplateVersions", uuid.MustParse(intTemplateID)).Return(fixedTemplateVersions(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, stderr, code := execRoot("plain", "y\n", "templates", "draft", "discard", intTemplateID)
	suite.Zero(code)
	suite.Contains(out, "Template draft discarded successfully")
	suite.Contains(out, "Name: Welcome")
	suite.Contains(stderr, "Are you sure")

	out, _, code = execRoot("table", "", "templates", "versions", "list", intTemplateID)
	suite.Zero(code)
	suite.Contains(out, intTemplateVersionID)

	suite.mockClient.AssertExpectations(suite.T())
}

// TestTemplateSend sends a template through messages send.
func (suite *TemplatesIntegrationTestSuite) TestTemplateSend() {
	suite.mockClient.On("SendTemplateMessageWithIdempotencyKey", requests.CreateTemplateMessageRequest{
		TemplateID: uuid.MustParse(intTemplateID),
		Recipients: []common.Recipient{{Email: "user@example.com"}},
	}, "send-key").Return(mocks.NewMessageResponse("msg-1"), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("plain", "", "messages", "send", "--template-id", intTemplateID,
		"--to", "user@example.com", "--idempotency-key", "send-key")
	suite.Zero(code)
	suite.Contains(out, "Message sent successfully")
	suite.mockClient.AssertExpectations(suite.T())
}

// TestValidationFailsBeforeAuth checks that local validation errors never
// reach the auth resolver.
func (suite *TemplatesIntegrationTestSuite) TestValidationFailsBeforeAuth() {
	cases := []struct {
		name    string
		args    []string
		message string
	}{
		{"get bad template ID", []string{"templates", "get", "nope"}, `invalid template ID value "nope": not a UUID`},
		{"update without fields", []string{"templates", "update", intTemplateID}, "at least one of --name"},
		{"versions get bad version ID", []string{"templates", "versions", "get", intTemplateID, "nope"}, `invalid version ID value "nope": not a UUID`},
		{"send with content", []string{"messages", "send", "--template-id", intTemplateID, "--text", "x"}, "--template-id and --text cannot be used together"},
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
func (suite *TemplatesIntegrationTestSuite) TestJSONRoundTrip() {
	suite.mockClient.On("GetTemplate", uuid.MustParse(intTemplateID)).Return(fixedTemplate(), nil)
	suite.mockClient.On("GetTemplateVersions", uuid.MustParse(intTemplateID)).Return(fixedTemplateVersions(), nil)
	testutil.InjectClient(suite.T(), suite.mockClient)

	out, _, code := execRoot("json", "", "templates", "get", intTemplateID)
	suite.Zero(code)
	var template responses.Template
	suite.Require().NoError(json.Unmarshal([]byte(out), &template))
	suite.Equal(*fixedTemplate(), template)
	suite.Contains(out, "<p>Hello {{ first_name }}</p>")

	out, _, code = execRoot("json", "", "templates", "versions", "list", intTemplateID)
	suite.Zero(code)
	var versions responses.TemplateVersionsResponse
	suite.Require().NoError(json.Unmarshal([]byte(out), &versions))
	suite.Equal(*fixedTemplateVersions(), versions)

	suite.mockClient.AssertExpectations(suite.T())
}

// TestAPIErrorPassThrough checks that JSON mode prints the API's error body
// unchanged and that every mode exits nonzero.
func (suite *TemplatesIntegrationTestSuite) TestAPIErrorPassThrough() {
	raw := `{"message":"template has no draft"}`
	for _, format := range []string{"json", "table", "plain"} {
		suite.Run(format, func() {
			mockClient := &mocks.MockClient{}
			mockClient.On("PublishTemplate", uuid.MustParse(intTemplateID), testutil.IdempotencyKeyArg("k")).Return(nil, &api.APIError{
				StatusCode: 422,
				Message:    "template has no draft",
				Raw:        []byte(raw),
			})
			testutil.InjectClient(suite.T(), mockClient)

			out, stderr, code := execRoot(format, "", "templates", "publish", intTemplateID, "--idempotency-key", "k")
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

func TestTemplatesIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(TemplatesIntegrationTestSuite))
}
