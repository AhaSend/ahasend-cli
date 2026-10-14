package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-cli/internal/validation"
)

// localValidationErrors are mistakes a command can catch alone, so it rejects
// them as validation errors before it authenticates or prompts.
var localValidationErrors = []struct {
	args    []string
	message string
}{
	{[]string{"suppressions", "create", "not-an-email", "--expires", "30d"},
		`invalid email value "not-an-email": not an email address`},
	{[]string{"suppressions", "create", "user@example.com", "--expires", "30d", "--domain", "not a domain"},
		`invalid --domain value "not a domain": not a domain name`},
	{[]string{"domains", "create", "not a domain"}, `invalid domain value "not a domain": not a domain name`},
	{[]string{"domains", "get", ".."}, `invalid domain value "..": not a domain`},
	// A blank argument would address the collection, or nothing
	{[]string{"domains", "get", "   "}, "domain cannot be empty"},
	{[]string{"contacts", "delete", " ", "--force"}, "contact ID or email cannot be empty"},
	{[]string{"lists", "members", "remove", rawTestID, " "}, "contact ID or email cannot be empty"},
	{[]string{"subaccounts", "create", "--name", "Acme", "--website", "acme"},
		`invalid --website value "acme": must be a fully qualified domain; use a domain such as example.com`},
	{[]string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:{not a domain}"},
		`invalid --scope value "messages:send:{not a domain}": "not a domain" is not a domain name`},
	{[]string{"suppressions", "check", "user@example.com", "--domain", "not a domain"}, `invalid --domain value "not a domain": not a domain name`},
	{[]string{"suppressions", "list", "--domain", "not a domain"}, `invalid --domain value "not a domain": not a domain name`},
	{[]string{"suppressions", "delete", "user@example.com", "--domain", "not a domain"}, `invalid --domain value "not a domain": not a domain name`},
	{[]string{"suppressions", "wipe", "--domain", "not a domain"}, `invalid --domain value "not a domain": not a domain name`},
	{[]string{"apikeys", "update", rawTestID, "--label", ""}, "--label cannot be empty"},
	{[]string{"apikeys", "update", rawTestID, "--scope", ""}, "--scope cannot be empty"},
	{[]string{"subaccounts", "api-keys", "update", rawTestID, rawTestID, "--label", ""}, "--label cannot be empty"},
	{[]string{"subaccounts", "api-keys", "update", rawTestID, rawTestID, "--scope", ""}, "--scope cannot be empty"},
	{[]string{"subaccounts", "update", rawTestID, "--website", ""}, "--website cannot be empty"},
	{[]string{"webhooks", "get", "abc"}, `invalid webhook ID value "abc": not a UUID`},
	{[]string{"messages", "list", "--sender", "not-an-email"}, `invalid --sender value "not-an-email": not an email address`},
	{[]string{"messages", "list", "--tag", "t1,t2,t3,t4,t5,t6,t7,t8,t9,t10,t11,t12,t13,t14,t15,t16,t17,t18,t19,t20,t21"},
		"--tag takes at most 20 tags, got 21"},
	// The API splits filter tags at commas
	{[]string{"messages", "list", "--tag", `"news,weekly"`},
		`invalid --tag value "news,weekly": a filter tag cannot hold a comma, at which the API splits the tags`},
	{[]string{"stats", "deliverability", "--tag", `"news, weekly"`},
		`invalid --tag value "news, weekly": a filter tag cannot hold a comma, at which the API splits the tags`},
	{[]string{"stats", "bounces", "--tag", `a,"b,c"`},
		`invalid --tag value "b,c": a filter tag cannot hold a comma, at which the API splits the tags`},
	{[]string{"stats", "delivery-time", "--tag", `"x,y"`},
		`invalid --tag value "x,y": a filter tag cannot hold a comma, at which the API splits the tags`},
	{[]string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:all", "--ip-allow-list", "203.0.113.0/33"},
		`invalid --ip-allow-list value "203.0.113.0/33": not an IP address or CIDR block`},
	{[]string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:all", "--ip-allow-list", "example.com"},
		`invalid --ip-allow-list value "example.com": not an IP address or CIDR block`},
	{[]string{"apikeys", "update", rawTestID, "--ip-allow-list", "::/0"},
		`invalid --ip-allow-list value "::/0": allows every IP; leave the list empty instead`},
	{[]string{"subaccounts", "api-keys", "update", rawTestID, rawTestID, "--ip-allow-list", "0.0.0.0/0"},
		`invalid --ip-allow-list value "0.0.0.0/0": allows every IP; leave the list empty instead`},
	{[]string{"subaccounts", "api-keys", "create", rawTestID, "--label", "ci", "--scope", "messages:send:all", "--ip-allow-list", "300.1.1.1"},
		`invalid --ip-allow-list value "300.1.1.1": not an IP address or CIDR block`},
	// A global webhook or credential ignores domains, so they need --scope scoped.
	{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--domains", "example.com"},
		"--domains needs --scope scoped"},
	{[]string{"smtp", "create", "--name", "App", "--domains", "example.com"}, "--domains needs --scope scoped"},
	{[]string{"webhooks", "update", rawTestID, "--scope", "global", "--domains", "example.com"},
		"--domains needs --scope scoped"},
	// A scoped webhook with no domains receives no events.
	{[]string{"webhooks", "update", rawTestID, "--domains", ""},
		"--domains cannot be empty; use --scope global to remove the domain restriction"},
	{[]string{"smtp", "create", "--name", "App", "--scope", "scoped", "--domains", "not a domain"},
		`invalid --domains value "not a domain": not a domain name`},
	{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--scope", "scoped", "--domains", "example.com,not a domain"},
		`invalid --domains value "not a domain": not a domain name`},
	{[]string{"webhooks", "update", rawTestID, "--domains", "not a domain"}, `invalid --domains value "not a domain": not a domain name`},
	{[]string{"stats", "deliverability", "--sender-domain", "not a domain"}, `invalid --sender-domain value "not a domain": not a domain name`},
	{[]string{"stats", "bounces", "--recipient-domain", "gmail.com", "--recipient-domain", "gmail..com"},
		`invalid --recipient-domain value "gmail..com": not a domain name`},
	{[]string{"suppressions", "create", "user@example.com", "--expires", "2020-01-01T00:00:00Z"},
		`invalid --expires value "2020-01-01T00:00:00Z": the time is in the past`},
	{append(sendArgs("--text", "Hi"), "--schedule", "2020-01-01T00:00:00Z"),
		`invalid --schedule value "2020-01-01T00:00:00Z": the time is in the past`},
	{[]string{"routes", "trigger", "abc"}, `invalid route ID value "abc": not a UUID`},
	{[]string{"smtp", "get", "abc"}, `invalid credential ID value "abc": not a UUID`},
	{[]string{"messages", "cancel", "abc", "--force"}, `invalid message ID value "abc": not a UUID or <uuid@domain>`},
	{[]string{"domains", "update", "example.com", "--dkim-selector", "not_a_label"},
		`invalid --dkim-selector value "not_a_label": must be one DNS label of 1-62 letters, digits and hyphens, without a leading or trailing hyphen`},
	{[]string{"stats", "bounces", "--from-time", "soon"},
		`invalid --from-time value "soon": use RFC 3339 or a relative time such as 24h or 7d`},
	{[]string{"stats", "deliverability", "--to-time", "30d"}, "--from-time must be before --to-time"},
	{[]string{"messages", "list", "--from-time", "1d", "--to-time", "7d"}, "--from-time must be before --to-time"},
	{[]string{"contacts", "list", "--from-time", "2026-02-01T00:00:00Z", "--to-time", "2026-01-01T00:00:00Z"},
		"--from-time must be before --to-time"},
	// A flag set to false still conflicts with the flag it cannot go with.
	{[]string{"apikeys", "update", rawTestID, "--clear-ip-allow-list=false", "--ip-allow-list", "192.0.2.1"},
		"--clear-ip-allow-list and --ip-allow-list cannot be used together"},
	{append(sendArgs("--text", "Hi"), "--text-file", "body.txt"), "--text and --text-file cannot be used together"},
	{append(sendArgs("--html", "<p>Hi</p>"), "--html-file", "body.html"), "--html and --html-file cannot be used together"},
	{append(sendArgs("--amp", "<p>Hi</p>"), "--amp-file", "body.amp"), "--amp and --amp-file cannot be used together"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--text", "Hi", "--text-file", "body.txt"},
		"--text and --text-file cannot be used together"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--html", "<p>Hi</p>", "--html-file", "body.html"},
		"--html and --html-file cannot be used together"},
	// A header value with an unquoted comma is split into a bogus header
	{append(sendArgs("--text", "Hi"), "--header", "List-Unsubscribe: <mailto:u@example.com>, <https://example.com/u>"),
		`invalid --header value " <https://example.com/u>": " <https" is not a header name, which is visible ASCII without spaces; ` +
			`a header value with a comma must be in double quotes: --header '"Name: a, b"'`},
	{append(sendArgs("--text", "Hi"), "--header", `"X-Test: a`),
		`invalid --header value "\"X-Test: a": a value with a comma or a double quote must be in double quotes, ` +
			`with each double quote in it doubled, such as '"a, ""b"""'`},
	// Limits the API checks: each scope once, a subdomain label, and send
	// times
	{[]string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:all", "--scope", "messages:send:all"},
		`invalid --scope value "messages:send:all": given more than once`},
	{[]string{"apikeys", "update", rawTestID, "--scope", "messages:send:{Example.com}", "--scope", "messages:send:{example.com}"},
		`invalid --scope value "messages:send:{example.com}": given more than once`},
	{[]string{"subaccounts", "api-keys", "create", rawTestID, "--label", "ci", "--scope", "messages:send:all,messages:send:all"},
		`invalid --scope value "messages:send:all": given more than once`},
	// The API drops a trailing dot of a scope's domain
	{[]string{"apikeys", "create", "--label", "ci", "--scope", "messages:send:{example.com.}", "--scope", "messages:send:{example.com}"},
		`invalid --scope value "messages:send:{example.com}": given more than once`},
	// The API refuses the scopes that manage sub-accounts to a sub-account's key
	{[]string{"subaccounts", "api-keys", "create", rawTestID, "--label", "ci", "--scope", "sub-accounts:read"},
		`invalid --scope value "sub-accounts:read": a sub-account's API key cannot have it`},
	{[]string{"subaccounts", "api-keys", "update", rawTestID, rawTestID, "--scope", "sub-account-api-keys:write"},
		`invalid --scope value "sub-account-api-keys:write": a sub-account's API key cannot have it`},
	{[]string{"domains", "create", "example.com", "--tracking-subdomain", "track_me"},
		`invalid --tracking-subdomain value "track_me": must be one DNS label of 1-63 letters, digits and hyphens, without a leading or trailing hyphen`},
	{[]string{"domains", "update", "example.com", "--media-subdomain", "media.cdn"},
		`invalid --media-subdomain value "media.cdn": must be one DNS label of 1-63 letters, digits and hyphens, without a leading or trailing hyphen`},
	{append(sendArgs("--text", "Hi"), "--schedule", "8d"), `invalid --schedule value "8d": the API takes a time at most 7 days ahead`},
	// Without --sandbox, a sandbox result would be ignored and real mail sent.
	{append(sendArgs("--text", "Hi"), "--sandbox-result", "bounce"), "--sandbox-result needs --sandbox"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--sandbox-result", "bounce"}, "--sandbox-result needs --sandbox"},
}

// sendArgs returns the arguments of a messages send with every required flag
// but the content, followed by content.
func sendArgs(content ...string) []string {
	return append([]string{"messages", "send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "Hi"}, content...)
}

// enumValidationErrors are values outside an enum flag's values, which every
// command rejects with one message before it authenticates.
var enumValidationErrors = []struct {
	args    []string
	message string
}{
	{[]string{"domains", "list", "--sending-type", "bulk"}, "--sending-type must be one of: transactional, marketing"},
	{[]string{"domains", "list", "--status", "active"}, "--status must be one of: verified, pending, failed"},
	{[]string{"domains", "create", "example.com", "--sending-type", "bulk"}, "--sending-type must be one of: transactional, marketing"},
	{[]string{"domains", "create", "example.com", "--format", "zonefile"}, "--format must be one of: bind, cloudflare, terraform"},
	{[]string{"domains", "update", "example.com", "--sending-type", "bulk"}, "--sending-type must be one of: transactional, marketing"},
	{[]string{"contacts", "create", "a@example.com", "--status", "active"}, "--status must be one of: enabled, disabled, blocked"},
	{[]string{"contacts", "update", "a@example.com", "--status", "active"}, "--status must be one of: enabled, disabled, blocked"},
	{[]string{"contacts", "list", "--status", "active"}, "--status must be one of: enabled, disabled, blocked"},
	{[]string{"contacts", "memberships", "a@example.com", "--subscription-status", "bounced"},
		"--subscription-status must be one of: unconfirmed, confirmed, unsubscribed, complained"},
	{[]string{"lists", "members", "list", rawTestID, "--subscription-status", "bounced"},
		"--subscription-status must be one of: unconfirmed, confirmed, unsubscribed, complained"},
	{[]string{"lists", "members", "set", rawTestID, "a@example.com", "--subscription-status", "complained"},
		"--subscription-status must be one of: confirmed, unconfirmed, unsubscribed"},
	{[]string{"messages", "list", "--status", "lost"},
		"--status must be one of: received, delivered, opened, clicked, deferred, bounced, failed, suppressed, " +
			"sandbox delivered, sandbox deferred, sandbox failed, sandbox bounced, sandbox suppressed"},
	{[]string{"stats", "bounces", "--group-by", "year"}, "--group-by must be one of: hour, day, week, month"},
	{[]string{"webhooks", "create", "--name", "Hook", "--url", "https://example.com", "--scope", "all"},
		"--scope must be one of: global, scoped"},
	{[]string{"webhooks", "update", rawTestID, "--scope", "all"}, "--scope must be one of: global, scoped"},
	{[]string{"smtp", "create", "--name", "App", "--scope", "all"}, "--scope must be one of: global, scoped"},
	{[]string{"messages", "send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "Hi", "--text", "Hi",
		"--sandbox", "--sandbox-result", "explode"},
		"--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--sandbox", "--sandbox-result", "explode"},
		"--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
	// A bad --sandbox-result is an error without --sandbox too.
	{append(sendArgs("--text", "Hi"), "--sandbox-result", "explode"),
		"--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--sandbox-result", "explode"},
		"--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
	{[]string{"smtp", "send", "--server", "127.0.0.1:1", "--test", "--sandbox-result", "explode"},
		"--sandbox-result must be one of: deliver, bounce, defer, fail, suppress"},
}

// TestEveryLimitIsCheckedBeforeAuthenticating runs every command that has
// --limit with a limit above its range and below it.
func TestEveryLimitIsCheckedBeforeAuthenticating(t *testing.T) {
	checked := 0
	for _, leaf := range leafCommands() {
		if leaf.Flags().Lookup("limit") == nil {
			continue
		}
		checked++
		path := strings.Fields(strings.TrimPrefix(leaf.CommandPath(), "ahasend "))
		args := slices.Concat(path, slices.Repeat([]string{rawTestID}, len(cmdutil.UsageArguments(leaf))))
		for _, limit := range []string{"101", "-1"} {
			t.Run(strings.Join(path, " ")+" "+limit, func(t *testing.T) {
				called := testutil.RefuseAuth(t)

				stdout, stderr, code := runRoot(t, slices.Concat(args, []string{"--limit", limit})...)

				assert.Equal(t, 4, code, "stderr: %s", stderr)
				assert.Empty(t, stdout)
				assert.Equal(t, "Error: --limit must be between 0 and 100\n", stderr)
				assert.False(t, *called, "the command must not authenticate")
			})
		}
	}
	assert.Greater(t, checked, 10, "the list commands have --limit")
}

func TestEnumValidationErrorsExitFourBeforeAuthenticating(t *testing.T) {
	for _, tt := range enumValidationErrors {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, tt.args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+tt.message+"\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

func TestLocalValidationErrorsExitFourBeforeAuthenticating(t *testing.T) {
	for _, tt := range localValidationErrors {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, tt.args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+tt.message+"\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// idempotentCommands are valid runs of every command that has
// --idempotency-key, which the key's length alone can fail.
var idempotentCommands = map[string][]string{
	"apikeys create":              {"--label", "ci", "--scope", "messages:send:all"},
	"contacts batch":              {"--file", "contacts.json"},
	"contacts create":             {"user@example.com"},
	"domains create":              {"example.com"},
	"lists create":                {"--name", "Newsletter"},
	"lists members add":           {rawTestID, "--email", "user@example.com"},
	"messages send":               {"--from", "a@example.com", "--to", "b@example.com", "--subject", "Hi", "--text", "Hi"},
	"routes create":               {"--name", "Inbound", "--url", "https://example.com/hook", "--recipient", "*@example.com"},
	"smtp create":                 {"--name", "App"},
	"subaccounts api-keys create": {rawTestID, "--label", "ci", "--scope", "messages:send:all"},
	"subaccounts create":          {"--name", "Acme", "--website", "acme.example.com"},
	"suppressions create":         {"user@example.com", "--expires", "30d"},
	"webhooks create":             {"--name", "Events", "--url", "https://example.com/hook", "--events", "delivered"},
}

// badIdempotencyKeys are keys every command refuses, with the error of each:
// one longer than the API accepts, and ones an HTTP header cannot carry as
// they are.
var badIdempotencyKeys = map[string]string{
	strings.Repeat("k", 256): "--idempotency-key must be at most 255 characters",
	"order\x017": "--idempotency-key must be printable ASCII: letters, digits, punctuation and spaces, " +
		"without control or non-ASCII characters",
	"commande-é": "--idempotency-key must be printable ASCII: letters, digits, punctuation and spaces, " +
		"without control or non-ASCII characters",
}

// TestEveryIdempotencyKeyIsCheckedBeforeAuthenticating runs every command
// that has --idempotency-key with each of badIdempotencyKeys.
func TestEveryIdempotencyKeyIsCheckedBeforeAuthenticating(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("contacts.json", []byte(`{"data":[{"email":"user@example.com"}]}`), 0o600))
	for _, leaf := range leafCommands() {
		if leaf.Flags().Lookup("idempotency-key") == nil {
			continue
		}
		name := strings.TrimPrefix(leaf.CommandPath(), "ahasend ")
		for key, message := range badIdempotencyKeys {
			t.Run(name+" "+message, func(t *testing.T) {
				args, ok := idempotentCommands[name]
				require.True(t, ok, "add a valid run of %s to idempotentCommands", name)
				called := testutil.RefuseAuth(t)

				stdout, stderr, code := runRoot(t, slices.Concat(strings.Fields(name), args, []string{"--idempotency-key", key})...)

				assert.Equal(t, 4, code, "stderr: %s", stderr)
				assert.Empty(t, stdout)
				assert.Equal(t, "Error: "+message+"\n", stderr)
				assert.False(t, *called, "the command must not authenticate")
			})
		}
	}
}

// TestEveryScopeCommandListsEveryScope checks that the help of every command
// that takes --scope lists every scope it accepts, and that a bad scope's
// error points to that help.
func TestEveryScopeCommandListsEveryScope(t *testing.T) {
	checked := 0
	for _, leaf := range leafCommands() {
		if leaf.Flags().Lookup("scope") == nil || leaf.Flags().Lookup("label") == nil {
			continue
		}
		checked++
		path := strings.TrimPrefix(leaf.CommandPath(), "ahasend ")
		t.Run(path, func(t *testing.T) {
			// A sub-account's key cannot have the scopes that manage
			// sub-accounts, so its commands do not list them.
			subAccountKey := strings.HasPrefix(path, "subaccounts ")
			for _, group := range validation.ScopeGroups {
				for _, scope := range group.Scopes {
					if subAccountKey && validation.IsParentOnlyScope(scope) {
						assert.NotContains(t, leaf.Long, scope)
						continue
					}
					assert.Contains(t, leaf.Long, scope)
				}
			}

			args := slices.Concat(strings.Fields(path), slices.Repeat([]string{rawTestID}, len(cmdutil.UsageArguments(leaf))),
				[]string{"--label", "ci", "--scope", "messages:send"})
			_, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code)
			assert.Equal(t, `Error: invalid --scope value "messages:send": not an API key scope; `+
				"`"+leaf.CommandPath()+" --help` lists the scopes\n", stderr)
		})
	}
	assert.Equal(t, 4, checked, "apikeys and subaccounts api-keys create and update take --scope")
}

// TestEveryNameAndLabelLengthIsCheckedBeforeAuthenticating runs every command
// that sends a name or a label the API limits to 255 characters with a
// longer one.
func TestEveryNameAndLabelLengthIsCheckedBeforeAuthenticating(t *testing.T) {
	long := strings.Repeat("é", 256)
	tests := [][]string{
		{"apikeys", "create", "--scope", "messages:send:all", "--label", long},
		{"apikeys", "update", rawTestID, "--label", long},
		{"subaccounts", "api-keys", "create", rawTestID, "--scope", "messages:send:all", "--label", long},
		{"subaccounts", "api-keys", "update", rawTestID, rawTestID, "--label", long},
		{"subaccounts", "create", "--website", "acme.example.com", "--name", long},
		{"subaccounts", "update", rawTestID, "--name", long},
		{"webhooks", "create", "--url", "https://example.com/hook", "--events", "delivered", "--name", long},
		{"webhooks", "update", rawTestID, "--name", long},
		{"routes", "create", "--url", "https://example.com/hook", "--recipient", "*@example.com", "--name", long},
		{"routes", "update", rawTestID, "--name", long},
		{"smtp", "create", "--name", long},
		{"lists", "create", "--name", long},
		{"lists", "update", rawTestID, "--name", long},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[:len(args)-1], " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+args[len(args)-2]+" must be at most 255 characters\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestLengthsAreCountedAsTheAPICountsThem runs every command that sends a
// name or a description the API cleans before it counts it, with a value at
// the limit once cleaned and longer as given: the spaces around a name or a
// description do not count. The command sends the value.
func TestLengthsAreCountedAsTheAPICountsThem(t *testing.T) {
	name := "  " + strings.Repeat("é", 255) + "\t\n"
	description := " " + strings.Repeat("é", 1000) + " "
	tests := [][]string{
		{"subaccounts", "create", "--website", "acme.example.com", "--name", name},
		{"subaccounts", "update", rawTestID, "--name", name},
		{"lists", "create", "--name", name},
		{"lists", "update", rawTestID, "--name", name},
		{"lists", "create", "--name", "News", "--description", description},
		{"lists", "update", rawTestID, "--description", description},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[:len(args)-1], " "), func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			for _, method := range []string{"CreateSubAccount", "UpdateSubAccount", "CreateList", "UpdateList"} {
				expectAnyArgs(mockClient, method, nil, rawAPIError(400))
			}
			called := testutil.InjectClient(t, mockClient)

			_, stderr, _ := runRoot(t, args...)

			assert.NotContains(t, stderr, "must be at most")
			assert.True(t, *called, "the command must send the value")
		})
	}
}

// TestEveryTextLengthIsCheckedBeforeAuthenticating runs every command that
// sends a text field the API limits, other than a name or a label, with one
// a character too long.
func TestEveryTextLengthIsCheckedBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		args []string
		max  int
	}{
		{[]string{"lists", "create", "--name", "News", "--description"}, 1000},
		{[]string{"lists", "update", rawTestID, "--description"}, 1000},
		{[]string{"suppressions", "create", "a@example.com", "--expires", "30d", "--reason"}, 255},
		{[]string{"subaccounts", "suspend", rawTestID, "--force", "--reason"}, 500},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)
			flag := test.args[len(test.args)-1]

			stdout, stderr, code := runRoot(t, append(test.args, strings.Repeat("é", test.max+1))...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, fmt.Sprintf("Error: %s must be at most %d characters\n", flag, test.max), stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestEveryBlankTextValueIsAnEmptyValue runs every command with a text flag
// that cannot be empty, or argument, given only white space. Each fails
// with the empty-value error before authenticating, and never sends the
// value.
func TestEveryBlankTextValueIsAnEmptyValue(t *testing.T) {
	const blank = " \t "
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"apikeys", "create", "--scope", "messages:send:all", "--label", blank}, "--label cannot be empty"},
		{[]string{"apikeys", "update", rawTestID, "--label", blank}, "--label cannot be empty"},
		{[]string{"subaccounts", "api-keys", "create", rawTestID, "--scope", "messages:send:all", "--label", blank}, "--label cannot be empty"},
		{[]string{"lists", "create", "--name", blank}, "--name cannot be empty"},
		{[]string{"lists", "update", rawTestID, "--name", blank}, "--name cannot be empty"},
		{[]string{"webhooks", "create", "--url", "https://example.com/hook", "--name", blank}, "--name cannot be empty"},
		{[]string{"webhooks", "create", "--name", "Hook", "--url", blank}, "--url cannot be empty"},
		{[]string{"webhooks", "update", rawTestID, "--name", blank}, "--name cannot be empty"},
		{[]string{"routes", "create", "--url", "https://example.com/hook", "--recipient", "*@example.com", "--name", blank}, "--name cannot be empty"},
		{[]string{"routes", "update", rawTestID, "--name", blank}, "--name cannot be empty"},
		{[]string{"routes", "update", rawTestID, "--recipient", blank}, "--recipient cannot be empty: a route's recipient cannot be cleared"},
		{[]string{"smtp", "create", "--name", blank}, "--name cannot be empty"},
		{[]string{"subaccounts", "create", "--website", "acme.example.com", "--name", blank}, "--name cannot be empty"},
		{[]string{"subaccounts", "create", "--name", "Acme", "--website", blank}, "--website cannot be empty"},
		{[]string{"subaccounts", "update", rawTestID, "--website", blank}, "--website cannot be empty"},
		{[]string{"subaccounts", "suspend", rawTestID, "--force", "--reason", blank}, "--reason cannot be empty"},
		{[]string{"contacts", "create", blank}, "email cannot be empty"},
		{[]string{"smtp", "send", "--server", blank, "--test"}, "--server cannot be empty"},
		{append(sendArgs("--text", "Hi"), "--subject", blank), "--subject cannot be empty"},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, test.args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+test.message+"\n", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestEveryEmailAddressIsCheckedBeforeAuthenticating runs every command that
// sends an email address the API checks, or filters by one the API matches
// exactly, with a value that is not a bare address.
func TestEveryEmailAddressIsCheckedBeforeAuthenticating(t *testing.T) {
	const notBare = "Jane <jane@example.com>"
	tests := [][]string{
		{"contacts", "update", "a@example.com", "--email", notBare},
		{"contacts", "list", "--email", notBare},
		{"lists", "members", "list", rawTestID, "--email", notBare},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code, "stderr: %s", stderr)
			assert.Empty(t, stdout)
			assert.Equal(t, fmt.Sprintf("Error: invalid %s value %q: not an email address\n", args[len(args)-2], notBare), stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestEveryEventURLIsCheckedLikeTheAPIURL runs every command that takes a
// URL to send events to with a URL without a host name, and with one with a
// password, which the commands that show the webhook or route would show.
func TestEveryEventURLIsCheckedLikeTheAPIURL(t *testing.T) {
	commands := [][]string{
		{"webhooks", "create", "--name", "Hook", "--scope", "global", "--all-events", "--url"},
		{"webhooks", "update", rawTestID, "--url"},
		{"routes", "create", "--name", "Inbound", "--recipient", "*@example.com", "--url"},
		{"routes", "update", rawTestID, "--url"},
		{"webhooks", "listen", "--forward-to"},
	}
	values := map[string]string{
		"http://:8080/hook":                    "must be an HTTP or HTTPS URL with a host, such as https://example.com/webhook",
		"https://user:secret@example.com/hook": "must have no user name or password, such as https://example.com/webhook",
	}
	for _, args := range commands {
		for value, reason := range values {
			t.Run(strings.Join(args, " ")+" "+value, func(t *testing.T) {
				called := testutil.RefuseAuth(t)

				stdout, stderr, code := runRoot(t, append(args, value)...)

				assert.Equal(t, 4, code, "stderr: %s", stderr)
				assert.Empty(t, stdout)
				assert.Equal(t, "Error: "+args[len(args)-1]+" "+reason+"\n", stderr)
				assert.False(t, *called, "the command must not authenticate")
			})
		}
	}
}
