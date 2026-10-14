package printer

import (
	"time"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
)

// printerRender renders one fixture through one handler method.
type printerRender struct {
	render func(ResponseHandler) error
	// sparse marks a fixture whose optional values are all missing: nil and
	// zero times, zero UUIDs, empty strings and lists.
	sparse bool
}

// renders renders a fixture through each handler method of every printer:
// a full fixture of each, the sparse fixtures, and the views that only some
// fixtures reach, such as DNS records, message content, several items and
// the auth views.
func renders() map[string]printerRender {
	full := map[string]func(ResponseHandler) error{}
	sparse := map[string]func(ResponseHandler) error{}
	addFullRenders(full)
	addMoreRenders(full)
	addSparseRenders(sparse)

	all := make(map[string]printerRender, len(full)+len(sparse))
	for name, render := range full {
		all[name] = printerRender{render: render}
	}
	for name, render := range sparse {
		all["sparse "+name] = printerRender{render: render, sparse: true}
	}
	return all
}

// addFullRenders adds the full fixture of each handler method.
func addFullRenders(renders map[string]func(ResponseHandler) error) {
	domain, message, webhook, route := styleDomain(), styleMessage(), styleWebhook(), styleRoute()
	suppression, credential, key, subAccount := styleSuppression(), styleSMTP(), styleAPIKey(), styleSubAccount()
	usage := mocks.NewSubAccountUsage(0)
	usage.BillingPeriod = responses.SubAccountUsageBillingPeriod{Start: styleTime, End: styleTime}
	contact := contactFixture()
	list := listFixture()
	member := listMemberFixture()
	template := templateFixture()
	version := templateVersionDetailFixture()
	draft := templateDraftFixture()

	add(renders, map[string]func(ResponseHandler) error{
		"domain list": func(h ResponseHandler) error {
			return h.HandleDomainList(&responses.PaginatedDomainsResponse{Data: []responses.Domain{domain}}, ListConfig{})
		},
		"domain": func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) },
		"message list": func(h ResponseHandler) error {
			return h.HandleMessageList(&responses.PaginatedMessagesResponse{Data: []responses.Message{message}}, ListConfig{})
		},
		"message": func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) },
		"webhook list": func(h ResponseHandler) error {
			return h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: []responses.Webhook{webhook}}, ListConfig{})
		},
		"webhook":         func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"created webhook": func(h ResponseHandler) error { return h.HandleCreateWebhook(&webhook, ResultConfig{}) },
		"updated webhook": func(h ResponseHandler) error { return h.HandleUpdateWebhook(&webhook, ResultConfig{}) },
		"route list": func(h ResponseHandler) error {
			return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: []responses.Route{route}}, ListConfig{})
		},
		"route":         func(h ResponseHandler) error { return h.HandleSingleRoute(&route, SingleConfig{}) },
		"created route": func(h ResponseHandler) error { return h.HandleCreateRoute(&route, ResultConfig{}) },
		"updated route": func(h ResponseHandler) error { return h.HandleUpdateRoute(&route, ResultConfig{}) },
		"suppression list": func(h ResponseHandler) error {
			return h.HandleSuppressionList(&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{suppression}}, ListConfig{})
		},
		"created suppressions": func(h ResponseHandler) error {
			return h.HandleCreateSuppression(&responses.CreateSuppressionResponse{Data: []responses.Suppression{suppression}}, ResultConfig{})
		},
		"suppression check": func(h ResponseHandler) error {
			return h.HandleCheckSuppression(&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{suppression}}, CheckConfig{})
		},
		"smtp list": func(h ResponseHandler) error {
			return h.HandleSMTPList(&responses.PaginatedSMTPCredentialsResponse{Data: []responses.SMTPCredential{credential}}, ListConfig{})
		},
		"smtp":         func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) },
		"created smtp": func(h ResponseHandler) error { return h.HandleCreateSMTP(&credential, ResultConfig{}) },
		"api key list": func(h ResponseHandler) error {
			return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{key}}, ListConfig{})
		},
		"api key":         func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{}) },
		"created api key": func(h ResponseHandler) error { return h.HandleCreateAPIKey(&key, ResultConfig{}) },
		"updated api key": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(&key, ResultConfig{}) },
		"sub-account list": func(h ResponseHandler) error {
			return h.HandleSubAccountList(&responses.PaginatedSubAccountsResponse{Data: []responses.SubAccount{subAccount}}, ListConfig{})
		},
		"sub-account":       func(h ResponseHandler) error { return h.HandleSingleSubAccount(&subAccount, SingleConfig{}) },
		"sub-account usage": func(h ResponseHandler) error { return h.HandleSubAccountUsage(usage, SingleConfig{}) },
		"deliverability": func(h ResponseHandler) error {
			return h.HandleDeliverabilityStats(&responses.DeliverabilityStatisticsResponse{Data: []responses.DeliverabilityStatistics{
				{FromTimestamp: styleTime, ToTimestamp: styleTime.Add(time.Hour), ReceptionCount: 2, DeliveredCount: 1},
			}}, StatsConfig{})
		},
		"bounces": func(h ResponseHandler) error {
			return h.HandleBounceStats(&responses.BounceStatisticsResponse{Data: []responses.BounceStatistics{
				{FromTimestamp: styleTime, ToTimestamp: styleTime.Add(time.Hour), Bounces: []responses.Bounce{{Classification: "invalid", Count: 1}}},
			}}, StatsConfig{})
		},
		"delivery times": func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(styleDeliveryTimes(), StatsConfig{}) },
		"contact list":   func(h ResponseHandler) error { return h.HandleContactList(contactListFixture(), ListConfig{}) },
		"contact":        func(h ResponseHandler) error { return h.HandleSingleContact(&contact, SingleConfig{}) },
		"lists list":     func(h ResponseHandler) error { return h.HandleListsList(listsPageFixture(), ListConfig{}) },
		"list":           func(h ResponseHandler) error { return h.HandleSingleList(&list, SingleConfig{}) },
		"members list":   func(h ResponseHandler) error { return h.HandleListMemberList(embeddedMembersFixture(), ListConfig{}) },
		"member":         func(h ResponseHandler) error { return h.HandleSingleListMember(&member, SingleConfig{}) },
		"templates list": func(h ResponseHandler) error { return h.HandleTemplateList(templatesPageFixture(), ListConfig{}) },
		"template":       func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) },
		"template versions": func(h ResponseHandler) error {
			return h.HandleTemplateVersionList(templateVersionsFixture(), ListConfig{})
		},
		"template version": func(h ResponseHandler) error { return h.HandleSingleTemplateVersion(&version, SingleConfig{}) },
		"template draft":   func(h ResponseHandler) error { return h.HandleTemplateDraft(&draft, SingleConfig{}) },
	})
}

// addMoreRenders adds the views the full fixtures do not reach: DNS records,
// message content, sends, notices, several items and periods, and the auth
// views.
func addMoreRenders(renders map[string]func(ResponseHandler) error) {
	domain := styleDomain()
	domain.DNSRecords = []responses.DNSRecord{{Type: "TXT", Host: "example.com", Content: "v=spf1"}}
	message := styleMessage()
	content := "Hello\nthere"
	message.Content = &content
	route := styleRoute()
	route.Enabled = false
	usage := mocks.NewSubAccountUsage(5)
	usage.AllocationNote = "Shared by reception count"
	enabled, days := true, int32(30)
	account := &responses.Account{
		ID: styleID, Name: "Acme", CreatedAt: styleTime, UpdatedAt: styleTime,
		TrackOpens: &enabled, MessageDataRetention: &days,
	}
	domainName, seconds := "example.com", 90.0
	period := func(from time.Time) responses.DeliveryTimeStatistics {
		return responses.DeliveryTimeStatistics{
			FromTimestamp: from, ToTimestamp: from, DeliveredCount: 3, AvgDeliveryTime: 4000,
			DeliveryTimes: []responses.DeliveryTime{{RecipientDomain: &domainName, DeliveryTime: &seconds}},
		}
	}
	add(renders, map[string]func(ResponseHandler) error{
		"domain with DNS records": func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) },
		"message with content":    func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) },
		"two domains": func(h ResponseHandler) error {
			return h.HandleDomainList(&responses.PaginatedDomainsResponse{Data: []responses.Domain{domain, domain}}, ListConfig{})
		},
		"sent messages": func(h ResponseHandler) error {
			return h.HandleCreateMessage(&responses.CreateMessageResponse{Data: []responses.CreateSingleMessageResponse{
				{Recipient: common.Recipient{Email: "a@example.com"}, Status: "queued"},
				{Recipient: common.Recipient{Email: "b@example.com"}, Status: "queued"},
			}}, ResultConfig{})
		},
		"created disabled route": func(h ResponseHandler) error { return h.HandleCreateRoute(&route, ResultConfig{}) },
		"two suppressions created": func(h ResponseHandler) error {
			return h.HandleCreateSuppression(&responses.CreateSuppressionResponse{Data: []responses.Suppression{styleSuppression(), styleSuppression()}}, ResultConfig{})
		},
		"smtp sent": func(h ResponseHandler) error {
			return h.HandleSMTPSend(&SMTPSendResult{Success: true, MessageID: "<id>"}, ResultConfig{})
		},
		"smtp test fails": func(h ResponseHandler) error {
			return h.HandleSMTPSend(&SMTPSendResult{TestMode: true, Connected: true, Error: "refused"}, ResultConfig{})
		},
		"smtp test": func(h ResponseHandler) error {
			return h.HandleSMTPSend(&SMTPSendResult{TestMode: true, Success: true}, ResultConfig{})
		},
		"sub-account usage with removed": func(h ResponseHandler) error {
			return h.HandleSubAccountUsage(usage, SingleConfig{SuccessMessage: "Usage"})
		},
		"two deliverability periods": func(h ResponseHandler) error {
			return h.HandleDeliverabilityStats(&responses.DeliverabilityStatisticsResponse{Data: []responses.DeliverabilityStatistics{
				{FromTimestamp: styleTime, ReceptionCount: 2, DeliveredCount: 1}, {FromTimestamp: styleTime},
			}}, StatsConfig{Title: "Deliverability"})
		},
		"two bounce periods": func(h ResponseHandler) error {
			return h.HandleBounceStats(&responses.BounceStatisticsResponse{Data: []responses.BounceStatistics{
				{FromTimestamp: styleTime}, {FromTimestamp: styleTime, Bounces: []responses.Bounce{{Classification: "invalid", Count: 1}}},
			}}, StatsConfig{Title: "Bounces"})
		},
		"two delivery time periods": func(h ResponseHandler) error {
			return h.HandleDeliveryTimeStats(&responses.DeliveryTimeStatisticsResponse{Data: []responses.DeliveryTimeStatistics{
				period(styleTime), period(styleTime),
			}}, StatsConfig{Title: "Delivery times"})
		},
		"auth result": func(h ResponseHandler) error {
			return h.HandleAuthResult(AuthResult{Profile: "default", Message: "Switched"})
		},
		"auth result without a profile": func(h ResponseHandler) error {
			return h.HandleAuthResult(AuthResult{Message: "Logged out"})
		},
		"auth status": func(h ResponseHandler) error {
			return h.HandleAuthStatus(&AuthStatus{Profile: "default", Valid: ahasend.Bool(true), Account: account})
		},
		"empty": func(h ResponseHandler) error { return h.HandleEmpty("Nothing found") },
	})
}

// addSparseRenders adds the sparse fixtures: only the name or the email of
// each resource is set.
func addSparseRenders(renders map[string]func(ResponseHandler) error) {
	domain := responses.Domain{Domain: "example.com"}
	message := responses.Message{Subject: "Hello"}
	webhook := responses.Webhook{Name: "Events"}
	route := responses.Route{Name: "Inbound"}
	suppression := responses.Suppression{Email: "user@example.com"}
	credential := responses.SMTPCredential{Name: "Relay"}
	key := responses.APIKey{Label: "Production"}
	subAccount := responses.SubAccount{Name: "Acme"}
	contact := responses.Contact{Email: "user@example.com"}
	list := responses.ContactList{Name: "Newsletter"}
	member := responses.ListContact{Email: "member@example.com"}
	template := responses.Template{Name: "Welcome"}
	version := responses.TemplateVersionDetail{}
	draft := responses.TemplateDraft{Subject: "Draft"}
	add(renders, map[string]func(ResponseHandler) error{
		"domain list": func(h ResponseHandler) error {
			return h.HandleDomainList(&responses.PaginatedDomainsResponse{Data: []responses.Domain{domain}}, ListConfig{})
		},
		"domain": func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) },
		"message list": func(h ResponseHandler) error {
			return h.HandleMessageList(&responses.PaginatedMessagesResponse{Data: []responses.Message{message}}, ListConfig{})
		},
		"message": func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) },
		"sent message": func(h ResponseHandler) error {
			return h.HandleCreateMessage(&responses.CreateMessageResponse{Data: []responses.CreateSingleMessageResponse{
				{Recipient: common.Recipient{Email: "to@example.com"}, Status: "queued"},
			}}, ResultConfig{})
		},
		"webhook list": func(h ResponseHandler) error {
			return h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: []responses.Webhook{webhook}}, ListConfig{})
		},
		"webhook":         func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"created webhook": func(h ResponseHandler) error { return h.HandleCreateWebhook(&webhook, ResultConfig{}) },
		"updated webhook": func(h ResponseHandler) error { return h.HandleUpdateWebhook(&webhook, ResultConfig{}) },
		"route list": func(h ResponseHandler) error {
			return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: []responses.Route{route}}, ListConfig{})
		},
		"route": func(h ResponseHandler) error { return h.HandleSingleRoute(&route, SingleConfig{}) },
		"suppression list": func(h ResponseHandler) error {
			return h.HandleSuppressionList(&responses.PaginatedSuppressionsResponse{Data: []responses.Suppression{suppression}}, ListConfig{})
		},
		"smtp list": func(h ResponseHandler) error {
			return h.HandleSMTPList(&responses.PaginatedSMTPCredentialsResponse{Data: []responses.SMTPCredential{credential}}, ListConfig{})
		},
		"smtp":         func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) },
		"created smtp": func(h ResponseHandler) error { return h.HandleCreateSMTP(&credential, ResultConfig{}) },
		"smtp send": func(h ResponseHandler) error {
			return h.HandleSMTPSend(&SMTPSendResult{Success: true}, ResultConfig{})
		},
		"api key list": func(h ResponseHandler) error {
			return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{key}}, ListConfig{})
		},
		"api key":         func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{}) },
		"created api key": func(h ResponseHandler) error { return h.HandleCreateAPIKey(&key, ResultConfig{}) },
		"updated api key": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(&key, ResultConfig{}) },
		"sub-account list": func(h ResponseHandler) error {
			return h.HandleSubAccountList(&responses.PaginatedSubAccountsResponse{Data: []responses.SubAccount{subAccount}}, ListConfig{})
		},
		"sub-account": func(h ResponseHandler) error { return h.HandleSingleSubAccount(&subAccount, SingleConfig{}) },
		"deliverability": func(h ResponseHandler) error {
			return h.HandleDeliverabilityStats(&responses.DeliverabilityStatisticsResponse{Data: []responses.DeliverabilityStatistics{{}}}, StatsConfig{})
		},
		"delivery times": func(h ResponseHandler) error {
			return h.HandleDeliveryTimeStats(&responses.DeliveryTimeStatisticsResponse{Data: []responses.DeliveryTimeStatistics{
				{DeliveryTimes: []responses.DeliveryTime{{}}},
			}}, StatsConfig{})
		},
		"auth status": func(h ResponseHandler) error {
			return h.HandleAuthStatus(&AuthStatus{Profile: "default", Account: &responses.Account{Name: "Acme"}})
		},
		"contact": func(h ResponseHandler) error { return h.HandleSingleContact(&contact, SingleConfig{}) },
		"contact list": func(h ResponseHandler) error {
			return h.HandleContactList(&responses.PaginatedContactsResponse{Data: []responses.Contact{contact}}, ListConfig{})
		},
		"list": func(h ResponseHandler) error { return h.HandleSingleList(&list, SingleConfig{}) },
		"lists list": func(h ResponseHandler) error {
			return h.HandleListsList(&responses.PaginatedContactListsResponse{Data: []responses.ContactList{list}}, ListConfig{})
		},
		"member": func(h ResponseHandler) error { return h.HandleSingleListMember(&member, SingleConfig{}) },
		"members list": func(h ResponseHandler) error {
			return h.HandleListMemberList(&responses.PaginatedListContactsResponse{Data: []responses.ListContact{member}}, ListConfig{})
		},
		"template":         func(h ResponseHandler) error { return h.HandleSingleTemplate(&template, SingleConfig{}) },
		"template version": func(h ResponseHandler) error { return h.HandleSingleTemplateVersion(&version, SingleConfig{}) },
		"template draft":   func(h ResponseHandler) error { return h.HandleTemplateDraft(&draft, SingleConfig{}) },
	})
}

// add adds each render of more to renders, and panics on a name renders
// already holds.
func add(renders, more map[string]func(ResponseHandler) error) {
	for name, render := range more {
		if _, ok := renders[name]; ok {
			panic("two renders named " + name)
		}
		renders[name] = render
	}
}
