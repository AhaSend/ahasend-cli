package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// complete runs Cobra's hidden __complete command for args, the last one
// the word being completed, and returns the completions and the directive
// line, such as ":4".
func complete(t *testing.T, args ...string) ([]string, string) {
	t.Helper()
	stdout, _, code := runRoot(t, append([]string{"__complete"}, args...)...)
	require.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.NotEmpty(t, lines)
	directive := lines[len(lines)-1]
	var completions []string
	for _, line := range lines[:len(lines)-1] {
		// A subcommand's completion carries its description after a tab
		name, _, _ := strings.Cut(line, "\t")
		completions = append(completions, name)
	}
	return completions, directive
}

// noFiles is the directive that tells the shell to complete no file names.
const noFiles = ":4"

// TestFixedValueFlagsCompleteTheValuesTheyAccept checks that every flag that
// takes one of a fixed set of values completes those values, and no file
// names.
func TestFixedValueFlagsCompleteTheValuesTheyAccept(t *testing.T) {
	statuses := []string{"received", "delivered", "opened", "clicked", "deferred", "bounced", "failed", "suppressed",
		"sandbox delivered", "sandbox deferred", "sandbox failed", "sandbox bounced", "sandbox suppressed"}
	sandboxResults := []string{"deliver", "bounce", "defer", "fail", "suppress"}
	scopes := []string{"global", "scoped"}
	sendingTypes := []string{"transactional", "marketing"}
	contactStatuses := []string{"enabled", "disabled", "blocked"}
	filters := []string{"unconfirmed", "confirmed", "unsubscribed", "complained"}
	events := []string{"reception", "delivered", "transient_error", "failed", "bounced", "suppressed", "opened",
		"clicked", "suppression_created", "dns_error", "message.reception", "message.delivered",
		"message.transient_error", "message.failed", "message.bounced", "message.suppressed", "message.opened",
		"message.clicked", "suppression.created", "domain.dns_error"}
	tests := []struct {
		args []string
		want []string
	}{
		{[]string{"--output"}, []string{"json", "table", "plain", "csv"}},
		{[]string{"domains", "list", "--status"}, []string{"verified", "pending", "failed"}},
		{[]string{"domains", "list", "--sending-type"}, sendingTypes},
		{[]string{"domains", "create", "example.com", "--sending-type"}, sendingTypes},
		{[]string{"domains", "create", "example.com", "--format"}, []string{"bind", "cloudflare", "terraform"}},
		{[]string{"domains", "update", "example.com", "--sending-type"}, sendingTypes},
		{[]string{"messages", "list", "--status"}, statuses},
		{[]string{"messages", "send", "--sandbox-result"}, sandboxResults},
		{[]string{"smtp", "send", "--sandbox-result"}, sandboxResults},
		{[]string{"smtp", "create", "--scope"}, scopes},
		{[]string{"webhooks", "create", "--scope"}, scopes},
		{[]string{"webhooks", "update", rawTestID, "--scope"}, scopes},
		{[]string{"webhooks", "create", "--events"}, events},
		{[]string{"webhooks", "update", rawTestID, "--events"}, events},
		{[]string{"webhooks", "trigger", rawTestID, "--events"}, events},
		{[]string{"webhooks", "listen", "--events"}, events},
		{[]string{"contacts", "create", "a@example.com", "--status"}, contactStatuses},
		{[]string{"contacts", "update", "a@example.com", "--status"}, contactStatuses},
		{[]string{"contacts", "list", "--status"}, contactStatuses},
		{[]string{"contacts", "memberships", "a@example.com", "--subscription-status"}, filters},
		{[]string{"lists", "members", "list", rawTestID, "--subscription-status"}, filters},
		{[]string{"lists", "members", "set", rawTestID, "a@example.com", "--subscription-status"},
			[]string{"confirmed", "unconfirmed", "unsubscribed"}},
		{[]string{"stats", "bounces", "--group-by"}, []string{"hour", "day", "week", "month"}},
	}
	for _, test := range tests {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			completions, directive := complete(t, append(test.args, "")...)

			assert.Equal(t, test.want, completions)
			assert.Equal(t, noFiles, directive)
		})
	}
}

// TestScopeCompletesTheScopesOfTheKeysOwner checks that --scope of the API
// key commands completes the scopes a key can have, without the parent-only
// ones for a sub-account's key.
func TestScopeCompletesTheScopesOfTheKeysOwner(t *testing.T) {
	for _, args := range [][]string{{"apikeys", "create"}, {"apikeys", "update", rawTestID}} {
		completions, directive := complete(t, append(args, "--scope", "")...)
		assert.Contains(t, completions, "messages:send:all")
		assert.Contains(t, completions, "sub-accounts:read")
		assert.Equal(t, noFiles, directive)
	}
	for _, args := range [][]string{{"subaccounts", "api-keys", "create", rawTestID}, {"subaccounts", "api-keys", "update", rawTestID, rawTestID}} {
		completions, _ := complete(t, append(args, "--scope", "")...)
		assert.Contains(t, completions, "messages:send:{domain}")
		assert.NotContains(t, completions, "sub-accounts:read")
	}
}

// TestProfileNamesCompleteFromTheConfigurationFile checks that --profile and
// the profile argument of auth switch and auth logout complete the names of
// the profiles in the configuration file, without a request.
func TestProfileNamesCompleteFromTheConfigurationFile(t *testing.T) {
	testutil.WriteProfiles(t, testutil.UseNewHome(t), "http://127.0.0.1:1", rawTestID, "staging", "Prod")

	for _, args := range [][]string{
		{"domains", "list", "--profile", ""},
		{"auth", "status", "--profile", ""},
		{"auth", "login", "--profile", ""},
		{"auth", "switch", ""},
		{"auth", "logout", ""},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			completions, directive := complete(t, args...)

			assert.Equal(t, []string{"Prod", "staging"}, completions)
			assert.Equal(t, noFiles, directive)
		})
	}

	completions, _ := complete(t, "auth", "switch", "Prod", "")
	assert.Empty(t, completions, "auth switch takes one profile")
}

// TestProfileNamesCompleteNothingWithoutAConfigurationFile checks that
// without a configuration file the profile completions are empty, with no
// error and without creating the CLI's directory.
func TestProfileNamesCompleteNothingWithoutAConfigurationFile(t *testing.T) {
	home := testutil.UseNewHome(t)

	stdout, stderr, code := runRoot(t, "__complete", "auth", "switch", "")

	assert.Equal(t, 0, code)
	assert.Equal(t, noFiles+"\n", stdout)
	assert.Equal(t, "Completion ended with directive: ShellCompDirectiveNoFileComp\n", stderr)
	assert.NoDirExists(t, filepath.Join(home, ".ahasend"))
}

// TestOnlyFileFlagsCompleteFileNames checks that a flag that takes a file
// path, under its name or a deprecated one, completes file names, and that
// other flags, and the arguments of commands, which are never files, do not.
func TestOnlyFileFlagsCompleteFileNames(t *testing.T) {
	for _, args := range [][]string{
		{"messages", "send", "--recipients"},
		{"messages", "send", "--attach"},
		{"messages", "send", "--text-template"},
		{"smtp", "send", "--html-file"},
		{"contacts", "batch", "--file"},
		{"domains", "create", "example.com", "--dkim-private-key-file"},
	} {
		_, directive := complete(t, append(args, "")...)
		assert.Equal(t, ":0", directive, "%v completes file names", args)
	}
	for _, args := range [][]string{
		{"messages", "send", "--subject"},
		{"domains", "list", "--limit"},
		{"--api-key"},
		{"domains", "list"},
		{"domains", "get"},
	} {
		_, directive := complete(t, append(args, "")...)
		assert.Equal(t, noFiles, directive, "%v completes no file names", args)
	}
}
