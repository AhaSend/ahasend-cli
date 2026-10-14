package printer

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	styleTime   = time.Date(2026, time.September, 10, 8, 30, 0, 0, time.UTC)
	styleID     = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
	styleLongID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	// styleURL is longer than any column the tables used to cut.
	styleURL = "https://hooks.example.com/inbound/a-path-long-enough-to-be-cut-in-a-table"
)

func styleDomain() responses.Domain {
	checked := styleTime
	return responses.Domain{
		ID: styleID, AccountID: styleID, Domain: "example.com", DNSValid: true,
		SendingType: "transactional", CreatedAt: styleTime, UpdatedAt: styleTime, LastDNSCheckAt: &checked,
	}
}

func styleMessage() responses.Message {
	delivered := styleTime
	return responses.Message{
		ID: styleID, AccountID: styleID, DomainID: styleID, Sender: "from@example.com",
		Recipient: "to@example.com", Subject: "Hello", Status: "delivered", Direction: "outgoing",
		MessageID: "<id@example.com>", CreatedAt: styleTime, UpdatedAt: styleTime,
		DeliveredAt: &delivered, RetainUntil: styleTime,
	}
}

func styleWebhook() responses.Webhook {
	called := styleTime
	return responses.Webhook{
		ID: styleID, Name: "Events", URL: styleURL, Enabled: true, OnDelivered: true,
		Scope: "account", Domains: []string{"example.com"}, Secret: "whsec_secret",
		SuccessCount: 2, LastRequestAt: &called, CreatedAt: styleTime, UpdatedAt: styleTime,
	}
}

func styleRoute() responses.Route {
	called := styleTime
	return responses.Route{
		ID: styleID, Name: "Inbound", URL: styleURL, Recipient: "*@example.com", Enabled: true,
		Secret: "rsec_secret", LastRequestAt: &called, CreatedAt: styleTime, UpdatedAt: styleTime,
	}
}

func styleSuppression() responses.Suppression {
	return responses.Suppression{
		ID: styleID, Email: "user@example.com", Domain: "example.com", Reason: "bounce",
		CreatedAt: styleTime, ExpiresAt: styleTime,
	}
}

func styleSMTP() responses.SMTPCredential {
	return responses.SMTPCredential{
		ID: styleID, Name: "Relay", Username: "relay", Password: "smtp-password", Scope: "scoped",
		Domains: []string{"example.com", "news.example.com"}, CreatedAt: styleTime, UpdatedAt: styleTime,
	}
}

func styleAPIKey() responses.APIKey {
	used := styleTime
	secret := "sk_secret"
	domainID := styleID
	return responses.APIKey{
		ID: styleID, AccountID: styleID, Label: "Production", PublicKey: "pk_a_public_key_longer_than_twenty",
		SecretKey: &secret, IPAllowList: []string{"192.0.2.1"}, LastUsedAt: &used,
		CreatedAt: styleTime, UpdatedAt: styleTime,
		Scopes: []responses.APIKeyScope{
			{Scope: "messages:send:all", CreatedAt: styleTime, UpdatedAt: styleTime},
			{Scope: "domains:read", DomainID: &domainID, CreatedAt: styleTime, UpdatedAt: styleTime},
		},
	}
}

func styleSubAccount() responses.SubAccount {
	active := styleTime
	return responses.SubAccount{
		ID: styleID, ParentAccountID: styleID, Name: "Acme", Status: "active", Website: "acme.example",
		CreatedAt: styleTime, LastActivityAt: &active,
	}
}

func styleDeliveryTimes() *responses.DeliveryTimeStatisticsResponse {
	domain := "example.com"
	seconds := 1.5
	return &responses.DeliveryTimeStatisticsResponse{Data: []responses.DeliveryTimeStatistics{{
		FromTimestamp: styleTime, ToTimestamp: styleTime.Add(time.Hour), DeliveredCount: 3, AvgDeliveryTime: 2,
		DeliveryTimes: []responses.DeliveryTime{{RecipientDomain: &domain, DeliveryTime: &seconds}},
	}}}
}

func renderWith(t *testing.T, format string, render func(ResponseHandler) error) string {
	t.Helper()
	var output bytes.Buffer
	require.NoError(t, render(testHandler(t, format, &output, os.Stderr)))
	return output.String()
}

// tableCells returns the cells of each header and data line of a rendered
// table, in order, with the borders and padding removed.
func tableCells(text string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "│") {
			continue
		}
		parts := strings.Split(strings.Trim(line, "│"), "│")
		cells := make([]string, len(parts))
		for i, part := range parts {
			cells[i] = strings.TrimSpace(part)
		}
		rows = append(rows, cells)
	}
	return rows
}

func TestPrintersUseNoPlaceholderForMissingValues(t *testing.T) {
	placeholders := []string{
		"N/A", "Never", "Not yet", "Not configured", "Not generated", "None", "All domains",
		"Unknown", "[Not provided", uuid.Nil.String(),
	}
	for _, format := range []string{"table", "plain", "csv"} {
		for name, r := range renders() {
			if !r.sparse {
				continue
			}
			t.Run(format+"/"+name, func(t *testing.T) {
				output := renderWith(t, format, r.render)
				for _, placeholder := range placeholders {
					assert.NotContains(t, output, placeholder)
				}
			})
		}
	}
}

func TestPlainLeavesMissingValuesOut(t *testing.T) {
	for name, r := range renders() {
		if !r.sparse {
			continue
		}
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "plain", r.render)
			assert.NotRegexp(t, `(?m):[ \t]+$`, output, "no label without a value")
		})
	}
}

func TestDomainLastDNSCheckIsMissingInEveryFormat(t *testing.T) {
	domain := styleDomain()
	domain.LastDNSCheckAt = nil
	single := func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) }
	list := func(h ResponseHandler) error {
		return h.HandleDomainList(&responses.PaginatedDomainsResponse{Data: []responses.Domain{domain}}, ListConfig{})
	}

	assert.Contains(t, tableCells(renderWith(t, "table", single)), []string{"Last DNS Check", "-"})
	assert.NotContains(t, renderWith(t, "plain", single), "Last DNS Check")
	assert.NotContains(t, renderWith(t, "plain", list), "Last DNS Check")
	for _, render := range []func(ResponseHandler) error{single, list} {
		_, rows := readCSV(t, renderWith(t, "csv", render))
		assert.Equal(t, "", rows[0]["last_dns_check_at"])
	}
}

func TestZeroUUIDIsMissing(t *testing.T) {
	subAccount := styleSubAccount()
	subAccount.ParentAccountID = uuid.Nil
	render := func(h ResponseHandler) error { return h.HandleSingleSubAccount(&subAccount, SingleConfig{}) }

	assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Parent Account ID", "-"})
	assert.NotContains(t, renderWith(t, "plain", render), "Parent Account ID")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "", rows[0]["parent_account_id"])
}

func TestZeroTimeIsMissing(t *testing.T) {
	message := styleMessage()
	message.RetainUntil = time.Time{}
	render := func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) }

	assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Retain Until", "-"})
	assert.NotContains(t, renderWith(t, "plain", render), "Retain Until")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "", rows[0]["retain_until"])
}

// TestEmptyIPAllowListReadsAnyForDisplay checks that a key without an IP
// allow list reads "any" in table and plain output, and that CSV output
// leaves the column empty, as it does every missing value.
func TestEmptyIPAllowListReadsAnyForDisplay(t *testing.T) {
	key := styleAPIKey()
	key.IPAllowList = nil
	render := func(h ResponseHandler) error {
		return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{key}}, ListConfig{})
	}
	single := func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{}) }

	assert.Contains(t, tableCells(renderWith(t, "table", single)), []string{"IP Allow List", "any"})
	assert.Contains(t, renderWith(t, "plain", render), "\nIP Allow List: any\n")
	_, rows := readCSV(t, renderWith(t, "csv", render))
	assert.Equal(t, "", rows[0]["ip_allow_list"])
}

// TestNoDomainFilterReadsAllForDisplay checks that a webhook or an SMTP
// credential limited to no domain reads "all" in table and plain output, and
// that CSV output leaves the column empty.
func TestNoDomainFilterReadsAllForDisplay(t *testing.T) {
	webhook := styleWebhook()
	webhook.Domains = nil
	credential := styleSMTP()
	credential.Domains = nil
	renders := map[string]func(h ResponseHandler) error{
		"webhook": func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"smtp":    func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) },
	}
	for name, render := range renders {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, tableCells(renderWith(t, "table", render)), []string{"Domains", "all"})
			assert.Contains(t, renderWith(t, "plain", render), "\nDomains: all\n")
			_, rows := readCSV(t, renderWith(t, "csv", render))
			assert.Equal(t, "", rows[0]["domains"])
		})
	}
}

// emoji matches the pictographs and dingbats the CLI's output used to carry.
var emoji = regexp.MustCompile(`[\x{1F300}-\x{1FAFF}\x{2600}-\x{27BF}\x{2B50}]`)

func TestPrintersWriteNoEmoji(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		for name, r := range renders() {
			t.Run(format+"/"+name, func(t *testing.T) {
				assert.NotRegexp(t, emoji, renderWith(t, format, r.render))
			})
		}
	}
}

func TestListTablesShowFullIDsAndValues(t *testing.T) {
	route, key := styleRoute(), styleAPIKey()
	route.ID, key.ID = uuid.MustParse(styleLongID), uuid.MustParse(styleLongID)
	key.Scopes = append(key.Scopes, responses.APIKeyScope{Scope: "webhooks:write:all"})

	routes := renderWith(t, "table", func(h ResponseHandler) error {
		return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: []responses.Route{route}}, ListConfig{})
	})
	keys := renderWith(t, "table", func(h ResponseHandler) error {
		return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{key}}, ListConfig{})
	})

	assert.Equal(t, []string{"Inbound", styleLongID, styleURL}, tableCells(routes)[1][:3])
	assert.Equal(t, []string{"Production", styleLongID, key.PublicKey,
		"messages:send:all, domains:read, webhooks:write:all"}, tableCells(keys)[1][:4])
	assert.NotContains(t, routes+keys, "...")
}

// everyList renders a page of count items of each list command, pointing to
// a next and a previous page, the way the list commands configure it.
func everyList(count int) map[string]func(ResponseHandler) error {
	next, previous := "next-page", "previous-page"
	paged := common.PaginationInfo{HasMore: true, NextCursor: &next, PreviousCursor: &previous}
	forward := paged
	config := ListConfig{SuccessMessage: "Retrieved", EmptyMessage: "None found"}

	return map[string]func(ResponseHandler) error{
		"domains": func(h ResponseHandler) error {
			return h.HandleDomainList(&responses.PaginatedDomainsResponse{Data: repeat(styleDomain(), count), Pagination: forward}, config)
		},
		"messages": func(h ResponseHandler) error {
			return h.HandleMessageList(&responses.PaginatedMessagesResponse{Data: repeat(styleMessage(), count), Pagination: forward}, config)
		},
		"webhooks": func(h ResponseHandler) error {
			return h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: repeat(styleWebhook(), count), Pagination: forward}, config)
		},
		"routes": func(h ResponseHandler) error {
			return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: repeat(styleRoute(), count), Pagination: forward}, config)
		},
		"suppressions": func(h ResponseHandler) error {
			return h.HandleSuppressionList(&responses.PaginatedSuppressionsResponse{Data: repeat(styleSuppression(), count), Pagination: forward}, config)
		},
		"SMTP credentials": func(h ResponseHandler) error {
			return h.HandleSMTPList(&responses.PaginatedSMTPCredentialsResponse{Data: repeat(styleSMTP(), count), Pagination: forward}, config)
		},
		"API keys": func(h ResponseHandler) error {
			return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: repeat(styleAPIKey(), count), Pagination: forward}, config)
		},
		"sub-accounts": func(h ResponseHandler) error {
			return h.HandleSubAccountList(&responses.PaginatedSubAccountsResponse{Data: repeat(styleSubAccount(), count), Pagination: forward}, config)
		},
	}
}

func repeat[T any](item T, count int) []T {
	items := make([]T, count)
	for i := range items {
		items[i] = item
	}
	return items
}

func TestEveryListEndsWithTheShowingLineAndThePageFlags(t *testing.T) {
	for _, format := range []string{"table", "plain"} {
		for _, count := range []int{1, 2} {
			for itemName, render := range everyList(count) {
				t.Run(format+"/"+itemName, func(t *testing.T) {
					output := renderWith(t, format, render)
					if count == 1 {
						itemName = strings.TrimSuffix(itemName, "s")
					}
					assert.True(t, strings.HasSuffix(output, "\nShowing "+formatInt(count)+" "+itemName+
						" (more available)\n--after next-page\n--before previous-page\n"), output)
				})
			}
		}
	}
}

// listTableHeads names the first two columns of each list table: the name,
// or the most identifying field of a resource without one, then the ID.
var listTableHeads = map[string][]string{
	"domains":          {"DOMAIN", "ID"},
	"messages":         {"SUBJECT", "ID"},
	"webhooks":         {"NAME", "ID"},
	"routes":           {"NAME", "ID"},
	"suppressions":     {"EMAIL", "ID"},
	"SMTP credentials": {"NAME", "ID"},
	"API keys":         {"LABEL", "ID"},
	"sub-accounts":     {"NAME", "ID"},
}

// listView is the fields of each item of a list view: every field, which
// plain and CSV output show, the summary columns of its table, and the label
// of each field.
type listView struct {
	fields, columns []string
	headers         map[string]string
}

// listViews holds every list view, keyed by the names of everyList.
var listViews = map[string]listView{
	"domains":              {domainListFields, domainTableColumns, domainHeaders},
	"messages":             {messageListFields, messageTableColumns, messageHeaders},
	"messages with detail": {detailedMessageListFields, detailedMessageTableColumns, messageHeaders},
	"webhooks":             {webhookListFields, webhookTableColumns, webhookHeaders},
	"routes":               {routeFields, routeTableColumns, routeHeaders},
	"suppressions":         {suppressionFields, suppressionTableColumns, suppressionHeaders},
	"SMTP credentials":     {smtpListFields, smtpTableColumns, smtpHeaders},
	"API keys":             {apiKeyListFields, apiKeyTableColumns, apiKeyHeaders},
	"sub-accounts":         {subAccountListFields, subAccountTableColumns, subAccountHeaders},
}

// TestListTableColumnsAreFieldsOfThePlainAndCSVList checks that the table of
// every list shows a summary of its fields: the name and the ID, then other
// fields in the order of plain and CSV output.
func TestListTableColumnsAreFieldsOfThePlainAndCSVList(t *testing.T) {
	for name := range everyList(1) {
		require.Contains(t, listViews, name)
	}
	for name, view := range listViews {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, view.fields[:2], view.columns[:2])
			remaining := view.fields
			for _, column := range view.columns {
				i := slices.Index(remaining, column)
				require.GreaterOrEqual(t, i, 0, "column %s is not a field after the previous column", column)
				remaining = remaining[i+1:]
			}
		})
	}
}

// TestListTablesShowTheirColumnsWithThePlainLabels checks that the table of
// every list shows its summary columns, each headed by the label plain
// output gives the field.
func TestListTablesShowTheirColumnsWithThePlainLabels(t *testing.T) {
	for name, render := range everyList(1) {
		t.Run(name, func(t *testing.T) {
			view := listViews[name]
			want := make([]string, len(view.columns))
			for i, column := range view.columns {
				require.NotEmpty(t, view.headers[column], column)
				want[i] = strings.ToUpper(view.headers[column])
			}
			assert.Equal(t, want, tableCells(renderWith(t, "table", render))[0])
		})
	}
}

func TestListTablesPutTheNameThenTheID(t *testing.T) {
	for name, render := range everyList(1) {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, listTableHeads, name)
			assert.Equal(t, listTableHeads[name], tableCells(renderWith(t, "table", render))[0][:2])
		})
	}
}

// listPlainHeads names the first two fields of each item of a plain list: the
// same name and ID as the table's first two columns.
var listPlainHeads = map[string][]string{
	"domains":          {"Domain", "ID"},
	"messages":         {"Subject", "ID"},
	"webhooks":         {"Name", "ID"},
	"routes":           {"Name", "ID"},
	"suppressions":     {"Email", "ID"},
	"SMTP credentials": {"Name", "ID"},
	"API keys":         {"Label", "ID"},
	"sub-accounts":     {"Name", "ID"},
}

// plainItems returns the label of each field line of each item of a plain
// list, which follows the success line and ends before the showing line.
func plainItems(output string) [][]string {
	blocks := strings.Split(strings.SplitN(output, "\n", 2)[1], "\n\n")
	var items [][]string
	for _, block := range blocks {
		if strings.HasPrefix(block, "Showing ") {
			break
		}
		var labels []string
		for _, line := range strings.Split(strings.TrimSpace(block), "\n") {
			labels = append(labels, strings.SplitN(line, ":", 2)[0])
		}
		items = append(items, labels)
	}
	return items
}

func TestPlainListsWriteEachItemAsFlatLinesWithTheNameThenTheID(t *testing.T) {
	for name, render := range everyList(2) {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, listPlainHeads, name)
			output := renderWith(t, "plain", render)
			items := plainItems(output)
			require.Len(t, items, 2, output)
			for _, labels := range items {
				assert.Equal(t, listPlainHeads[name], labels[:2])
			}
			assert.NotRegexp(t, `(?m)^ +[A-Z][A-Za-z ]*: `, output, "no indented field lines")
		})
	}
}

// listCSVHeads names the first two columns of each list's csv.
var listCSVHeads = map[string][]string{
	"domains":          {"domain", "id"},
	"messages":         {"subject", "id"},
	"webhooks":         {"name", "id"},
	"routes":           {"name", "id"},
	"suppressions":     {"email", "id"},
	"SMTP credentials": {"name", "id"},
	"API keys":         {"label", "id"},
	"sub-accounts":     {"name", "id"},
}

func TestCSVListsPutTheNameThenTheIDAndNoObjectColumn(t *testing.T) {
	for name, render := range everyList(1) {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, listCSVHeads, name)
			headers, _ := readCSV(t, renderWith(t, "csv", render))
			assert.Equal(t, listCSVHeads[name], headers[:2])
			assert.NotContains(t, headers, "object")
		})
	}
}

func TestPlainSingleViewsDoNotStartWithTheID(t *testing.T) {
	for name, r := range renders() {
		if r.sparse {
			continue
		}
		t.Run(name, func(t *testing.T) {
			output := strings.TrimLeft(renderWith(t, "plain", r.render), "\n")
			assert.False(t, strings.HasPrefix(output, "ID:"), output)
		})
	}
}

// callLabels are the call counter labels of webhooks and routes.
var callLabels = []string{"Successful Calls", "Failed Calls", "Errors Since Last Success", "Last Called"}

func TestWebhooksAndRoutesShowTheirCallsWithTheSameLabelsInEveryView(t *testing.T) {
	route, webhook := styleRoute(), styleWebhook()
	views := map[string]func(ResponseHandler) error{
		"route list": func(h ResponseHandler) error {
			return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: []responses.Route{route}}, ListConfig{})
		},
		"route":         func(h ResponseHandler) error { return h.HandleSingleRoute(&route, SingleConfig{}) },
		"updated route": func(h ResponseHandler) error { return h.HandleUpdateRoute(&route, ResultConfig{}) },
		"webhook list": func(h ResponseHandler) error {
			return h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: []responses.Webhook{webhook}}, ListConfig{})
		},
		"webhook":         func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"updated webhook": func(h ResponseHandler) error { return h.HandleUpdateWebhook(&webhook, ResultConfig{}) },
	}
	for name, render := range views {
		t.Run(name, func(t *testing.T) {
			plain := renderWith(t, "plain", render)
			for _, label := range callLabels {
				assert.Contains(t, plain, "\n"+label+": ")
			}
			assert.NotContains(t, plain, "Webhook ID")
			if strings.HasSuffix(name, "list") {
				return
			}
			table := renderWith(t, "table", render)
			for _, label := range callLabels {
				assert.Contains(t, table, "│ "+label+" ")
			}
		})
	}
}

func TestPlainSingleViewsStartTheFieldsRightAfterTheHeading(t *testing.T) {
	credential, key, subAccount, domain := styleSMTP(), styleAPIKey(), styleSubAccount(), styleDomain()
	views := map[string]func(ResponseHandler) error{
		"smtp": func(h ResponseHandler) error {
			return h.HandleSingleSMTP(&credential, SingleConfig{SuccessMessage: "Done"})
		},
		"created smtp": func(h ResponseHandler) error {
			return h.HandleCreateSMTP(&credential, ResultConfig{SuccessMessage: "Done"})
		},
		"api key":         func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{SuccessMessage: "Done"}) },
		"created api key": func(h ResponseHandler) error { return h.HandleCreateAPIKey(&key, ResultConfig{SuccessMessage: "Done"}) },
		"updated api key": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(&key, ResultConfig{SuccessMessage: "Done"}) },
		"sub-account": func(h ResponseHandler) error {
			return h.HandleSingleSubAccount(&subAccount, SingleConfig{SuccessMessage: "Done"})
		},
		"domain": func(h ResponseHandler) error {
			return h.HandleSingleDomain(&domain, SingleConfig{SuccessMessage: "Done"})
		},
	}
	for name, render := range views {
		t.Run(name, func(t *testing.T) {
			output := renderWith(t, "plain", render)
			assert.Regexp(t, `^Done\n(Name|Label|Domain): `, output)
		})
	}
}

func TestWebhookListShowsEventsInEveryFormat(t *testing.T) {
	for _, format := range []string{"table", "plain", "csv"} {
		t.Run(format, func(t *testing.T) {
			var out bytes.Buffer
			h := testHandler(t, format, &out, io.Discard)
			webhook := styleWebhook()

			require.NoError(t, h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: []responses.Webhook{webhook}}, ListConfig{}))
			assert.Contains(t, out.String(), formatWebhookEvents(&webhook))
		})
	}
}

// TestDeliveryTimeTableNamesThePerDomainColumnAndCountsDomains checks the
// hyphenated header and the singular and plural domain counts.
func TestDeliveryTimeTableNamesThePerDomainColumnAndCountsDomains(t *testing.T) {
	stats := styleDeliveryTimes()
	output := renderWith(t, "table", func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(stats, StatsConfig{}) })
	assert.Contains(t, output, "PER-DOMAIN DELIVERY TIMES")
	assert.Equal(t, "1 domain", tableCells(output)[1][3])

	stats.Data[0].DeliveryTimes = append(stats.Data[0].DeliveryTimes, stats.Data[0].DeliveryTimes[0])
	output = renderWith(t, "table", func(h ResponseHandler) error { return h.HandleDeliveryTimeStats(stats, StatsConfig{}) })
	assert.Equal(t, "2 domains", tableCells(output)[1][3])
}

// TestBounceTableNamesItsColumnsLikePlainOutputAndCountsTypes checks the
// header that matches the plain label, the singular and plural type counts,
// and the blank line before each period's classifications.
func TestBounceTableNamesItsColumnsLikePlainOutputAndCountsTypes(t *testing.T) {
	period := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	stats := &responses.BounceStatisticsResponse{Data: []responses.BounceStatistics{{
		FromTimestamp: period, ToTimestamp: period.Add(time.Hour),
		Bounces: []responses.Bounce{{Classification: "BadMailbox", Count: 3}},
	}}}
	render := func(h ResponseHandler) error { return h.HandleBounceStats(stats, StatsConfig{}) }

	output := renderWith(t, "table", render)
	assert.Equal(t, "BOUNCE CLASSIFICATIONS", tableCells(output)[0][2])
	assert.Equal(t, "1 type", tableCells(output)[1][2])
	assert.Contains(t, output, "┘\n\nBounce Classifications - ")
	assert.Contains(t, renderWith(t, "plain", render), "  Bounce Classifications:\n")

	stats.Data[0].Bounces = append(stats.Data[0].Bounces, responses.Bounce{Classification: "Spam", Count: 1})
	assert.Equal(t, "2 types", tableCells(renderWith(t, "table", render))[1][2])
}

func TestMessageContentPreviewCutsWholeCharacters(t *testing.T) {
	message := styleMessage()
	content := strings.Repeat("é", 120)
	message.Content = &content
	render := func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) }

	assert.Contains(t, tableCells(renderWith(t, "table", render)),
		[]string{"Content", strings.Repeat("é", 97) + "..."})
}
