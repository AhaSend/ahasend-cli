package subaccounts

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/auth"
	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/logger"
	"github.com/AhaSend/ahasend-cli/internal/printer"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/spf13/cobra"
)

// monthlyCreditMin and monthlyCreditMax bound the optional --monthly-credit
// flag locally, mirroring the SDK's CreateSubAccountRequest.Validate() backstop
// so out-of-range values are rejected before authentication.
const (
	monthlyCreditMin = int64(0)
	monthlyCreditMax = int64(1000000000)
)

// NewCreateCommand creates the create command
func NewCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create --name <name> --website <domain>",
		Short: "Create a sub-account",
		Long: `Create a sub-account under your AhaSend parent account.

A sub-account requires a name and a website domain, such as example.com.
An http or https URL is also accepted: the command sends the website as the
API stores it, the host alone in lowercase, without the scheme, path, query
or port, and an international domain in Punycode, so
https://Example.com/about is sent as example.com and bücher.de as
xn--bcher-kva.de. You can optionally set a monthly credit allocation.
Creation is idempotent: provide your own --idempotency-key to make retries
safe, or one is generated for you.`,
		Example: `  # Create a sub-account
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com

  # Create with a monthly credit allocation
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com --monthly-credit 5000

  # Create with a custom idempotency key for safe retries
  ahasend subaccounts create --name "Acme Inc" --website acme.example.com --idempotency-key my-unique-key`,
		Args:         cmdutil.NoArgs,
		RunE:         runSubAccountsCreate,
		SilenceUsage: true,
	}

	cmd.Flags().String("name", "", "Sub-account name (required)")
	cmd.Flags().String("website", "", "Sub-account website domain, e.g. example.com (required)")
	cmd.Flags().Int64("monthly-credit", 0, "Monthly credit allocation (0-1000000000)")
	cmdutil.AddIdempotencyKeyFlag(cmd)

	return cmd
}

func runSubAccountsCreate(cmd *cobra.Command, args []string) error {
	// Get response handler instance
	handler := printer.GetResponseHandlerFromCommand(cmd)

	// The API trims the name. The name is sent trimmed, as the SDK refuses
	// one over the limit before the API trims it.
	name, err := cmdutil.ReadRequiredText(cmd, "name")
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if err := cmdutil.CheckMaxLength("name", &name, cmdutil.MaxNameLength); err != nil {
		return err
	}

	website, err := cmdutil.ReadRequiredText(cmd, "website")
	if err != nil {
		return err
	}
	if website, err = normalizeWebsite(website); err != nil {
		return err
	}

	// Build the request, distinguishing an omitted --monthly-credit from an
	// explicit zero via Changed().
	req := requests.CreateSubAccountRequest{
		Name:    name,
		Website: website,
	}

	if req.MonthlyCredit, err = readMonthlyCredit(cmd); err != nil {
		return err
	}

	// SDK backstop validation after local validation and before auth.
	if err := req.Validate(); err != nil {
		return errors.NewValidationError(err.Error(), nil)
	}

	idempotency, err := cmdutil.IdempotencyOptions(cmd)
	if err != nil {
		return err
	}

	// Only authenticate after local validation passes
	client, err := auth.GetAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	logger.Get().WithFields(map[string]interface{}{
		"name":           name,
		"website":        website,
		"monthly_credit": req.MonthlyCredit,
	}).Debug("Executing subaccounts create command")

	response, err := client.CreateSubAccount(req, idempotency...)
	if err != nil {
		return err
	}

	config := printer.SingleConfig{
		SuccessMessage: "Sub-account created successfully",
		EmptyMessage:   "No sub-account created",
	}

	return handler.HandleSingleSubAccount(response, config)
}

// maxWebsiteLength is the longest website the API takes, after it reduces
// the website to its host.
const maxWebsiteLength = 255

// websitePattern is the API's rule for a website, the fqdn rule of its
// validator: labels of letters, digits and hyphens that start with a letter
// or digit, and a last label of letters and digits that starts with a
// letter.
var websitePattern = regexp.MustCompile(`^([a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})(\.[a-zA-Z0-9]{1}[a-zA-Z0-9-]{0,62})*?(\.[a-zA-Z]{1}[a-zA-Z0-9]{0,62})\.?$`)

// normalizeWebsite returns website, which is not blank, as the API stores
// it, with websiteHost, or a validation error when the API would refuse it:
// when it has no host, or its host is not a fully qualified domain.
func normalizeWebsite(website string) (string, error) {
	website = strings.TrimSpace(website)
	invalid := func(reason string) error {
		return validation.InvalidValueError("--website", website, reason+"; use a domain such as example.com")
	}

	host, reason := websiteHost(website)
	switch {
	case reason != "":
		return "", invalid(reason)
	case !strings.Contains(host, "."):
		return "", invalid("must be a fully qualified domain")
	case len(host) > maxWebsiteLength:
		return "", invalid(fmt.Sprintf("longer than %d characters", maxWebsiteLength))
	case !websitePattern.MatchString(host):
		labels := strings.Split(host, ".")
		if tld := labels[len(labels)-1]; validation.IsDomainName(host) && (tld[0] < 'a' || tld[0] > 'z') {
			return "", invalid("must end in a valid top-level domain")
		}
		return "", invalid("not a domain name")
	}
	return host, nil
}

// websiteHost reduces website, trimmed, to its host as the API does: it
// drops an http or https scheme, a leading "//", a path, query, fragment and
// port, and a trailing dot, lowercases the host and writes an international
// one in Punycode. It returns why website has no host the API would keep
// instead, when it has none.
func websiteHost(website string) (host, reason string) {
	candidate := website
	if strings.HasPrefix(candidate, "//") {
		candidate = "https:" + candidate
	}
	host = candidate
	if strings.Contains(candidate, "://") {
		parsed, err := url.Parse(candidate)
		if err != nil {
			return "", "not a valid URL"
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return "", "only http and https URLs are accepted"
		}
		if parsed.User != nil {
			return "", "must not include credentials"
		}
		host = parsed.Host
	} else {
		// The path goes before the port, so "example.com:8443/team" reduces
		// as "https://example.com:8443/team" does
		host = strings.TrimRight(cutWebsitePath(host), ".")
		if colon := strings.Index(host, ":"); colon >= 0 && !isPort(host[colon+1:]) {
			return "", "not a domain or an http or https URL"
		}
	}

	host = cutWebsitePath(host)
	if strings.Contains(host, "@") {
		return "", "must not include credentials"
	}
	if colon := strings.LastIndex(host, ":"); colon >= 0 && !strings.Contains(host, "]") {
		host = host[:colon]
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return "", "not a domain name"
	}
	return validation.ASCIIDomain(host), ""
}

// cutWebsitePath returns value without its path, query and fragment: what
// follows the first "/", "?" or "#".
func cutWebsitePath(value string) string {
	if i := strings.IndexAny(value, "/?#"); i >= 0 {
		return value[:i]
	}
	return value
}

// isPort reports whether s is a port: one or more digits.
func isPort(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// readMonthlyCredit returns --monthly-credit, or nil when the flag is not set,
// so an explicit 0 is sent. A value outside its range is a validation error.
func readMonthlyCredit(cmd *cobra.Command) (*int64, error) {
	if !cmd.Flags().Changed("monthly-credit") {
		return nil, nil
	}
	credit, _ := cmd.Flags().GetInt64("monthly-credit")
	if credit < monthlyCreditMin || credit > monthlyCreditMax {
		return nil, errors.NewValidationError(fmt.Sprintf("--monthly-credit must be between %d and %d", monthlyCreditMin, monthlyCreditMax), nil)
	}
	return &credit, nil
}
