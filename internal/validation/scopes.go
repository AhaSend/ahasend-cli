package validation

import (
	"fmt"
	"strings"
)

// ScopeGroup is a group of API key scopes, such as the scopes of messages.
type ScopeGroup struct {
	Name   string
	Scopes []string
}

// ScopeGroups are the API key scopes the API accepts, grouped and ordered as
// the API reference lists them. The API key commands and the sub-account API
// key commands check and list scopes from this one list; a sub-account's key
// cannot have the parent-only scopes (see IsParentOnlyScope). A scope ending
// in {domain} takes one of the key's account's domains in its place, such as
// messages:send:{example.com}.
var ScopeGroups = []ScopeGroup{
	{"Messages", []string{
		"messages:send:all", "messages:send:{domain}",
		"messages:cancel:all", "messages:cancel:{domain}",
		"messages:read:all", "messages:read:{domain}",
	}},
	{"Domains", []string{"domains:read", "domains:write", "domains:delete:all", "domains:delete:{domain}"}},
	{"Accounts", []string{
		"accounts:read", "accounts:write", "accounts:billing",
		"accounts:members:read", "accounts:members:add", "accounts:members:update", "accounts:members:remove",
	}},
	{"Webhooks", []string{
		"webhooks:read:all", "webhooks:read:{domain}",
		"webhooks:write:all", "webhooks:write:{domain}",
		"webhooks:delete:all", "webhooks:delete:{domain}",
	}},
	{"Routes", []string{
		"routes:read:all", "routes:read:{domain}",
		"routes:write:all", "routes:write:{domain}",
		"routes:delete:all", "routes:delete:{domain}",
	}},
	{"Suppressions", []string{"suppressions:read", "suppressions:write", "suppressions:delete", "suppressions:wipe"}},
	{"Templates", []string{"templates:read", "templates:write", "templates:delete"}},
	{"Campaigns", []string{"campaigns:read", "campaigns:write", "campaigns:delete", "campaigns:send"}},
	{"Contacts", []string{"contacts:read", "contacts:write", "contacts:delete"}},
	{"Lists", []string{"lists:read", "lists:write", "lists:delete"}},
	{"SMTP Credentials", []string{
		"smtp-credentials:read:all", "smtp-credentials:read:{domain}",
		"smtp-credentials:write:all", "smtp-credentials:write:{domain}",
		"smtp-credentials:delete:all", "smtp-credentials:delete:{domain}",
	}},
	{"Statistics", []string{"statistics-transactional:read:all", "statistics-transactional:read:{domain}"}},
	{"API Keys", []string{"api-keys:read", "api-keys:write", "api-keys:delete"}},
	{"Sub-Accounts", []string{
		"sub-accounts:read", "sub-accounts:write", "sub-accounts:delete", "sub-accounts:suspend", "sub-accounts:usage",
	}},
	{"Sub-Account API Keys", []string{
		"sub-account-api-keys:read", "sub-account-api-keys:write", "sub-account-api-keys:delete",
	}},
}

// domainPlaceholder ends a scope that takes a domain.
const domainPlaceholder = "{domain}"

// ValidateScope checks that scope, a value of --scope, is one of ScopeGroups,
// with a domain name in place of {domain}. An unknown scope's error points to
// the help of helpCommand, which lists the scopes.
func ValidateScope(scope, helpCommand string) error {
	for _, group := range ScopeGroups {
		for _, valid := range group.Scopes {
			prefix, takesDomain := strings.CutSuffix(valid, domainPlaceholder)
			if !takesDomain {
				if scope == valid {
					return nil
				}
				continue
			}
			rest, ok := strings.CutPrefix(scope, prefix+"{")
			if !ok {
				continue
			}
			if domain, ok := strings.CutSuffix(rest, "}"); ok {
				if !IsDomainName(scopeDomain(domain)) {
					return InvalidValueError("--scope", scope, fmt.Sprintf("%q is not a domain name", domain))
				}
				return nil
			}
		}
	}
	return InvalidValueError("--scope", scope,
		fmt.Sprintf("not an API key scope; `%s --help` lists the scopes", helpCommand))
}

// CanonicalScope returns scope as the API compares scopes: the domain of a
// scope that takes one in lower case and as scopeDomain reads it.
func CanonicalScope(scope string) string {
	prefix, domain, found := strings.Cut(scope, "{")
	if !found {
		return scope
	}
	return prefix + "{" + strings.ToLower(scopeDomain(strings.TrimSuffix(domain, "}"))) + "}"
}

// scopeDomain returns the domain of a scope as the API reads it, without a
// trailing dot, as in example.com.
func scopeDomain(domain string) string {
	return strings.TrimSuffix(domain, ".")
}

// IsParentOnlyScope reports whether scope manages sub-accounts or their keys,
// which the API refuses to a key that a sub-account owns.
func IsParentOnlyScope(scope string) bool {
	return strings.HasPrefix(scope, "sub-accounts:") || strings.HasPrefix(scope, "sub-account-api-keys:")
}
