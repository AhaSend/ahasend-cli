package cmdutil

import (
	"strings"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"

	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
)

// KeyOwner is whose API key a command creates or updates, which decides the
// scopes the key can have.
type KeyOwner int

const (
	// AccountKey is a key of the account the CLI authenticates as.
	AccountKey KeyOwner = iota
	// SubAccountKey is a key of one of its sub-accounts, which cannot have
	// the scopes that manage sub-accounts.
	SubAccountKey
)

// ownerScopes returns the scopes of group that a key of owner can have.
func ownerScopes(group validation.ScopeGroup, owner KeyOwner) []string {
	var scopes []string
	for _, scope := range group.Scopes {
		if owner == AccountKey || !validation.IsParentOnlyScope(scope) {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

// completeScopes makes the shell complete --scope of cmd with every scope a
// key of owner can have, a scope that takes a domain with {domain} to
// replace.
func completeScopes(cmd *cobra.Command, owner KeyOwner) {
	var scopes []string
	for _, group := range validation.ScopeGroups {
		scopes = append(scopes, ownerScopes(group, owner)...)
	}
	CompleteValues(cmd, "scope", scopes)
}

// ScopesHelp is the section of the help of a command that takes --scope that
// lists every scope a key of owner can have, from validation.ScopeGroups.
func ScopesHelp(owner KeyOwner) string {
	var help strings.Builder
	help.WriteString("Available Scopes:\n")
	for _, group := range validation.ScopeGroups {
		scopes := ownerScopes(group, owner)
		if len(scopes) == 0 {
			continue
		}
		line := "  " + group.Name + ":"
		for i, scope := range scopes {
			word := " " + scope
			if i < len(scopes)-1 {
				word += ","
			}
			if len(line)+len(word) > 80 {
				help.WriteString(line + "\n")
				line = "   "
			}
			line += word
		}
		help.WriteString(line + "\n")
	}
	if owner == SubAccountKey {
		help.WriteString(`
A scope ending in {domain} takes one of the sub-account's domains in its
place, such as messages:send:{example.com}.`)
		return help.String()
	}
	help.WriteString(`
A scope ending in {domain} takes one of your domains in its place, such as
messages:send:{example.com}. The domain must exist in your account.`)
	return help.String()
}

// AddAPIKeyCreateFlags adds the --label, --scope and --ip-allow-list flags of
// a command that creates a key of owner. ReadAPIKeyCreate reads them.
func AddAPIKeyCreateFlags(cmd *cobra.Command, owner KeyOwner) {
	cmd.Flags().String("label", "", "Label for the API key (required)")
	AddRepeatableFlag(cmd, "scope", "Scope to grant (required; can be used multiple times)")
	completeScopes(cmd, owner)
	AddRepeatableFlag(cmd, "ip-allow-list", "CIDR block or IP address allowed to use the key (can be used multiple times)")
}

// ReadAPIKeyCreate reads the flags AddAPIKeyCreateFlags adds into a create
// request for a key of owner. A missing label or scope and an invalid scope
// are validation errors.
func ReadAPIKeyCreate(cmd *cobra.Command, owner KeyOwner) (requests.CreateAPIKeyRequest, error) {
	label, err := ReadRequiredText(cmd, "label")
	if err != nil {
		return requests.CreateAPIKeyRequest{}, err
	}
	if err := CheckMaxLength("label", &label, MaxNameLength); err != nil {
		return requests.CreateAPIKeyRequest{}, err
	}
	scopes, err := readScopes(cmd, owner)
	if err != nil {
		return requests.CreateAPIKeyRequest{}, err
	}
	if scopes == nil {
		return requests.CreateAPIKeyRequest{}, errors.NewValidationError("--scope is required", nil)
	}
	ipAllowList, err := ReadRepeatable(cmd, "ip-allow-list", "")
	if err != nil {
		return requests.CreateAPIKeyRequest{}, err
	}
	if err := validation.CheckIPAllowList(ipAllowList); err != nil {
		return requests.CreateAPIKeyRequest{}, err
	}
	return requests.CreateAPIKeyRequest{
		Label:       label,
		Scopes:      scopes,
		IPAllowList: ipAllowList,
	}, nil
}

// AddAPIKeyUpdateFlags adds the --label, --scope, --ip-allow-list and
// --clear-ip-allow-list flags of a command that updates a key of owner.
// ReadAPIKeyUpdate reads them.
func AddAPIKeyUpdateFlags(cmd *cobra.Command, owner KeyOwner) {
	cmd.Flags().String("label", "", "New label for the API key")
	AddRepeatableFlag(cmd, "scope", "New scope to grant, replacing all scopes (can be used multiple times)")
	completeScopes(cmd, owner)
	AddRepeatableFlag(cmd, "ip-allow-list", "New CIDR block or IP address allowed to use the key, replacing the list (can be used multiple times)")
	cmd.Flags().Bool("clear-ip-allow-list", false, "Remove the IP allow list so the key works from any IP")
}

// ReadAPIKeyUpdate reads the flags AddAPIKeyUpdateFlags adds into an update
// request for a key of owner that holds only the fields given. Giving none of
// them, an empty value and an invalid scope are validation errors.
func ReadAPIKeyUpdate(cmd *cobra.Command, owner KeyOwner) (requests.UpdateAPIKeyRequest, error) {
	label, err := ReadNonEmpty(cmd, "label", "")
	if err != nil {
		return requests.UpdateAPIKeyRequest{}, err
	}
	if err := CheckMaxLength("label", label, MaxNameLength); err != nil {
		return requests.UpdateAPIKeyRequest{}, err
	}
	scopes, err := readScopes(cmd, owner)
	if err != nil {
		return requests.UpdateAPIKeyRequest{}, err
	}
	ipAllowList, err := ReadListUpdate(cmd, "ip-allow-list", "clear-ip-allow-list")
	if err != nil {
		return requests.UpdateAPIKeyRequest{}, err
	}
	if ipAllowList != nil {
		if err := validation.CheckIPAllowList(*ipAllowList); err != nil {
			return requests.UpdateAPIKeyRequest{}, err
		}
	}
	if label == nil && scopes == nil && ipAllowList == nil {
		return requests.UpdateAPIKeyRequest{}, errors.NewValidationError("at least one of --label, --scope, --ip-allow-list or --clear-ip-allow-list must be provided", nil)
	}

	req := requests.UpdateAPIKeyRequest{
		Label:       label,
		IPAllowList: ipAllowList,
	}
	if scopes != nil {
		req.Scopes = &scopes
	}
	// The SDK's own checks back up the ones above.
	if err := req.Validate(); err != nil {
		return requests.UpdateAPIKeyRequest{}, errors.NewValidationError(err.Error(), nil)
	}
	return req, nil
}

// readScopes returns the values of the --scope flag, or nil when it was not
// set, after checking each one is a valid scope for a key of owner given
// once: the API refuses a scope given twice, the domains of two scopes
// compared as the API compares them.
func readScopes(cmd *cobra.Command, owner KeyOwner) ([]string, error) {
	scopes, err := ReadRepeatable(cmd, "scope", "")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, scope := range scopes {
		if err := validation.ValidateScope(scope, cmd.CommandPath()); err != nil {
			return nil, err
		}
		if owner == SubAccountKey && validation.IsParentOnlyScope(scope) {
			return nil, validation.InvalidValueError("--scope", scope, "a sub-account's API key cannot have it")
		}
		key := validation.CanonicalScope(scope)
		if seen[key] {
			return nil, validation.InvalidValueError("--scope", scope, "given more than once")
		}
		seen[key] = true
	}
	return scopes, nil
}

// CheckCredentialFlags checks the --api-key and --account-id flags given on
// the command line: the two go together, the key cannot be empty, and the
// account ID must be a UUID. It runs before authentication.
func CheckCredentialFlags(cmd *cobra.Command) error {
	apiKeySet, accountIDSet := cmd.Flags().Changed("api-key"), cmd.Flags().Changed("account-id")
	switch {
	case apiKeySet && !accountIDSet:
		return errors.NewValidationError("--account-id is required when using --api-key", nil)
	case accountIDSet && !apiKeySet:
		return errors.NewValidationError("--api-key is required when using --account-id", nil)
	case !apiKeySet:
		return nil
	}
	apiKey, _ := cmd.Flags().GetString("api-key")
	if apiKey == "" {
		return errors.NewValidationError("--api-key cannot be empty", nil)
	}
	if !validation.IsPrintableASCII(apiKey) {
		return errors.NewValidationError("--api-key "+validation.APIKeyNotPrintable, nil)
	}
	accountID, _ := cmd.Flags().GetString("account-id")
	return validation.ValidateUUIDField("account ID", accountID)
}
