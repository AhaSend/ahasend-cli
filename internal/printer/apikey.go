package printer

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// apiKeyHeaders names the table column or row of each API key field.
var apiKeyHeaders = map[string]string{
	"label":         "Label",
	"id":            "ID",
	"public_key":    "Public Key",
	"secret_key":    "Secret Key",
	"account_id":    "Account ID",
	"scopes":        "Scopes",
	"ip_allow_list": "IP Allow List",
	"last_used_at":  "Last Used",
	"created_at":    "Created",
	"updated_at":    "Updated",
}

// The fields of each key of a list, which plain and CSV output show in this
// order; a table of keys shows apiKeyTableColumns.
var (
	// apiKeyListFields are the fields of each key of a list.
	apiKeyListFields = []string{"label", "id", "public_key", "scopes", "ip_allow_list", "last_used_at", "created_at", "updated_at"}
	// apiKeyTableColumns are the fields a table of keys shows.
	apiKeyTableColumns = []string{"label", "id", "public_key", "scopes", "last_used_at"}
)

// apiKeyView describes a view of one API key: the fields it shows, in this
// order in every format. Table and plain output show the scopes field as a
// list, one scope a line with the domain it is limited to.
type apiKeyView struct {
	fields []string
	// hideSecret shows the secret key as [HIDDEN] in table and plain output
	// and leaves it out of CSV output: the API returns it only on creation.
	hideSecret bool
	// created marks the view of a new key, which ends with the notice to
	// save its secret.
	created bool
}

var (
	apiKeyDetailView = apiKeyView{
		fields:     []string{"label", "id", "public_key", "secret_key", "account_id", "scopes", "ip_allow_list", "last_used_at", "created_at", "updated_at"},
		hideSecret: true,
	}
	// createdAPIKeyView holds the key's secret, which the API shows only on
	// creation.
	createdAPIKeyView = apiKeyView{
		fields:  []string{"label", "id", "public_key", "secret_key", "account_id", "scopes", "ip_allow_list", "created_at"},
		created: true,
	}
	updatedAPIKeyView = apiKeyView{
		fields: []string{"label", "id", "public_key", "account_id", "scopes", "ip_allow_list", "updated_at"},
	}
)

func (h *jsonHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	return h.printAPIResponse(response)
}

func (h *jsonHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	return h.printAPIResponse(key)
}

func (h *jsonHandler) HandleCreateAPIKey(key *responses.APIKey, config ResultConfig) error {
	return h.printAPIResponse(key)
}

func (h *jsonHandler) HandleUpdateAPIKey(key *responses.APIKey, config ResultConfig) error {
	return h.printAPIResponse(key)
}

func (h *tableHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	return writeTableList(h, config, pageOf(response), "API keys", apiKeyTableColumns, apiKeyHeaders, apiKeyDisplayFields)
}

func (h *tableHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.renderAPIKey(key, config.SuccessMessage, apiKeyDetailView)
}

// HandleCreateAPIKey shows the key's secret, which the API shows only on
// creation.
func (h *tableHandler) HandleCreateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	return h.renderAPIKey(key, config.SuccessMessage, createdAPIKeyView)
}

func (h *tableHandler) HandleUpdateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	return h.renderAPIKey(key, config.SuccessMessage, updatedAPIKeyView)
}

// renderAPIKey renders the fields of view of key, its scopes in their place,
// followed by the secret notice of a new key.
func (h *tableHandler) renderAPIKey(key *responses.APIKey, message string, view apiKeyView) error {
	if _, err := fmt.Fprintf(h.writer, "%s\n\n", message); err != nil {
		return err
	}
	beforeScopes, afterScopes := apiKeyDetailRows(key, view)
	scopes := []string{"Scopes", strings.Join(apiKeyScopeLines(key), "\n")}
	h.renderFieldTable(slices.Concat(beforeScopes, [][]string{scopes}, afterScopes))
	return writeAPIKeySecretNotice(h.writer, key, view)
}

func (h *plainHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	return writePlainList(h, config, pageOf(response), "API keys", func(key *responses.APIKey) error {
		return h.writeFieldLines(apiKeyListFields, apiKeyHeaders, apiKeyDisplayFields(key))
	})
}

func (h *plainHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		return h.HandleEmpty(config.EmptyMessage)
	}
	return h.writeAPIKey(key, config.SuccessMessage, apiKeyDetailView)
}

// HandleCreateAPIKey shows the key's secret, which the API shows only on
// creation.
func (h *plainHandler) HandleCreateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	return h.writeAPIKey(key, config.SuccessMessage, createdAPIKeyView)
}

func (h *plainHandler) HandleUpdateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return h.HandleEmpty(config.SuccessMessage)
	}
	return h.writeAPIKey(key, config.SuccessMessage, updatedAPIKeyView)
}

// writeAPIKey writes the fields of view of key, with its scopes in their
// place, followed by the secret notice of a new key.
func (h *plainHandler) writeAPIKey(key *responses.APIKey, message string, view apiKeyView) error {
	if _, err := fmt.Fprintln(h.writer, message); err != nil {
		return err
	}
	beforeScopes, afterScopes := apiKeyDetailRows(key, view)
	if err := writeLabelLines(h.writer, "", beforeScopes); err != nil {
		return err
	}
	if err := writeAPIKeyScopes(h.writer, "", key); err != nil {
		return err
	}
	if err := writeLabelLines(h.writer, "", afterScopes); err != nil {
		return err
	}
	return writeAPIKeySecretNotice(h.writer, key, view)
}

func (h *csvHandler) HandleAPIKeyList(response *responses.PaginatedAPIKeysResponse, config ListConfig) error {
	return writeCSVList(h, pageOf(response), apiKeyListFields, apiKeyCSVFields)
}

func (h *csvHandler) HandleSingleAPIKey(key *responses.APIKey, config SingleConfig) error {
	if key == nil {
		return nil // No CSV output for nil key
	}
	return h.writeFieldMaps(apiKeyDetailView.csvColumns(), apiKeyCSVFields(key))
}

// HandleCreateAPIKey writes the key with its secret, which the API shows only
// on creation.
func (h *csvHandler) HandleCreateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return nil
	}
	return h.writeFieldMaps(createdAPIKeyView.csvColumns(), apiKeyCSVFields(key))
}

func (h *csvHandler) HandleUpdateAPIKey(key *responses.APIKey, config ResultConfig) error {
	if key == nil {
		return nil
	}
	return h.writeFieldMaps(updatedAPIKeyView.csvColumns(), apiKeyCSVFields(key))
}

func apiKeyCSVFields(key *responses.APIKey) map[string]string {
	return map[string]string{
		"id":            formatUUID(key.ID),
		"label":         key.Label,
		"public_key":    key.PublicKey,
		"secret_key":    formatOptionalString(key.SecretKey),
		"scopes":        formatStringSlice(apiKeyScopeNames(key)),
		"ip_allow_list": formatStringSlice(key.IPAllowList),
		"last_used_at":  formatOptionalCSVTime(key.LastUsedAt),
		"account_id":    formatUUID(key.AccountID),
		"created_at":    formatCSVTime(key.CreatedAt),
		"updated_at":    formatCSVTime(key.UpdatedAt),
	}
}

// apiKeyDisplayFields maps each API key field to its table and plain value.
func apiKeyDisplayFields(key *responses.APIKey) map[string]string {
	return map[string]string{
		"label":         key.Label,
		"id":            formatUUID(key.ID),
		"public_key":    key.PublicKey,
		"secret_key":    formatOptionalString(key.SecretKey),
		"account_id":    formatUUID(key.AccountID),
		"scopes":        formatStringSlice(apiKeyScopeNames(key)),
		"ip_allow_list": formatIPAllowList(key.IPAllowList),
		"last_used_at":  formatOptionalTime(key.LastUsedAt),
		"created_at":    formatTime(key.CreatedAt),
		"updated_at":    formatTime(key.UpdatedAt),
	}
}

// apiKeyDetailRows returns the rows view shows of key before and after its
// scopes.
func apiKeyDetailRows(key *responses.APIKey, view apiKeyView) (beforeScopes, afterScopes [][]string) {
	scopes := slices.Index(view.fields, "scopes")
	values := apiKeyDisplayFields(key)
	if view.hideSecret {
		values["secret_key"] = hiddenSecret
	}
	return fieldRows(view.fields[:scopes], apiKeyHeaders, values), fieldRows(view.fields[scopes+1:], apiKeyHeaders, values)
}

// csvColumns returns the CSV columns of the view: its fields, without a
// hidden secret key.
func (view apiKeyView) csvColumns() []string {
	if view.hideSecret {
		return withoutField(view.fields, "secret_key")
	}
	return view.fields
}

// writeAPIKeySecretNotice tells to save the secret of a new key, when the API
// returned one.
func writeAPIKeySecretNotice(w io.Writer, key *responses.APIKey, view apiKeyView) error {
	if !view.created || formatOptionalString(key.SecretKey) == "" {
		return nil
	}
	_, err := fmt.Fprintf(w, "\n%s\n", apiKeySecretNotice)
	return err
}

// writeAPIKeyScopes writes a "Scopes:" line after indent, followed by each
// scope with the domain it is limited to. A key without scopes writes
// nothing.
func writeAPIKeyScopes(w io.Writer, indent string, key *responses.APIKey) error {
	if len(key.Scopes) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(w, "%sScopes:\n", indent); err != nil {
		return err
	}
	for _, line := range apiKeyScopeLines(key) {
		if _, err := fmt.Fprintf(w, "%s  - %s\n", indent, line); err != nil {
			return err
		}
	}
	return nil
}

// apiKeyScopeLines returns each scope of key with the domain it is limited
// to, if any, as table and plain output list them.
func apiKeyScopeLines(key *responses.APIKey) []string {
	lines := make([]string, len(key.Scopes))
	for i, scope := range key.Scopes {
		lines[i] = scope.Scope
		if domainID := formatOptionalUUID(scope.DomainID); domainID != "" {
			lines[i] += " (Domain: " + domainID + ")"
		}
	}
	return lines
}

// formatIPAllowList returns an API key's allowed source IPs, for table and
// plain output, or "any" when the key has no allow list and works from any
// IP. CSV output leaves the column empty instead, as it does every missing
// value.
func formatIPAllowList(list []string) string {
	if len(list) == 0 {
		return "any"
	}
	return strings.Join(list, ", ")
}

// apiKeyScopeNames returns the name of each scope of an API key.
func apiKeyScopeNames(key *responses.APIKey) []string {
	names := make([]string, len(key.Scopes))
	for i, scope := range key.Scopes {
		names[i] = scope.Scope
	}
	return names
}
