package printer

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The secrets of the fixtures: styleSMTP's password, styleAPIKey's secret
// key, styleWebhook's secret and styleRoute's secret.
const (
	smtpPasswordFixture  = "smtp-password"
	apiKeySecretFixture  = "sk_secret"
	webhookSecretFixture = "whsec_secret"
	routeSecretFixture   = "rsec_secret"
)

// secretViews renders each view of a resource with a secret, keyed by name.
func secretViews() map[string]func(ResponseHandler) error {
	credential, key, webhook, route := styleSMTP(), styleAPIKey(), styleWebhook(), styleRoute()
	return map[string]func(ResponseHandler) error{
		"smtp get": func(h ResponseHandler) error { return h.HandleSingleSMTP(&credential, SingleConfig{}) },
		"smtp list": func(h ResponseHandler) error {
			return h.HandleSMTPList(&responses.PaginatedSMTPCredentialsResponse{Data: []responses.SMTPCredential{credential}}, ListConfig{})
		},
		"smtp create": func(h ResponseHandler) error { return h.HandleCreateSMTP(&credential, ResultConfig{}) },
		"api key get": func(h ResponseHandler) error { return h.HandleSingleAPIKey(&key, SingleConfig{}) },
		"api key list": func(h ResponseHandler) error {
			return h.HandleAPIKeyList(&responses.PaginatedAPIKeysResponse{Data: []responses.APIKey{key}}, ListConfig{})
		},
		"api key update": func(h ResponseHandler) error { return h.HandleUpdateAPIKey(&key, ResultConfig{}) },
		"api key create": func(h ResponseHandler) error { return h.HandleCreateAPIKey(&key, ResultConfig{}) },
		"webhook get":    func(h ResponseHandler) error { return h.HandleSingleWebhook(&webhook, SingleConfig{}) },
		"webhook list": func(h ResponseHandler) error {
			return h.HandleWebhookList(&responses.PaginatedWebhooksResponse{Data: []responses.Webhook{webhook}}, ListConfig{})
		},
		"webhook update": func(h ResponseHandler) error { return h.HandleUpdateWebhook(&webhook, ResultConfig{}) },
		"webhook create": func(h ResponseHandler) error { return h.HandleCreateWebhook(&webhook, ResultConfig{}) },
		"route get":      func(h ResponseHandler) error { return h.HandleSingleRoute(&route, SingleConfig{}) },
		"route list": func(h ResponseHandler) error {
			return h.HandleRouteList(&responses.PaginatedRoutesResponse{Data: []responses.Route{route}}, ListConfig{})
		},
		"route update": func(h ResponseHandler) error { return h.HandleUpdateRoute(&route, ResultConfig{}) },
		"route create": func(h ResponseHandler) error { return h.HandleCreateRoute(&route, ResultConfig{}) },
	}
}

// TestSecretsShowOnlyOnCreation checks every table and plain view of a
// resource with a secret: a get shows a password or secret key as [HIDDEN],
// no other view shows a secret, and a create shows the one-time secret.
func TestSecretsShowOnlyOnCreation(t *testing.T) {
	tests := map[string]struct {
		label, value string // the row the view must show, if any
		secret       string // the secret only a create may show
	}{
		"smtp get":       {"Password", "[HIDDEN]", smtpPasswordFixture},
		"smtp list":      {"", "", smtpPasswordFixture},
		"smtp create":    {"Password", smtpPasswordFixture, ""},
		"api key get":    {"Secret Key", "[HIDDEN]", apiKeySecretFixture},
		"api key list":   {"", "", apiKeySecretFixture},
		"api key update": {"", "", apiKeySecretFixture},
		"api key create": {"Secret Key", apiKeySecretFixture, ""},
		"webhook get":    {"", "", webhookSecretFixture},
		"webhook list":   {"", "", webhookSecretFixture},
		"webhook update": {"", "", webhookSecretFixture},
		"webhook create": {"Secret", webhookSecretFixture, ""},
		"route get":      {"", "", routeSecretFixture},
		"route list":     {"", "", routeSecretFixture},
		"route update":   {"", "", routeSecretFixture},
		"route create":   {"Secret", routeSecretFixture, ""},
	}
	views := secretViews()
	for name, test := range tests {
		render := views[name]
		t.Run(name+"/table", func(t *testing.T) {
			output := renderWith(t, "table", render)
			if test.label != "" {
				assert.Contains(t, tableCells(output), []string{test.label, test.value})
			}
			if test.secret != "" {
				assert.NotContains(t, output, test.secret)
			}
		})
		t.Run(name+"/plain", func(t *testing.T) {
			output := renderWith(t, "plain", render)
			if test.label != "" {
				assert.Contains(t, output, "\n"+test.label+": "+test.value+"\n")
			}
			if test.secret != "" {
				assert.NotContains(t, output, test.secret)
			}
		})
	}
}

// TestWebhookAndRouteViewsShowNoSecretTheAPIDoesNotReturn checks that the
// get and update views of a webhook or a route have no secret field: the API
// returns the secret only when it creates the webhook or route.
func TestWebhookAndRouteViewsShowNoSecretTheAPIDoesNotReturn(t *testing.T) {
	views := secretViews()
	for _, name := range []string{"webhook get", "webhook update", "route get", "route update"} {
		for _, format := range []string{"table", "plain", "csv"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				assert.NotContains(t, strings.ToLower(renderWith(t, format, views[name])), "secret")
			})
		}
	}
}

// TestCreatedWebhookAndRouteShowTheSecretNotice checks that table and plain
// output of a new webhook or route tell to save its one-time secret, and CSV
// output has its secret column.
func TestCreatedWebhookAndRouteShowTheSecretNotice(t *testing.T) {
	tests := map[string]struct{ notice, secret string }{
		"webhook create": {webhookSecretNotice, webhookSecretFixture},
		"route create":   {routeSecretNotice, routeSecretFixture},
	}
	views := secretViews()
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			for _, format := range []string{"table", "plain"} {
				assert.Contains(t, renderWith(t, format, views[name]), "\n"+test.notice+"\n", format)
			}
			_, rows := readCSV(t, renderWith(t, "csv", views[name]))
			require.Len(t, rows, 1)
			assert.Equal(t, test.secret, rows[0]["secret"])
		})
	}
}
