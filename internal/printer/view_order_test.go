package printer

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
)

// fieldKey names a field the same way in every format: a table or plain
// label such as "Account ID" or "Updated" and a CSV column such as
// account_id or updated_at give the same key.
func fieldKey(name string) string {
	key := strings.ReplaceAll(strings.ToLower(name), " ", "_")
	if key == "created" || key == "updated" {
		return key + "_at"
	}
	return key
}

func fieldKeys(names []string) []string {
	keys := make([]string, len(names))
	for i, name := range names {
		keys[i] = fieldKey(name)
	}
	return keys
}

// firstTableLabels returns the label of each row of the first table of a
// table output, which is a single view's Field/Value table.
func firstTableLabels(output string) []string {
	var labels []string
	header := true
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "└") {
			break
		}
		if !strings.HasPrefix(line, "│") {
			continue
		}
		if header {
			header = false
			continue
		}
		if label := strings.TrimSpace(strings.Split(strings.Trim(line, "│"), "│")[0]); label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

// plainFieldLine matches a plain line that starts a field: "Label: value", or
// "Label:" followed by indented lines.
var plainFieldLine = regexp.MustCompile(`^([A-Z][A-Za-z ]*):( |$)`)

func plainFieldLabels(output string) []string {
	var labels []string
	for _, line := range strings.Split(output, "\n") {
		if match := plainFieldLine.FindStringSubmatch(line); match != nil {
			labels = append(labels, match[1])
		}
	}
	return labels
}

// shared returns the keys of keys that others holds too, in order.
func shared(keys, others []string) []string {
	return slices.DeleteFunc(slices.Clone(keys), func(key string) bool { return !slices.Contains(others, key) })
}

// TestSingleViewsShowTheirFieldsInOneOrder checks that table, plain and CSV
// output of a single item show the fields they share in the same order.
func TestSingleViewsShowTheirFieldsInOneOrder(t *testing.T) {
	message, key, domain, webhook, route := styleMessage(), styleAPIKey(), styleDomain(), styleWebhook(), styleRoute()
	domain.DNSRecords = []responses.DNSRecord{{Type: "TXT", Host: "_dmarc.example.com", Content: "v=DMARC1; p=none;"}}
	content := "Hello"
	message.Content = &content
	sent := styleTime
	message.SentAt = &sent
	message.DeliveryAttempts = []responses.DeliveryEvent{{Time: styleTime, Status: "delivered", Log: "250 OK"}}
	credential, subAccount := styleSMTP(), styleSubAccount()
	views := map[string]func(ResponseHandler) error{
		"api key":         func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{}) },
		"created api key": func(h ResponseHandler) error { return h.HandleCreateAPIKey(&key, ResultConfig{}) },
		"updated api key": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(&key, ResultConfig{}) },
		"message":         func(h ResponseHandler) error { return h.HandleSingleMessage(&message, SingleConfig{}) },
		"domain":          func(h ResponseHandler) error { return h.HandleSingleDomain(&domain, SingleConfig{}) },
		"webhook":         func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"route":           func(h ResponseHandler) error { return h.HandleSingleRoute(&route, SingleConfig{}) },
		"created route":   func(h ResponseHandler) error { return h.HandleCreateRoute(&route, ResultConfig{}) },
		"created webhook": func(h ResponseHandler) error { return h.HandleCreateWebhook(&webhook, ResultConfig{}) },
		"smtp credential": func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) },
		"sub-account":     func(h ResponseHandler) error { return h.HandleSingleSubAccount(&subAccount, SingleConfig{}) },
	}
	for name, render := range views {
		t.Run(name, func(t *testing.T) {
			table := fieldKeys(firstTableLabels(renderWith(t, "table", render)))
			plain := fieldKeys(plainFieldLabels(renderWith(t, "plain", render)))
			header, _ := readCSV(t, renderWith(t, "csv", render))
			csv := fieldKeys(header)

			assert.Equal(t, shared(table, plain), shared(plain, table), "table and plain")
			assert.Equal(t, shared(table, csv), shared(csv, table), "table and CSV")
			assert.Equal(t, shared(plain, csv), shared(csv, plain), "plain and CSV")
			assert.GreaterOrEqual(t, len(shared(table, csv)), 5, "the formats share too few fields to compare")
		})
	}
}

// TestSummaryViewsShowTheirFieldsInOneOrder checks the views whose table
// output is not one Field/Value table: the fields plain and CSV output share,
// and those the table's first table shares with CSV, come in one order.
func TestSummaryViewsShowTheirFieldsInOneOrder(t *testing.T) {
	account := &responses.Account{ID: styleID, Name: "Acme", CreatedAt: styleTime, UpdatedAt: styleTime}
	status := &AuthStatus{Profile: "default", AccountID: styleID.String(), Account: account, Valid: ahasend.Bool(true)}
	usage := &responses.SubAccountUsageResponse{
		Currency: "USD", AllocationMethod: "reception", AllocationNote: "By reception",
		BillingPeriod: responses.SubAccountUsageBillingPeriod{Start: styleTime, End: styleTime},
	}
	views := map[string]func(ResponseHandler) error{
		"auth status":       func(h ResponseHandler) error { return h.HandleAuthStatus(status) },
		"sub-account usage": func(h ResponseHandler) error { return h.HandleSubAccountUsage(usage, SingleConfig{}) },
	}
	for name, render := range views {
		t.Run(name, func(t *testing.T) {
			plain := fieldKeys(plainFieldLabels(renderWith(t, "plain", render)))
			header, _ := readCSV(t, renderWith(t, "csv", render))
			csv := fieldKeys(header)

			assert.Equal(t, shared(plain, csv), shared(csv, plain), "plain and CSV")
			assert.GreaterOrEqual(t, len(shared(plain, csv)), 3, "the formats share too few fields to compare")
			table := fieldKeys(firstTableLabels(renderWith(t, "table", render)))
			assert.Equal(t, shared(table, csv), shared(csv, table), "table and CSV")
		})
	}
}
