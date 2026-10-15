// Package validation provides the input checks that CLI commands share:
//
//   - Email address format
//   - UUID IDs and path arguments
//   - Domain name format
//   - Page limits
//   - API key scopes and IP allow lists
//   - API URLs and loopback hosts
//
// A check of a flag or an argument returns a validation error from the
// errors package, which exits with 4; CheckAPIURL returns only the reason,
// which its callers put in their own error.
package validation

import (
	"fmt"
	"net"
	"net/mail"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/AhaSend/ahasend-cli/internal/errors"
)

// Domain validation regex pattern
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateEmail checks that email, the value of the input name, a flag such
// as "--from" or an argument such as "email", is an email address (see
// IsEmailAddress): `invalid --from value "x": not an email address`. An
// empty value, or one of only white space, fails with
// `--from cannot be empty`.
func ValidateEmail(name, email string) error {
	if strings.TrimSpace(email) == "" {
		return errors.NewValidationError(name+" cannot be empty", nil)
	}
	if !IsEmailAddress(email) {
		return InvalidValueError(name, email, "not an email address")
	}
	return nil
}

// ValidateOptionalEmail validates email like ValidateEmail, when there is
// one.
func ValidateOptionalEmail(name string, email *string) error {
	if email == nil {
		return nil
	}
	return ValidateEmail(name, *email)
}

// IsEmailAddress reports whether email is a bare email address: one that
// net/mail parses as itself, with no display name, angle brackets, quotes or
// surrounding spaces. It accepts addresses such as o'brien@example.com and
// non-ASCII ones, which the mail format allows, and leaves deliverability to
// the API.
func IsEmailAddress(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

// ValidateUUIDField validates that a named field contains a valid UUID format
func ValidateUUIDField(fieldName, value string) error {
	_, err := ParseUUIDField(fieldName, value)
	return err
}

// ParseUUIDField parses a named field that must contain a UUID, returning the
// same errors as ValidateUUIDField.
func ParseUUIDField(fieldName, value string) (uuid.UUID, error) {
	if value == "" {
		return uuid.Nil, errors.NewValidationError(fieldName+" cannot be empty", nil)
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, InvalidValueError(fieldName, value, "not a UUID")
	}
	return id, nil
}

// ParseMessageID checks a message ID argument, which is a UUID or the
// Message-ID messages send returns: a UUID and a domain in angle brackets,
// such as <uuid@example.com>. It returns the ID to send to the API.
func ParseMessageID(fieldName, value string) (string, error) {
	if value == "" {
		return "", errors.NewValidationError(fieldName+" cannot be empty", nil)
	}
	inner, bracketed := strings.CutPrefix(value, "<")
	if !bracketed {
		if id, err := uuid.Parse(value); err == nil {
			return id.String(), nil
		}
	}
	inner, closed := strings.CutSuffix(inner, ">")
	local, domain, found := strings.Cut(inner, "@")
	if _, err := uuid.Parse(local); !bracketed || !closed || !found || err != nil || !IsDomainName(domain) {
		return "", InvalidValueError(fieldName, value, "not a UUID or <uuid@domain>")
	}
	return value, nil
}

// ParseUUIDs parses each of values, IDs of the field fieldName such as
// "list ID", like ParseUUIDField. It returns nil for no values.
func ParseUUIDs(fieldName string, values []string) ([]uuid.UUID, error) {
	if values == nil {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(values))
	for i, value := range values {
		id, err := ParseUUIDField(fieldName, value)
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}

// ValidatePathSegment validates that the argument fieldName, such as "domain"
// or "contact ID or email", can address one path segment: not empty or blank,
// not a dot segment and free of control characters, the values the SDK
// refuses to put in a path or that address nothing. Its format is left to
// the API.
func ValidatePathSegment(fieldName, value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.NewValidationError(fieldName+" cannot be empty", nil)
	}
	if value == "." || value == ".." || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return InvalidValueError(fieldName, value, "not a "+fieldName)
	}
	return nil
}

// ValidatePageLimit validates an optional page limit
func ValidatePageLimit(limit int32) error {
	if limit < 0 || limit > 100 {
		return errors.NewValidationError("--limit must be between 0 and 100", nil)
	}
	return nil
}

// ValidateOptionalDomainName validates domain like ValidateDomainName, when
// there is one.
func ValidateOptionalDomainName(name string, domain *string) error {
	if domain == nil {
		return nil
	}
	return ValidateDomainName(name, *domain)
}

// ValidateDomainNames validates each of domains like ValidateDomainName.
func ValidateDomainNames(name string, domains []string) error {
	for _, domain := range domains {
		if err := ValidateDomainName(name, domain); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDomainName validates domain, the value of the input called name: a
// flag such as "--domain" or an argument such as "domain".
func ValidateDomainName(name, domain string) error {
	if domain == "" {
		return errors.NewValidationError(name+" cannot be empty", nil)
	}
	if !IsDomainName(domain) {
		return InvalidValueError(name, domain, "not a domain name")
	}
	return nil
}

// IsDomainName reports whether domain is a domain name: dot-separated labels
// of letters, digits and inner hyphens, 253 characters at most.
func IsDomainName(domain string) bool {
	return len(domain) <= 253 && domainRegex.MatchString(domain)
}

// IsPrintableASCII reports whether s holds only printable ASCII: letters,
// digits, punctuation and spaces, without control or non-ASCII characters.
func IsPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// APIKeyNotPrintable is the reason an API key with a character that is not
// printable ASCII is refused: no AhaSend API key holds one, and an HTTP
// header cannot carry it, so a request with it could only fail.
const APIKeyNotPrintable = "must be printable ASCII, without control or non-ASCII characters"

// InvalidValueError is the validation error of value, the value of the input
// called name, a flag such as "--domain" or an argument such as "domain",
// with the reason it is invalid: `invalid --domain value "x": reason`.
func InvalidValueError(name, value, reason string) error {
	return errors.NewValidationError(fmt.Sprintf("invalid %s value %q: %s", name, value, reason), nil)
}

// IsLoopbackHost reports whether host, a host name or IP address without a
// port, is this machine: localhost or a loopback address such as 127.0.0.1
// or ::1. Data sent to it never leaves the machine.
func IsLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// CheckHTTPURL returns why rawURL is not an HTTP or HTTPS URL with a host and
// no user name or password, or nil; the reason ends with example, such as
// https://example.com/webhook. The CLI shows the URLs it takes, such as in
// the output of webhooks get, so a password in one would show too.
func CheckHTTPURL(rawURL, example string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("must be an HTTP or HTTPS URL with a host, such as %s", example)
	}
	if parsed.User != nil {
		return fmt.Errorf("must have no user name or password, such as %s", example)
	}
	return nil
}

// CheckAPIURL returns why rawURL cannot be the AhaSend API URL, or nil. The
// API URL passes CheckHTTPURL and has nothing after its host: the API's paths
// start at the root of the host, so a path would be lost, and the API key is
// the only credential the CLI sends.
func CheckAPIURL(rawURL string) error {
	const example = "https://api.ahasend.com"
	if err := CheckHTTPURL(rawURL, example); err != nil {
		return err
	}
	parsed, _ := url.Parse(rawURL)
	if strings.TrimSuffix(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must have no path, query or fragment, such as %s", example)
	}
	return nil
}

// maxIPAllowListEntries is the most entries an API key's IP allow list
// holds after the API removes duplicates.
const maxIPAllowListEntries = 100

// CheckIPAllowList checks the --ip-allow-list values of an API key: each is a
// CIDR block or an IP address other than an allow-all prefix, and they make
// at most 100 distinct blocks.
func CheckIPAllowList(entries []string) error {
	blocks := map[netip.Prefix]bool{}
	for _, entry := range entries {
		block, err := allowListBlock(entry)
		if err != nil {
			return InvalidValueError("--ip-allow-list", entry, "not an IP address or CIDR block")
		}
		if block.Bits() == 0 {
			return InvalidValueError("--ip-allow-list", entry, "allows every IP; leave the list empty instead")
		}
		blocks[block.Masked()] = true
	}
	if len(blocks) > maxIPAllowListEntries {
		return errors.NewValidationError(fmt.Sprintf("--ip-allow-list takes at most %d distinct entries, got %d", maxIPAllowListEntries, len(blocks)), nil)
	}
	return nil
}

// allowListBlock returns an IP allow list entry as a CIDR block: the block
// itself, or a single address as a block of one.
func allowListBlock(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		return netip.ParsePrefix(entry)
	}
	address, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(address, address.BitLen()), nil
}

// SendsInClear reports whether rawURL sends data over plain HTTP to a host
// that is not a loopback host.
func SendsInClear(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	return err == nil && parsed.Scheme == "http" && !IsLoopbackHost(parsed.Hostname())
}

// PlainHTTPWarning is the warning about rawURL, a URL that SendsInClear:
// subject names it, such as "the API URL" or "--url", and data is what it
// sends unencrypted, such as "the API key".
func PlainHTTPWarning(subject, rawURL, data string) string {
	return fmt.Sprintf("Warning: %s %s uses plain HTTP, so %s is sent unencrypted. Use HTTPS.", subject, rawURL, data)
}
