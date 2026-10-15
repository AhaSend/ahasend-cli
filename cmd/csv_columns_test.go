package cmd

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

const (
	csvAPIKeyBody     = `{"object":"api_key","id":"` + rawTestID + `","label":"CI","public_key":"pk","secret_key":"sk","account_id":"` + rawTestOtherID + `","scopes":[]}`
	csvItemBody       = `{"id":"` + rawTestID + `"}`
	csvListBody       = `{"object":"list","data":[` + csvItemBody + `],"pagination":{"has_more":false}}`
	csvCampaignTest   = `{"object":"campaign_test","sent_to":["user@example.com"]}`
	csvSubAccountsUse = `{"object":"sub_account_usage","currency":"USD"}`
	// csvContactsFile stands in a view's arguments for a contacts file.
	csvContactsFile = "<contacts file>"
)

// csvView is one list or single view: a command that prints it, the API call
// behind it, and the CSV header the view writes.
type csvView struct {
	args   []string
	call   apiCall
	header string
}

// csvViews holds a command for every list and single view of the printers.
var csvViews = []csvView{
	// API keys
	{[]string{"apikeys", "list"}, returns[responses.PaginatedAPIKeysResponse]("ListAPIKeys", csvListBody),
		"label,id,public_key,scopes,ip_allow_list,last_used_at,created_at,updated_at"},
	{[]string{"apikeys", "get", rawTestID}, returns[responses.APIKey]("GetAPIKey", csvAPIKeyBody),
		"label,id,public_key,account_id,scopes,ip_allow_list,last_used_at,created_at,updated_at"},
	{[]string{"apikeys", "create", "--label", "CI", "--scope", "domains:read"}, returns[responses.APIKey]("CreateAPIKey", csvAPIKeyBody),
		"label,id,public_key,secret_key,account_id,scopes,ip_allow_list,created_at"},
	{[]string{"apikeys", "update", rawTestID, "--label", "CI"}, returns[responses.APIKey]("UpdateAPIKey", csvAPIKeyBody),
		"label,id,public_key,account_id,scopes,ip_allow_list,updated_at"},

	// Campaigns
	{[]string{"campaigns", "list"}, returns[responses.PaginatedCampaignsResponse]("ListCampaigns", csvListBody),
		"name,id,status,status_reason,subject,preheader,from,reply_to,send_at,send_resumes_at,finished_at,created_at,updated_at"},
	{[]string{"campaigns", "get", rawTestID}, returns[responses.Campaign]("GetCampaign", csvItemBody),
		"name,id,status,status_reason,subject,preheader,from,reply_to,send_at,send_resumes_at,finished_at,created_at,updated_at,list_ids,editor,ready,issues,mjml,html,text,text_is_custom"},
	{[]string{"campaigns", "send", rawTestID, "--force"}, returns[responses.CampaignSend]("SendCampaign", csvItemBody),
		"name,id,status,status_reason,subject,preheader,from,reply_to,send_at,send_resumes_at,finished_at,created_at,updated_at,list_ids,editor,ready,issues,mjml,html,text,text_is_custom,warnings"},
	{[]string{"campaigns", "test", rawTestID, "--to", "user@example.com"}, returns[responses.CampaignTest]("TestCampaign", csvCampaignTest),
		"email,outcome,reason"},
	{[]string{"campaigns", "stats", rawTestID}, returns[responses.CampaignStats]("GetCampaignStats", csvItemBody),
		"campaign_id,recipients,delivered,opens,clicks,unsubscribes,complaints,open_rate,click_rate,bounce_rate,deferral_rate,recount_failed,report_computed_at,report_recipients,report_openers,report_clickers,report_bounces,report_deferrals,report_by_status,report_by_bounce_classification"},
	{[]string{"campaigns", "recipients", rawTestID, "--status", "Delivered"}, returns[responses.PaginatedCampaignRecipientsResponse]("ListCampaignRecipients", csvListBody),
		"email,contact_id,list_name,status,bounce_classification,bounce_reason,bounce_message,attempts,created_at,updated_at"},
	{[]string{"campaigns", "templates", "list"}, returns[responses.PaginatedCampaignTemplatesResponse]("ListCampaignTemplates", csvListBody),
		"name,id,editor,created_at,updated_at,mjml,html,text,text_is_custom"},
	{[]string{"campaigns", "templates", "get", rawTestID}, returns[responses.CampaignTemplate]("GetCampaignTemplate", csvItemBody),
		"name,id,editor,created_at,updated_at,mjml,html,text,text_is_custom"},

	// Contacts
	{[]string{"contacts", "list"}, returns[responses.PaginatedContactsResponse]("GetContacts", csvListBody),
		"email,id,first_name,last_name,status,status_reason,unsubscribed,unsubscribed_at,attributes,validation_status,last_validated_at,created_at,updated_at"},
	{[]string{"contacts", "get", "user@example.com"}, returns[responses.Contact]("GetContact", csvItemBody),
		"email,id,first_name,last_name,status,status_reason,unsubscribed,unsubscribed_at,attributes,validation_status,last_validated_at,created_at,updated_at"},
	{[]string{"contacts", "memberships", "user@example.com"},
		returns[responses.PaginatedListContactsResponse]("GetContactLists", csvListBody),
		"list_name,list_id,email,contact_id,subscription_status,subscribed_at,unsubscribed_at,created_at,updated_at"},
	{[]string{"contacts", "batch", "--file", csvContactsFile}, returns[responses.BatchUpsertContactsResponse]("BatchUpsertContacts", csvListBody),
		"position,email,contact_id,outcome,reason"},

	// Domains
	{[]string{"domains", "list"}, returns[responses.PaginatedDomainsResponse]("ListDomains", csvListBody),
		"domain,id,dns_valid,sending_type,paused_at,pause_reason,created_at,updated_at,last_dns_check_at"},
	{[]string{"domains", "get", "example.com"}, returns[responses.Domain]("GetDomain", csvItemBody),
		"domain,id,account_id,dns_valid,sending_type,paused_at,pause_reason,tracking_subdomain,return_path_subdomain,subscription_subdomain,media_subdomain,dkim_selector,dkim_rotation_interval_days,created_at,updated_at,last_dns_check_at,dns_records"},

	// Lists
	{[]string{"lists", "list"}, returns[responses.PaginatedContactListsResponse]("GetLists", csvListBody),
		"name,id,description,tags,contact_count,created_at,updated_at"},
	{[]string{"lists", "get", rawTestID}, returns[responses.ContactList]("GetList", csvItemBody),
		"name,id,description,tags,contact_count,created_at,updated_at"},
	{[]string{"lists", "members", "list", rawTestID}, returns[responses.PaginatedListContactsResponse]("GetListContacts", csvListBody),
		"email,contact_id,first_name,last_name,list_id,subscription_status,subscribed_at,unsubscribed_at,created_at,updated_at"},
	{[]string{"lists", "members", "set", rawTestID, "user@example.com", "--subscription-status", "confirmed"},
		returns[responses.ListContact]("UpsertListContact", csvItemBody),
		"email,contact_id,list_id,subscription_status,subscribed_at,unsubscribed_at,created_at,updated_at"},
	{[]string{"lists", "members", "add", rawTestID, "--email", "user@example.com"},
		returns[responses.BatchAddListContactsResponse]("BatchAddListContacts", csvListBody),
		"position,email,contact_id,outcome,reason"},

	// Messages
	{[]string{"messages", "list", "--sender", "sender@example.com"}, returns[responses.PaginatedMessagesResponse]("GetMessages", csvListBody),
		"subject,id,sender,recipient,status,created_at,delivered_at,opens,clicks,template_id"},
	{[]string{"messages", "list", "--sender", "sender@example.com", "--show-details"},
		returns[responses.PaginatedMessagesResponse]("GetMessages", csvListBody),
		"subject,id,sender,recipient,status,created_at,delivered_at,opens,clicks,template_id,message_id,direction,domain_id,attempts,tags,bounce_class,retain_until"},
	{[]string{"messages", "get", rawTestID}, returns[responses.Message]("GetMessage", csvItemBody),
		"subject,id,account_id,sender,recipient,status,direction,created_at,updated_at,sent_at,delivered_at,opens,clicks,attempts,bounce_class,is_bounce_notification,message_id,domain_id,template_id,tags,retain_until,delivery_attempts,content_size,content"},
	{[]string{"messages", "send", "--from", "sender@example.com", "--to", "user@example.com", "--subject", "Hi", "--text", "Hi"},
		returns[responses.CreateMessageResponse]("SendMessageWithIdempotencyKey", `{"object":"list","data":[{"object":"message"}]}`),
		"recipient,id,status,error"},

	// Routes
	{[]string{"routes", "list"}, returns[responses.PaginatedRoutesResponse]("ListRoutes", csvListBody),
		"name,id,url,enabled,recipient,attachments,headers,group_by_message_id,strip_replies,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},
	{[]string{"routes", "get", rawTestID}, returns[responses.Route]("GetRoute", csvItemBody),
		"name,id,url,enabled,recipient,attachments,headers,group_by_message_id,strip_replies,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},
	{[]string{"routes", "create", "--name", "Inbound", "--url", "https://example.com/hook", "--recipient", "*@example.com"},
		returns[responses.Route]("CreateRoute", csvItemBody),
		"name,id,url,enabled,recipient,secret,attachments,headers,group_by_message_id,strip_replies,created_at,updated_at"},
	{[]string{"routes", "update", rawTestID, "--name", "Inbound"}, returns[responses.Route]("UpdateRoute", csvItemBody),
		"name,id,url,enabled,recipient,attachments,headers,group_by_message_id,strip_replies,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},

	// SMTP credentials
	{[]string{"smtp", "list"}, returns[responses.PaginatedSMTPCredentialsResponse]("ListSMTPCredentials", csvListBody),
		"name,id,username,scope,domains,sandbox,created_at,updated_at"},
	{[]string{"smtp", "get", rawTestID}, returns[responses.SMTPCredential]("GetSMTPCredential", csvItemBody),
		"name,id,username,scope,domains,sandbox,created_at,updated_at"},
	{[]string{"smtp", "create", "--name", "App", "--scope", "global"}, returns[responses.SMTPCredential]("CreateSMTPCredential", csvItemBody),
		"name,id,username,password,scope,domains,sandbox,created_at"},

	// Sub-accounts
	{[]string{"subaccounts", "list"}, returns[responses.PaginatedSubAccountsResponse]("ListSubAccounts", csvListBody),
		"name,id,status,website,monthly_credit,domain_count,member_count,created_at"},
	{[]string{"subaccounts", "get", rawTestID}, returns[responses.SubAccount]("GetSubAccount", csvItemBody),
		"name,id,parent_account_id,status,website,monthly_credit,domain_count,member_count,created_at,last_activity_at"},
	{[]string{"subaccounts", "usage"}, returns[responses.SubAccountUsageResponse]("GetSubAccountsUsage", csvSubAccountsUse),
		"billing_period_start,billing_period_end,currency,allocation_method,allocation_note,account_name,account_id,reception_count,allocated_cost"},

	// Suppressions
	{[]string{"suppressions", "list"}, returns[responses.PaginatedSuppressionsResponse]("ListSuppressions", csvListBody),
		"email,id,domain,reason,created_at,expires_at,protected"},
	{[]string{"suppressions", "check", "user@example.com"}, returns[responses.PaginatedSuppressionsResponse]("ListSuppressions", csvListBody),
		"email,id,domain,reason,created_at,expires_at,protected"},
	{[]string{"suppressions", "create", "user@example.com", "--expires", "30d"},
		returns[responses.CreateSuppressionResponse]("CreateSuppression", csvListBody),
		"email,id,domain,reason,created_at,expires_at,protected"},

	// Templates
	{[]string{"templates", "list"}, returns[responses.PaginatedTemplatesResponse]("GetTemplates", csvListBody),
		"name,id,editor,subject,preheader,from,reply_to,has_draft,variables,created_at,updated_at,mjml,html,text,text_is_custom"},
	{[]string{"templates", "get", rawTestID}, returns[responses.Template]("GetTemplate", csvItemBody),
		"name,id,editor,subject,preheader,from,reply_to,has_draft,variables,created_at,updated_at,mjml,html,text,text_is_custom"},
	{[]string{"templates", "draft", "get", rawTestID}, returns[responses.TemplateDraft]("GetTemplateDraft", csvItemBody),
		"subject,template_id,preheader,from,reply_to,updated_at,variables,mjml,html,text,text_is_custom"},
	{[]string{"templates", "versions", "list", rawTestID}, returns[responses.TemplateVersionsResponse]("GetTemplateVersions", csvListBody),
		"version,id,published_at,published_by"},
	{[]string{"templates", "versions", "get", rawTestID, rawTestOtherID},
		returns[responses.TemplateVersionDetail]("GetTemplateVersion", csvItemBody),
		"version,id,published_at,published_by,subject,preheader,from,reply_to,variables,mjml,html,text,text_is_custom"},

	// Webhooks
	{[]string{"webhooks", "list"}, returns[responses.PaginatedWebhooksResponse]("ListWebhooks", csvListBody),
		"name,id,url,enabled,events,scope,domains,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},
	{[]string{"webhooks", "get", rawTestID}, returns[responses.Webhook]("GetWebhook", csvItemBody),
		"name,id,url,enabled,events,scope,domains,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},
	{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--all-events"},
		returns[responses.Webhook]("CreateWebhook", csvItemBody),
		"name,id,url,enabled,events,secret,scope,domains,created_at,updated_at"},
	{[]string{"webhooks", "update", rawTestID, "--name", "Hook"}, returns[responses.Webhook]("UpdateWebhook", csvItemBody),
		"name,id,url,enabled,events,scope,domains,created_at,updated_at,success_count,error_count,errors_since_last_success,last_request_at"},
}

// TestCSVColumnsAreTheFieldsOfTheTableView checks the CSV header of every
// list and single view. Each holds the fields the view's table shows, no
// fewer, in the same order.
func TestCSVColumnsAreTheFieldsOfTheTableView(t *testing.T) {
	contactsFile := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"user@example.com"}]}`)
	for _, view := range csvViews {
		t.Run(strings.Join(view.args, " "), func(t *testing.T) {
			args := slices.Clone(view.args)
			if i := slices.Index(args, csvContactsFile); i >= 0 {
				args[i] = contactsFile
			}
			mockClient := &mocks.MockClient{}
			mockClient.On("GetAccountID").Return(rawTestID).Maybe()
			expectAnyArgs(mockClient, view.call.method, view.call.result(t), nil)
			testutil.InjectClient(t, mockClient)

			stdout, stderr, code := runRoot(t, append(args, "--output", "csv")...)

			require.Equal(t, 0, code, stderr)
			assert.Equal(t, view.header, strings.SplitN(stdout, "\n", 2)[0])
		})
	}
}

// viewsWithoutID are the views of csvViews whose fixture sets no ID they
// show, with a value they show instead, or "" for a view whose fixture sets
// nothing it shows; the printer tests check those.
var viewsWithoutID = map[string]string{
	"campaigns test":       "user@example.com",
	"subaccounts usage":    "USD",
	"campaigns stats":      "",
	"campaigns recipients": "",
	"contacts batch":       "",
	"contacts memberships": "",
	"lists members":        "",
	"messages send":        "",
	"templates draft":      "",
}

// TestTableAndPlainOutputOfEveryView runs every view of csvViews through the
// root in table and plain output and checks that it prints the item's ID.
func TestTableAndPlainOutputOfEveryView(t *testing.T) {
	contactsFile := testutil.WriteTempFile(t, "contacts.json", `{"data":[{"email":"user@example.com"}]}`)
	for _, view := range csvViews {
		shows, withoutID := viewsWithoutID[strings.Join(view.args[:2], " ")]
		if !withoutID {
			shows = rawTestID
		}
		if shows == "" {
			continue
		}
		for _, format := range []string{"table", "plain"} {
			t.Run(format+" "+strings.Join(view.args, " "), func(t *testing.T) {
				args := slices.Clone(view.args)
				if i := slices.Index(args, csvContactsFile); i >= 0 {
					args[i] = contactsFile
				}
				mockClient := &mocks.MockClient{}
				mockClient.On("GetAccountID").Return(rawTestID).Maybe()
				expectAnyArgs(mockClient, view.call.method, view.call.result(t), nil)
				testutil.InjectClient(t, mockClient)

				stdout, stderr, code := runRoot(t, append(args, "--output", format)...)

				require.Equal(t, 0, code, stderr)
				assert.Contains(t, stdout, shows)
			})
		}
	}
}

// TestCSVListsWriteTheHeaderForAnEmptyPage checks that every list view of
// csvViews and every statistics report writes its header row, and nothing
// else, for a page without items, so a script finds the columns either way.
func TestCSVListsWriteTheHeaderForAnEmptyPage(t *testing.T) {
	type emptyView struct {
		args   []string
		method string
		result any
		header string
	}
	views := []emptyView{
		{[]string{"stats", "deliverability"}, "GetDeliverabilityStatistics", &responses.DeliverabilityStatisticsResponse{},
			"from_timestamp,to_timestamp,reception_count,delivered_count,deferred_count,bounced_count,failed_count,suppressed_count,opened_count,clicked_count,delivery_rate,open_rate"},
		{[]string{"stats", "bounces"}, "GetBounceStatistics", &responses.BounceStatisticsResponse{},
			"from_timestamp,to_timestamp,classification,count,percentage"},
		{[]string{"stats", "delivery-time"}, "GetDeliveryTimeStatistics", &responses.DeliveryTimeStatisticsResponse{},
			"from_timestamp,to_timestamp,delivered_count,avg_delivery_time,recipient_domain,domain_delivered_count,domain_delivery_time"},
	}
	for _, view := range csvViews {
		full := reflect.TypeOf(view.call.result(t)).Elem()
		if !strings.HasPrefix(full.Name(), "Paginated") && full.Name() != "TemplateVersionsResponse" {
			continue
		}
		views = append(views, emptyView{view.args, view.call.method, reflect.New(full).Interface(), view.header})
	}
	require.Len(t, views, 22)
	for _, view := range views {
		t.Run(strings.Join(view.args, " "), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On("GetAccountID").Return(rawTestID).Maybe()
			expectAnyArgs(mockClient, view.method, view.result, nil)
			testutil.InjectClient(t, mockClient)

			stdout, stderr, code := runRoot(t, append(slices.Clone(view.args), "--output", "csv")...)

			require.Equal(t, 0, code, stderr)
			assert.Equal(t, view.header+"\n", stdout)
		})
	}
}
