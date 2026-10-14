package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandLineErrors are mistakes that Cobra itself, or a flag conflict check,
// catches before the command authenticates.
var commandLineErrors = []struct {
	name    string
	args    []string
	message string
}{
	{"unknown flag", []string{"apikeys", "list", "--bogus"}, "unknown flag: --bogus"},
	{"bad flag value", []string{"domains", "list", "--limit", "many"}, `invalid argument "many" for "--limit"`},
	{"missing argument", []string{"apikeys", "get"}, "the key-id argument is required"},
	{"extra argument", []string{"apikeys", "list", "extra"}, `unexpected argument "extra"`},
	{"unknown command", []string{"bogus"}, `unknown command "bogus" for "ahasend"`},
	{"unknown help topic", []string{"help", "bogus"}, `unknown command "bogus" for "ahasend"`},
	{"unknown help subtopic", []string{"help", "domains", "bogus"}, `unknown command "bogus" for "ahasend domains"`},
	{"unknown completion shell", []string{"completion", "bogus"}, `unknown command "bogus" for "ahasend completion"`},
	{"flag conflict", []string{"webhooks", "update", rawTestID, "--events", "delivered", "--clear-events"},
		"--events and --clear-events cannot be used together"},
	{"send recipient conflict", []string{"messages", "send", "--to", "user@example.com", "--recipients", "list.csv"},
		"--to and --recipients cannot be used together"},
	{"deprecated switch with a value", []string{"webhooks", "update", rawTestID, "--enable=false"},
		"--enable takes no value; leave it out, or give --enabled=false"},
	{"deprecated negated switch with a value", []string{"routes", "update", rawTestID, "--disabled=false"},
		"--disabled takes no value; leave it out, or give --enabled"},
	{"deprecated interactive switch with a value", []string{"routes", "create", "--interactive=false"},
		"--interactive takes no value; leave it out, as prompting is the default, or give --non-interactive to turn prompting off"},
}

func TestCommandLineErrorsAreValidationErrorsInJSON(t *testing.T) {
	for _, tt := range commandLineErrors {
		t.Run(tt.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			// --output json comes last, so a bad flag stops Cobra's parsing
			// before it reaches it.
			stdout, stderr, code := runRoot(t, append(tt.args, "--output", "json")...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stderr)
			assert.False(t, *called, "the command must not authenticate")
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &body), "stdout: %s", stdout)
			assert.Equal(t, true, body["error"])
			assert.Contains(t, body["message"], tt.message)
		})
	}
}

func TestUnsupportedOutputFormatIsAValidationError(t *testing.T) {
	called := testutil.RefuseAuth(t)

	stdout, stderr, code := runRoot(t, "apikeys", "list", "--output", "xml")

	assert.Equal(t, 4, code)
	assert.Empty(t, stdout)
	assert.Equal(t, "Error: --output must be one of: json, table, plain, csv\n", stderr)
	assert.False(t, *called, "the command must not authenticate")
}

func TestCommandLineErrorsGoToStderrOutsideJSON(t *testing.T) {
	for _, tt := range commandLineErrors {
		t.Run(tt.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, tt.args...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout)
			assert.True(t, strings.HasPrefix(stderr, "Error: "), "stderr: %s", stderr)
			assert.Contains(t, stderr, tt.message)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestRequiredFlagAndFlagGroupErrorsAreValidationErrors checks that a
// missing required flag and a broken flag group fail as command-line
// mistakes, exit 4, like Cobra's other errors, though Cobra checks them
// after the root's checks.
func TestRequiredFlagAndFlagGroupErrorsAreValidationErrors(t *testing.T) {
	tests := map[string]struct {
		mark    func(cmd *cobra.Command) error
		args    []string
		message string
	}{
		"required flag": {
			func(cmd *cobra.Command) error { return cmd.MarkFlagRequired("name") },
			nil, `required flag(s) "name" not set`,
		},
		"mutually exclusive flags": {
			func(cmd *cobra.Command) error { cmd.MarkFlagsMutuallyExclusive("name", "other"); return nil },
			[]string{"--name", "a", "--other", "b"}, "if any flags in the group [name other] are set none of the others can be",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			root := NewRootCmdForTesting()
			marked := &cobra.Command{Use: "marked", RunE: func(*cobra.Command, []string) error { return nil }}
			marked.Flags().String("name", "", "")
			marked.Flags().String("other", "", "")
			require.NoError(t, tt.mark(marked))
			root.AddCommand(marked)
			var stdout, stderr bytes.Buffer
			root.SetOut(&stdout)
			root.SetErr(&stderr)

			code := run(root, append([]string{"marked"}, tt.args...))

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout.String())
			assert.Contains(t, stderr.String(), "Error: "+tt.message)
		})
	}
}

// TestGlobalAuthFlagErrorsExitBeforeAuthenticating checks that the root
// command rejects a wrong use of --api-key and --account-id with exit 4
// before any command authenticates.
func TestGlobalAuthFlagErrorsExitBeforeAuthenticating(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		message string
	}{
		{"api key without account ID", []string{"--api-key", "key"},
			"--account-id is required when using --api-key"},
		{"account ID without api key", []string{"--account-id", rawTestID},
			"--api-key is required when using --account-id"},
		{"account ID that is not a UUID", []string{"--api-key", "key", "--account-id", "not-a-uuid"},
			`invalid account ID value "not-a-uuid": not a UUID`},
		{"empty api key", []string{"--api-key", "", "--account-id", rawTestID}, "--api-key cannot be empty"},
		{"api key with a control character", []string{"--api-key", "aha-sk-\x1b[Dkey", "--account-id", rawTestID},
			"--api-key must be printable ASCII, without control or non-ASCII characters"},
		{"api key with a non-ASCII character", []string{"--api-key", "aha-sk-k\u00e9y", "--account-id", rawTestID},
			"--api-key must be printable ASCII, without control or non-ASCII characters"},
		{"api key with a profile", []string{"--api-key", "key", "--account-id", rawTestID, "--profile", "staging"},
			"--api-key and --profile cannot be used together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, append([]string{"domains", "list"}, tt.args...)...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout)
			assert.Contains(t, stderr, tt.message)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

// TestLoginChecksTheCredentialFlagsLikeEveryCommand runs auth login, which
// the root command does not check, with the credential flag mistakes the
// root command rejects.
func TestLoginChecksTheCredentialFlagsLikeEveryCommand(t *testing.T) {
	tests := map[string][]string{
		"--account-id is required when using --api-key":                              {"--api-key", "key"},
		"--api-key is required when using --account-id":                              {"--account-id", rawTestID},
		"--api-key cannot be empty":                                                  {"--api-key", "", "--account-id", rawTestID},
		"--api-key must be printable ASCII, without control or non-ASCII characters": {"--api-key", "aha-sk-\x1akey", "--account-id", rawTestID},
	}
	for message, args := range tests {
		t.Run(message, func(t *testing.T) {
			testutil.UseNewHome(t)

			stdout, stderr, code := runRoot(t, append([]string{"auth", "login", "--api-url", "http://127.0.0.1:1"}, args...)...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+message+"\n", stderr)
		})
	}
}

func TestLoginRejectsAnAccountIDThatIsNotAUUID(t *testing.T) {
	testutil.UseNewHome(t)

	stdout, stderr, code := runRoot(t, "auth", "login", "--api-key", "key", "--account-id", "not-a-uuid",
		"--api-url", "http://127.0.0.1:1")

	assert.Equal(t, 4, code)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, `invalid account ID value "not-a-uuid": not a UUID`)
}

func TestCommandLineErrorsPrintOnlyTheError(t *testing.T) {
	tests := []struct {
		args    []string
		message string
	}{
		{[]string{"apikeys", "list", "--bogus"}, "unknown flag: --bogus"},
		{[]string{"apikeys", "list", "--limit", "many"},
			`invalid argument "many" for "--limit" flag: strconv.ParseInt: parsing "many": invalid syntax`},
		{[]string{"apikeys", "bogus"}, `unknown command "bogus" for "ahasend apikeys"`},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			stdout, stderr, code := runRoot(t, tt.args...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout)
			assert.Equal(t, "Error: "+tt.message+"\n", stderr)
		})
	}
}

func TestArgumentErrorsDoNotShowUsage(t *testing.T) {
	for _, args := range [][]string{{"apikeys", "get"}, {"apikeys", "list", "extra"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, stderr, code := runRoot(t, args...)

			assert.Equal(t, 4, code)
			assert.NotContains(t, stderr, "Usage:")
		})
	}
}

func TestCommandErrorsDoNotShowUsage(t *testing.T) {
	_, stderr, code := runRoot(t, "apikeys", "get", "not-a-uuid")

	assert.Equal(t, 4, code)
	assert.NotContains(t, stderr, "Usage:")
}

// TestUnknownProfileIsAConfigError checks that every way of naming a profile
// that does not exist exits 3.
func TestUnknownProfileIsAConfigError(t *testing.T) {
	for _, args := range [][]string{
		{"domains", "list", "--profile", "nope"},
		{"auth", "status", "--profile", "nope"},
		{"auth", "switch", "nope"},
		{"auth", "logout", "nope"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			testutil.UseNewHome(t)

			stdout, stderr, code := runRoot(t, append(args, "--output", "json")...)

			assert.Equal(t, 3, code, "stderr: %s", stderr)
			assert.Contains(t, stdout, "not found")
		})
	}
}

func TestStatsBadGroupByPrintsOneErrorBeforeAuthenticating(t *testing.T) {
	for _, command := range []string{"bounces", "deliverability", "delivery-time"} {
		t.Run(command, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, _, code := runRoot(t, "stats", command, "--group-by", "year", "--output", "json")

			assert.Equal(t, 4, code)
			assert.False(t, *called, "the command must not authenticate")
			decoder := json.NewDecoder(strings.NewReader(stdout))
			var first, second map[string]any
			require.NoError(t, decoder.Decode(&first))
			assert.Contains(t, first["message"], "--group-by must be one of: hour, day, week, month")
			assert.Error(t, decoder.Decode(&second), "stdout holds a second JSON document")
		})
	}
}

// commandGroups returns the path of every command group below root, nested
// groups included.
func commandGroups(t *testing.T) [][]string {
	t.Helper()
	var groups [][]string
	walkCommands(NewRootCmdForTesting(), func(command *cobra.Command) {
		if command.HasParent() && command.HasSubCommands() {
			groups = append(groups, strings.Fields(strings.TrimPrefix(command.CommandPath(), "ahasend ")))
		}
	})
	require.NotEmpty(t, groups)
	return groups
}

func TestUnknownSubcommandOfAGroupIsAValidationError(t *testing.T) {
	for _, group := range commandGroups(t) {
		path := strings.Join(group, " ")
		t.Run(path, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			stdout, stderr, code := runRoot(t, append(group, "bogus")...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stdout)
			assert.True(t, strings.HasPrefix(stderr, `Error: unknown command "bogus" for "ahasend `+path+`"`), "stderr: %s", stderr)
			assert.False(t, *called, "the command must not authenticate")
		})
	}
}

func TestUnknownSubcommandOfAGroupIsAJSONErrorInJSON(t *testing.T) {
	for _, group := range commandGroups(t) {
		path := strings.Join(group, " ")
		t.Run(path, func(t *testing.T) {
			stdout, stderr, code := runRoot(t, append(group, "bogus", "--output", "json")...)

			assert.Equal(t, 4, code)
			assert.Empty(t, stderr)
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &body), "stdout: %s", stdout)
			assert.Equal(t, true, body["error"])
			assert.Equal(t, `unknown command "bogus" for "ahasend `+path+`"`, body["message"])
		})
	}
}

// TestHelpShowsTheHelpOfTheCommandItNames checks that help of a command
// shows that command's help, and help alone the CLI's.
func TestHelpShowsTheHelpOfTheCommandItNames(t *testing.T) {
	stdout, _, code := runRoot(t, "help", "domains", "list")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "ahasend domains list [flags]")

	stdout, _, code = runRoot(t, "help")
	assert.Equal(t, 0, code)
	assert.Contains(t, stdout, "ahasend [command]")
}

// TestCompletionWritesTheScriptOfAShell checks that a completion command
// still writes its script. Cobra's completion commands write to the stdout
// the root command had when they were added, os.Stdout here.
func TestCompletionWritesTheScriptOfAShell(t *testing.T) {
	script, err := os.Create(filepath.Join(t.TempDir(), "script"))
	require.NoError(t, err)
	defer script.Close()
	stdout := os.Stdout
	os.Stdout = script
	t.Cleanup(func() { os.Stdout = stdout })

	_, _, code := runRoot(t, "completion", "bash")

	assert.Equal(t, 0, code)
	written, err := os.ReadFile(script.Name())
	require.NoError(t, err)
	assert.Contains(t, string(written), "bash completion V2 for ahasend")
}

func TestGroupWithoutASubcommandShowsItsHelp(t *testing.T) {
	for _, group := range commandGroups(t) {
		t.Run(strings.Join(group, " "), func(t *testing.T) {
			stdout, _, code := runRoot(t, group...)

			assert.Equal(t, 0, code)
			assert.Contains(t, stdout, "Available Commands:")
		})
	}
}
