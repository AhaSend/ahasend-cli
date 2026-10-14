package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/apiresponse"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRootJSONPrinterPreservesContactSuccessEnvelope(t *testing.T) {
	root := NewRootCmdForTesting()
	root.SetContext(context.Background())
	var output bytes.Buffer
	root.SetOut(&output)
	require.NoError(t, root.ParseFlags([]string{"--output", "json"}))
	require.NoError(t, initializePrinter(root))

	next := "next"
	response := &responses.PaginatedContactsResponse{
		Object: "list",
		Data: []responses.Contact{{
			Object:       "contact",
			Email:        "normalized@example.com",
			Status:       "active",
			Unsubscribed: false,
			Attributes: map[string]any{
				"profile": map[string]any{"flags": []any{true, nil, 7}},
			},
		}},
		Pagination: responses.ContactPagination{HasMore: true, NextCursor: &next},
	}
	handler := printer.GetResponseHandlerFromCommand(root)
	require.NoError(t, handler.HandleContactList(response, printer.ListConfig{}))

	var actual map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &actual))
	assert.Equal(t, "list", actual["object"])
	assert.Equal(t, "next", actual["pagination"].(map[string]any)["next_cursor"])
	contact := actual["data"].([]any)[0].(map[string]any)
	assert.Equal(t, "normalized@example.com", contact["email"])
	assert.Equal(t, "active", contact["status"])
	assert.Equal(t, false, contact["unsubscribed"])
	assert.Equal(t, []any{true, nil, float64(7)}, contact["attributes"].(map[string]any)["profile"].(map[string]any)["flags"])
}

// API bodies that re-encoding a decoded SDK struct cannot reproduce: keys out
// of alphabetical order, keys no SDK struct has, keys the SDK struct always
// encodes left out, and number literals that do not survive a float64.
const (
	rawObjectBody  = `{"zeta":1.50,"object":"thing","future_field":{"nested":[1,"x"]},"big":12345678901234567890}`
	rawListBody    = `{"zeta":1.50,"object":"list","data":[],"future_field":{"nested":[1,"x"]},"big":12345678901234567890}`
	rawMessageBody = `{"zeta":1.50,"message":"Done","future_field":{"nested":[1,"x"]},"big":12345678901234567890}`
)

// runRootJSON runs args through a fresh root command with --output json and
// returns stdout and the exit code.
func runRootJSON(t *testing.T, mockClient *mocks.MockClient, args ...string) (string, int) {
	t.Helper()
	testutil.InjectClient(t, mockClient)

	var stdout, stderr bytes.Buffer
	root := NewRootCmdForTesting()
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	var code int
	leaked := captureOSStdout(t, func() { code = run(root, append(args, "--output", "json")) })
	assert.Empty(t, leaked, "output written to os.Stdout instead of the command's stdout")
	if stderr.Len() > 0 {
		t.Logf("stderr: %s", stderr.String())
	}
	return stdout.String(), code
}

// captureOSStdout runs fn with os.Stdout redirected to a pipe and returns what
// fn wrote there, which the command's own stdout does not capture.
func captureOSStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	require.NoError(t, err)
	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	captured := make(chan string)
	go func() {
		var buffer bytes.Buffer
		_, _ = io.Copy(&buffer, reader)
		captured <- buffer.String()
	}()

	fn()
	require.NoError(t, writer.Close())
	return <-captured
}

// TestJSONOutputIsTheRawAPIResponse runs every command that prints one API
// response and checks that stdout is that response's body, unchanged.
func TestJSONOutputIsTheRawAPIResponse(t *testing.T) {
	contactsFile := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"user@example.com"}]}`)

	tests := []struct {
		name  string
		args  []string
		calls []apiCall
		body  string
	}{
		// apikeys
		{"apikeys create", []string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:all"},
			[]apiCall{returns[responses.APIKey]("CreateAPIKey", rawObjectBody)}, rawObjectBody},
		{"apikeys get", []string{"apikeys", "get", rawTestID},
			[]apiCall{returns[responses.APIKey]("GetAPIKey", rawObjectBody)}, rawObjectBody},
		{"apikeys list", []string{"apikeys", "list"},
			[]apiCall{returns[responses.PaginatedAPIKeysResponse]("ListAPIKeys", rawListBody)}, rawListBody},
		{"apikeys update", []string{"apikeys", "update", rawTestID, "--label", "ci"},
			[]apiCall{returns[responses.APIKey]("UpdateAPIKey", rawObjectBody)}, rawObjectBody},
		{"apikeys delete", []string{"apikeys", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteAPIKey", rawMessageBody)}, rawMessageBody},

		// campaigns
		{"campaigns create", []string{"campaigns", "create", "--name", "Launch"},
			[]apiCall{returns[responses.Campaign]("CreateCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns get", []string{"campaigns", "get", rawTestID},
			[]apiCall{returns[responses.Campaign]("GetCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns list", []string{"campaigns", "list"},
			[]apiCall{returns[responses.PaginatedCampaignsResponse]("ListCampaigns", rawListBody)}, rawListBody},
		{"campaigns update", []string{"campaigns", "update", rawTestID, "--name", "Launch"},
			[]apiCall{returns[responses.Campaign]("UpdateCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns duplicate", []string{"campaigns", "duplicate", rawTestID},
			[]apiCall{returns[responses.Campaign]("DuplicateCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns send", []string{"campaigns", "send", rawTestID, "--force"},
			[]apiCall{returns[responses.CampaignSend]("SendCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns unschedule", []string{"campaigns", "unschedule", rawTestID},
			[]apiCall{returns[responses.Campaign]("UnscheduleCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns test", []string{"campaigns", "test", rawTestID, "--to", "user@example.com"},
			[]apiCall{returns[responses.CampaignTest]("TestCampaign", rawObjectBody)}, rawObjectBody},
		{"campaigns stats", []string{"campaigns", "stats", rawTestID},
			[]apiCall{returns[responses.CampaignStats]("GetCampaignStats", rawObjectBody)}, rawObjectBody},
		{"campaigns recipients", []string{"campaigns", "recipients", rawTestID, "--status", "Delivered"},
			[]apiCall{returns[responses.PaginatedCampaignRecipientsResponse]("ListCampaignRecipients", rawListBody)}, rawListBody},
		{"campaigns delete", []string{"campaigns", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteCampaign", rawMessageBody)}, rawMessageBody},

		// campaigns templates
		{"campaigns templates create", []string{"campaigns", "templates", "create", "--name", "Base", "--editor", "simple"},
			[]apiCall{returns[responses.CampaignTemplate]("CreateCampaignTemplate", rawObjectBody)}, rawObjectBody},
		{"campaigns templates get", []string{"campaigns", "templates", "get", rawTestID},
			[]apiCall{returns[responses.CampaignTemplate]("GetCampaignTemplate", rawObjectBody)}, rawObjectBody},
		{"campaigns templates list", []string{"campaigns", "templates", "list"},
			[]apiCall{returns[responses.PaginatedCampaignTemplatesResponse]("ListCampaignTemplates", rawListBody)}, rawListBody},
		{"campaigns templates update", []string{"campaigns", "templates", "update", rawTestID, "--name", "Base"},
			[]apiCall{returns[responses.CampaignTemplate]("UpdateCampaignTemplate", rawObjectBody)}, rawObjectBody},
		{"campaigns templates delete", []string{"campaigns", "templates", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteCampaignTemplate", rawMessageBody)}, rawMessageBody},

		// contacts
		{"contacts create", []string{"contacts", "create", "user@example.com"},
			[]apiCall{returns[responses.Contact]("CreateContact", rawObjectBody)}, rawObjectBody},
		{"contacts get", []string{"contacts", "get", "user@example.com"},
			[]apiCall{returns[responses.Contact]("GetContact", rawObjectBody)}, rawObjectBody},
		{"contacts list", []string{"contacts", "list"},
			[]apiCall{returns[responses.PaginatedContactsResponse]("GetContacts", rawListBody)}, rawListBody},
		{"contacts update", []string{"contacts", "update", "user@example.com", "--first-name", "Ada"},
			[]apiCall{returns[responses.Contact]("UpdateContact", rawObjectBody)}, rawObjectBody},
		{"contacts delete", []string{"contacts", "delete", "user@example.com", "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteContact", rawMessageBody)}, rawMessageBody},
		{"contacts batch", []string{"contacts", "batch", "--file", contactsFile},
			[]apiCall{returns[responses.BatchUpsertContactsResponse]("BatchUpsertContacts", rawObjectBody)}, rawObjectBody},
		{"contacts memberships", []string{"contacts", "memberships", "user@example.com"},
			[]apiCall{returns[responses.PaginatedListContactsResponse]("GetContactLists", rawListBody)}, rawListBody},

		// domains
		{"domains create", []string{"domains", "create", "example.com"},
			[]apiCall{returns[responses.Domain]("CreateDomain", rawObjectBody)}, rawObjectBody},
		{"domains get", []string{"domains", "get", "example.com"},
			[]apiCall{returns[responses.Domain]("GetDomain", rawObjectBody)}, rawObjectBody},
		{"domains update", []string{"domains", "update", "example.com", "--sending-type", "marketing"},
			[]apiCall{returns[responses.Domain]("UpdateDomain", rawObjectBody)}, rawObjectBody},
		{"domains verify", []string{"domains", "verify", "example.com"},
			[]apiCall{returns[responses.Domain]("GetDomain", rawObjectBody)}, rawObjectBody},
		{"domains check-dns", []string{"domains", "check-dns", "example.com"},
			[]apiCall{returns[responses.Domain]("CheckDomainDNS", rawObjectBody)}, rawObjectBody},
		{"domains list", []string{"domains", "list"},
			[]apiCall{returns[responses.PaginatedDomainsResponse]("ListDomains", rawListBody)}, rawListBody},
		{"domains list --status", []string{"domains", "list", "--status", "verified"},
			[]apiCall{returns[responses.PaginatedDomainsResponse]("ListDomains", rawListBody)}, rawListBody},
		{"domains delete", []string{"domains", "delete", "example.com", "--force"},
			[]apiCall{
				returns[responses.Domain]("GetDomain", rawObjectBody),
				returns[common.SuccessResponse]("DeleteDomain", rawMessageBody),
			}, rawMessageBody},

		// lists
		{"lists create", []string{"lists", "create", "--name", "Newsletter"},
			[]apiCall{returns[responses.ContactList]("CreateList", rawObjectBody)}, rawObjectBody},
		{"lists get", []string{"lists", "get", rawTestID},
			[]apiCall{returns[responses.ContactList]("GetList", rawObjectBody)}, rawObjectBody},
		{"lists list", []string{"lists", "list"},
			[]apiCall{returns[responses.PaginatedContactListsResponse]("GetLists", rawListBody)}, rawListBody},
		{"lists update", []string{"lists", "update", rawTestID, "--name", "Newsletter"},
			[]apiCall{returns[responses.ContactList]("UpdateList", rawObjectBody)}, rawObjectBody},
		{"lists delete", []string{"lists", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteList", rawMessageBody)}, rawMessageBody},

		// lists members
		{"lists members add", []string{"lists", "members", "add", rawTestID, "--email", "user@example.com"},
			[]apiCall{returns[responses.BatchAddListContactsResponse]("BatchAddListContacts", rawObjectBody)}, rawObjectBody},
		{"lists members list", []string{"lists", "members", "list", rawTestID},
			[]apiCall{returns[responses.PaginatedListContactsResponse]("GetListContacts", rawListBody)}, rawListBody},
		{"lists members set", []string{"lists", "members", "set", rawTestID, "user@example.com", "--subscription-status", "confirmed"},
			[]apiCall{returns[responses.ListContact]("UpsertListContact", rawObjectBody)}, rawObjectBody},
		{"lists members remove", []string{"lists", "members", "remove", rawTestID, "user@example.com", "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteListContact", rawMessageBody)}, rawMessageBody},

		// messages
		{"messages get", []string{"messages", "get", rawTestID},
			[]apiCall{returns[responses.Message]("GetMessage", rawObjectBody)}, rawObjectBody},
		{"messages list", []string{"messages", "list", "--sender", "sender@example.com"},
			[]apiCall{returns[responses.PaginatedMessagesResponse]("GetMessages", rawListBody)}, rawListBody},
		{"messages cancel", []string{"messages", "cancel", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("CancelMessage", rawMessageBody)}, rawMessageBody},
		{"messages send", []string{"messages", "send", "--from", "sender@example.com", "--to", "user@example.com",
			"--subject", "Hello", "--text", "Hi"},
			[]apiCall{returns[responses.CreateMessageResponse]("SendMessageWithIdempotencyKey", rawListBody)}, rawListBody},

		// ping
		{"ping", []string{"ping"},
			[]apiCall{returns[common.SuccessResponse]("Ping", rawMessageBody)}, rawMessageBody},

		// routes
		{"routes create", []string{"routes", "create", "--name", "Inbound", "--url", "https://example.com/hook", "--recipient", "*@example.com"},
			[]apiCall{returns[responses.Route]("CreateRoute", rawObjectBody)}, rawObjectBody},
		{"routes get", []string{"routes", "get", rawTestID},
			[]apiCall{returns[responses.Route]("GetRoute", rawObjectBody)}, rawObjectBody},
		{"routes list", []string{"routes", "list"},
			[]apiCall{returns[responses.PaginatedRoutesResponse]("ListRoutes", rawListBody)}, rawListBody},
		{"routes update", []string{"routes", "update", rawTestID, "--name", "Inbound"},
			[]apiCall{returns[responses.Route]("UpdateRoute", rawObjectBody)}, rawObjectBody},
		{"routes delete", []string{"routes", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteRoute", rawMessageBody)}, rawMessageBody},
		{"routes trigger", []string{"routes", "trigger", rawTestID},
			[]apiCall{returns[common.SuccessResponse]("TriggerRoute", rawMessageBody)}, rawMessageBody},

		// smtp
		{"smtp create", []string{"smtp", "create", "--name", "App", "--scope", "global"},
			[]apiCall{returns[responses.SMTPCredential]("CreateSMTPCredential", rawObjectBody)}, rawObjectBody},
		{"smtp get", []string{"smtp", "get", rawTestID},
			[]apiCall{returns[responses.SMTPCredential]("GetSMTPCredential", rawObjectBody)}, rawObjectBody},
		{"smtp list", []string{"smtp", "list"},
			[]apiCall{returns[responses.PaginatedSMTPCredentialsResponse]("ListSMTPCredentials", rawListBody)}, rawListBody},
		{"smtp delete", []string{"smtp", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteSMTPCredential", rawMessageBody)}, rawMessageBody},

		// stats
		{"stats bounces", []string{"stats", "bounces"},
			[]apiCall{returns[responses.BounceStatisticsResponse]("GetBounceStatistics", rawObjectBody)}, rawObjectBody},
		{"stats deliverability", []string{"stats", "deliverability"},
			[]apiCall{returns[responses.DeliverabilityStatisticsResponse]("GetDeliverabilityStatistics", rawObjectBody)}, rawObjectBody},
		{"stats delivery-time", []string{"stats", "delivery-time"},
			[]apiCall{returns[responses.DeliveryTimeStatisticsResponse]("GetDeliveryTimeStatistics", rawObjectBody)}, rawObjectBody},

		// subaccounts
		{"subaccounts create", []string{"subaccounts", "create", "--name", "Acme", "--website", "acme.example.com"},
			[]apiCall{returns[responses.SubAccount]("CreateSubAccount", rawObjectBody)}, rawObjectBody},
		{"subaccounts get", []string{"subaccounts", "get", rawTestID},
			[]apiCall{returns[responses.SubAccount]("GetSubAccount", rawObjectBody)}, rawObjectBody},
		{"subaccounts list", []string{"subaccounts", "list"},
			[]apiCall{returns[responses.PaginatedSubAccountsResponse]("ListSubAccounts", rawListBody)}, rawListBody},
		{"subaccounts update", []string{"subaccounts", "update", rawTestID, "--name", "Acme"},
			[]apiCall{returns[responses.SubAccount]("UpdateSubAccount", rawObjectBody)}, rawObjectBody},
		{"subaccounts suspend", []string{"subaccounts", "suspend", rawTestID, "--reason", "billing", "--force"},
			[]apiCall{returns[responses.SubAccount]("SuspendSubAccount", rawObjectBody)}, rawObjectBody},
		{"subaccounts unsuspend", []string{"subaccounts", "unsuspend", rawTestID},
			[]apiCall{returns[responses.SubAccount]("UnsuspendSubAccount", rawObjectBody)}, rawObjectBody},
		{"subaccounts usage", []string{"subaccounts", "usage"},
			[]apiCall{returns[responses.SubAccountUsageResponse]("GetSubAccountsUsage", rawObjectBody)}, rawObjectBody},
		{"subaccounts unpause-domain", []string{"subaccounts", "unpause-domain", rawTestID, "example.com"},
			[]apiCall{returns[responses.Domain]("UnpauseSubAccountDomain", rawObjectBody)}, rawObjectBody},
		{"subaccounts delete", []string{"subaccounts", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteSubAccount", rawMessageBody)}, rawMessageBody},

		// subaccounts api-keys
		{"subaccounts api-keys create", []string{"subaccounts", "api-keys", "create", rawTestID, "--label", "ci",
			"--scope", "messages:send:all"},
			[]apiCall{returns[responses.APIKey]("CreateSubAccountAPIKey", rawObjectBody)}, rawObjectBody},
		{"subaccounts api-keys get", []string{"subaccounts", "api-keys", "get", rawTestID, rawTestOtherID},
			[]apiCall{returns[responses.APIKey]("GetSubAccountAPIKey", rawObjectBody)}, rawObjectBody},
		{"subaccounts api-keys list", []string{"subaccounts", "api-keys", "list", rawTestID},
			[]apiCall{returns[responses.PaginatedAPIKeysResponse]("ListSubAccountAPIKeys", rawListBody)}, rawListBody},
		{"subaccounts api-keys update", []string{"subaccounts", "api-keys", "update", rawTestID, rawTestOtherID, "--label", "ci"},
			[]apiCall{returns[responses.APIKey]("UpdateSubAccountAPIKey", rawObjectBody)}, rawObjectBody},
		{"subaccounts api-keys delete", []string{"subaccounts", "api-keys", "delete", rawTestID, rawTestOtherID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteSubAccountAPIKey", rawMessageBody)}, rawMessageBody},

		// suppressions
		{"suppressions create", []string{"suppressions", "create", "user@example.com", "--expires", "30d"},
			[]apiCall{returns[responses.CreateSuppressionResponse]("CreateSuppression", rawListBody)}, rawListBody},
		{"suppressions list", []string{"suppressions", "list"},
			[]apiCall{returns[responses.PaginatedSuppressionsResponse]("ListSuppressions", rawListBody)}, rawListBody},
		{"suppressions check", []string{"suppressions", "check", "user@example.com"},
			[]apiCall{returns[responses.PaginatedSuppressionsResponse]("ListSuppressions", rawListBody)}, rawListBody},
		{"suppressions delete", []string{"suppressions", "delete", "user@example.com", "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteSuppression", rawMessageBody)}, rawMessageBody},
		{"suppressions wipe", []string{"suppressions", "wipe", "--force"},
			[]apiCall{returns[common.SuccessResponse]("WipeSuppressions", rawMessageBody)}, rawMessageBody},

		// templates
		{"templates create", []string{"templates", "create", "--name", "Welcome", "--editor", "simple"},
			[]apiCall{returns[responses.Template]("CreateTemplate", rawObjectBody)}, rawObjectBody},
		{"templates get", []string{"templates", "get", rawTestID},
			[]apiCall{returns[responses.Template]("GetTemplate", rawObjectBody)}, rawObjectBody},
		{"templates list", []string{"templates", "list"},
			[]apiCall{returns[responses.PaginatedTemplatesResponse]("GetTemplates", rawListBody)}, rawListBody},
		{"templates update", []string{"templates", "update", rawTestID, "--name", "Welcome"},
			[]apiCall{returns[responses.Template]("UpdateTemplate", rawObjectBody)}, rawObjectBody},
		{"templates publish", []string{"templates", "publish", rawTestID},
			[]apiCall{returns[responses.Template]("PublishTemplate", rawObjectBody)}, rawObjectBody},
		{"templates delete", []string{"templates", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteTemplate", rawMessageBody)}, rawMessageBody},
		{"templates draft get", []string{"templates", "draft", "get", rawTestID},
			[]apiCall{returns[responses.TemplateDraft]("GetTemplateDraft", rawObjectBody)}, rawObjectBody},
		{"templates draft discard", []string{"templates", "draft", "discard", rawTestID, "--force"},
			[]apiCall{returns[responses.Template]("DiscardTemplateDraft", rawObjectBody)}, rawObjectBody},
		{"templates versions list", []string{"templates", "versions", "list", rawTestID},
			[]apiCall{returns[responses.TemplateVersionsResponse]("GetTemplateVersions", rawListBody)}, rawListBody},
		{"templates versions get", []string{"templates", "versions", "get", rawTestID, rawTestOtherID},
			[]apiCall{returns[responses.TemplateVersionDetail]("GetTemplateVersion", rawObjectBody)}, rawObjectBody},
		{"templates versions restore", []string{"templates", "versions", "restore", rawTestID, rawTestOtherID, "--force"},
			[]apiCall{returns[responses.Template]("RestoreTemplateVersion", rawObjectBody)}, rawObjectBody},

		// webhooks
		{"webhooks create", []string{"webhooks", "create", "--name", "Events", "--url", "https://example.com/hook",
			"--scope", "global", "--events", "delivered"},
			[]apiCall{returns[responses.Webhook]("CreateWebhook", rawObjectBody)}, rawObjectBody},
		{"webhooks get", []string{"webhooks", "get", rawTestID},
			[]apiCall{returns[responses.Webhook]("GetWebhook", rawObjectBody)}, rawObjectBody},
		{"webhooks list", []string{"webhooks", "list"},
			[]apiCall{returns[responses.PaginatedWebhooksResponse]("ListWebhooks", rawListBody)}, rawListBody},
		{"webhooks update", []string{"webhooks", "update", rawTestID, "--name", "Events"},
			[]apiCall{returns[responses.Webhook]("UpdateWebhook", rawObjectBody)}, rawObjectBody},
		{"webhooks delete", []string{"webhooks", "delete", rawTestID, "--force"},
			[]apiCall{returns[common.SuccessResponse]("DeleteWebhook", rawMessageBody)}, rawMessageBody},
		{"webhooks trigger", []string{"webhooks", "trigger", rawTestID, "--events", "message.delivered"},
			[]apiCall{returns[common.SuccessResponse]("TriggerWebhook", rawMessageBody)}, rawMessageBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetAccountID").Return(rawTestID).Maybe()
			for _, call := range tt.calls {
				expectAnyArgs(mockClient, call.method, call.result(t), nil)
			}

			stdout, code := runRootJSON(t, mockClient, tt.args...)

			assert.Equal(t, 0, code)
			assert.Equal(t, testutil.IndentedJSON(t, tt.body), stdout)
		})
	}
}

func TestJSONOutputOfSeveralMessageCancelsIsAnArrayOfAPIResponses(t *testing.T) {
	const thirdID = "33333333-3333-4333-8333-333333333333"
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(rawTestID)
	mockClient.On("CancelMessage", rawTestID).
		Return(mocks.APIResponse[common.SuccessResponse](t, rawMessageBody), nil)
	mockClient.On("CancelMessage", rawTestOtherID).Return(nil, rawAPIError(404))
	mockClient.On("CancelMessage", thirdID).
		Return(mocks.APIResponse[common.SuccessResponse](t, rawObjectBody), nil)

	stdout, code := runRootJSON(t, mockClient, "messages", "cancel", rawTestID, rawTestOtherID, thirdID, "--force")

	assert.Equal(t, 1, code)
	assert.Equal(t, testutil.IndentedJSON(t, "["+rawMessageBody+","+rawErrorBody+","+rawObjectBody+"]"), stdout)
}

func TestJSONOutputOfMultiRequestSendIsAnArrayOfAPIResponses(t *testing.T) {
	// The batch writes the failed recipients file under the working directory.
	t.Chdir(t.TempDir())
	var rows strings.Builder
	rows.WriteString("email\n")
	for i := 0; i < 150; i++ {
		fmt.Fprintf(&rows, "user%d@example.com\n", i)
	}
	recipientsPath := testutil.WriteTempFile(t, "recipients.csv", rows.String())
	recipientCount := func(count int) any {
		return mock.MatchedBy(func(request requests.CreateMessageRequest) bool { return len(request.Recipients) == count })
	}

	mockClient := &mocks.MockClient{}
	mockClient.On("SendMessageWithIdempotencyKey", recipientCount(100), mock.Anything).
		Return(mocks.APIResponse[responses.CreateMessageResponse](t, rawListBody), nil)
	mockClient.On("SendMessageWithIdempotencyKey", recipientCount(50), mock.Anything).
		Return(nil, rawAPIError(422))

	stdout, code := runRootJSON(t, mockClient, "messages", "send", "--from", "sender@example.com",
		"--recipients", recipientsPath, "--subject", "Hello", "--text", "Hi", "--max-retries", "0")

	assert.Equal(t, 1, code)
	assert.Equal(t, testutil.IndentedJSON(t, "["+rawListBody+","+rawErrorBody+"]"), stdout)
}

func TestJSONOutputOfAFailedSingleCallIsTheRawAPIError(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		noResult any // the method's typed nil result
		args     []string
	}{
		{"messages send", "SendMessageWithIdempotencyKey", (*responses.CreateMessageResponse)(nil),
			[]string{"messages", "send", "--from", "sender@example.com", "--to", "user@example.com",
				"--subject", "Hello", "--text", "Hi", "--max-retries", "0"}},
		{"stats bounces", "GetBounceStatistics", (*responses.BounceStatisticsResponse)(nil),
			[]string{"stats", "bounces"}},
		{"webhooks trigger", "TriggerWebhook", (*common.SuccessResponse)(nil),
			[]string{"webhooks", "trigger", rawTestID, "--events", "message.delivered"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			expectAnyArgs(mockClient, tt.method, tt.noResult, rawAPIError(422))

			stdout, code := runRootJSON(t, mockClient, tt.args...)

			assert.Equal(t, 4, code)
			assert.Equal(t, testutil.IndentedJSON(t, rawErrorBody), stdout)
		})
	}
}

func TestJSONOutputOfAnEmptyAPIBodyIsNothing(t *testing.T) {
	empty := &common.SuccessResponse{}
	apiresponse.Remember(empty, []byte{})
	mockClient := &mocks.MockClient{}
	mockClient.On("Ping").Return(empty, nil)

	stdout, code := runRootJSON(t, mockClient, "ping")

	assert.Equal(t, 0, code)
	assert.Empty(t, stdout)
}

func TestRootPrinterRendersContactInEveryOutputFormat(t *testing.T) {
	contact := &responses.Contact{
		Email:        "normalized@example.com",
		Status:       "active",
		Unsubscribed: false,
		Attributes:   map[string]any{"profile": map[string]any{"score": 7}},
	}

	for _, format := range printer.GetSupportedFormats() {
		t.Run(format, func(t *testing.T) {
			root := NewRootCmdForTesting()
			root.SetContext(context.Background())
			var output bytes.Buffer
			root.SetOut(&output)
			require.NoError(t, root.ParseFlags([]string{"--output", format}))
			require.NoError(t, initializePrinter(root))

			handler := printer.GetResponseHandlerFromCommand(root)
			require.Equal(t, format, handler.GetFormat())
			require.NoError(t, handler.HandleSingleContact(contact, printer.SingleConfig{
				SuccessMessage: "Contact retrieved",
			}))
			assert.Contains(t, output.String(), "normalized@example.com")
			assert.Contains(t, output.String(), "active")
			assert.Contains(t, output.String(), "score")
		})
	}
}
