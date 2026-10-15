package domains

import (
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/prompt"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// unverifiedDomainBody is a domain whose DNS records are not configured yet.
const unverifiedDomainBody = `{"object":"domain","id":"11111111-1111-4111-8111-111111111111","domain":"example.com",` +
	`"dns_valid":false,"dns_records":[{"type":"TXT","host":"_dmarc.example.com","content":"v=DMARC1; p=none;","required":true,"propagated":false}]}`

// dnsHelpCommands are the commands that print DNS help for an unverified
// domain, with the client call each one makes.
var dnsHelpCommands = []struct {
	name    string
	command func() *cobra.Command
	args    []string
	method  string
	arg     any
}{
	{"create", NewCreateCommand, []string{"example.com"}, "CreateDomain", mock.AnythingOfType("requests.CreateDomainRequest")},
	{"verify", NewVerifyCommand, []string{"example.com"}, "GetDomain", "example.com"},
	{"check-dns", NewCheckDNSCommand, []string{"example.com"}, "CheckDomainDNS", "example.com"},
}

func TestDNSHelpGoesToStderrOutsideJSON(t *testing.T) {
	for _, tt := range dnsHelpCommands {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(tt.method, tt.arg).Return(mocks.APIResponse[responses.Domain](t, unverifiedDomainBody), nil)
			testutil.InjectClient(t, mockClient)

			stdout, stderr, err := testutil.RunCommand(t, tt.command(), "plain", "", tt.args...)

			require.NoError(t, err)
			assert.Contains(t, stdout, "example.com")
			assert.NotContains(t, stdout, "DNS Configuration for")
			assert.Contains(t, stderr, "DNS Configuration for example.com")
			assert.Contains(t, stderr, "_dmarc.example.com")
			assert.NotContains(t, stderr, "--show-dns-records")
			if tt.name != "create" {
				assert.Contains(t, stderr, "• Run 'ahasend domains check-dns example.com' to check the records again\n")
			}
			assert.Contains(t, stderr, "Once configured, run: ahasend domains check-dns example.com\n")
			assert.NotContains(t, stderr, "domains verify")
		})
	}
}

func TestDNSHelpIsSkippedInJSON(t *testing.T) {
	for _, tt := range dnsHelpCommands {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(tt.method, tt.arg).Return(mocks.APIResponse[responses.Domain](t, unverifiedDomainBody), nil)
			testutil.InjectClient(t, mockClient)

			stdout, stderr, err := testutil.RunCommand(t, tt.command(), "json", "", tt.args...)

			require.NoError(t, err)
			assert.Equal(t, testutil.IndentedJSON(t, unverifiedDomainBody), stdout)
			assert.Empty(t, stderr)
		})
	}
}

func TestCreatePromptsForTheDomainOnStderrBeforeAuthenticating(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	mockClient := &mocks.MockClient{}
	mockClient.On("CreateDomain", requests.CreateDomainRequest{Domain: "example.com"}).
		Return(mocks.APIResponse[responses.Domain](t, unverifiedDomainBody), nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "json", "example.com\n")

	require.NoError(t, err)
	assert.Equal(t, testutil.IndentedJSON(t, unverifiedDomainBody), stdout)
	assert.Equal(t, "Domain name: ", stderr)
	mockClient.AssertExpectations(t)
}

func TestCreateEndingInputAtThePromptCancels(t *testing.T) {
	t.Cleanup(prompt.TreatStdinAsTerminalForTesting())
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "")

	require.Error(t, err)
	assert.Equal(t, "operation cancelled", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.False(t, *called)
}

func TestCreateWithoutATerminalRequiresTheDomain(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, stderr, err := testutil.RunCommand(t, NewCreateCommand(), "plain", "example.com\n")

	require.Error(t, err)
	assert.Equal(t, "the domain argument is required", err.Error())
	assert.Equal(t, 4, clierrors.GetExitCode(err))
	assert.NotContains(t, stderr, "Domain name:")
	assert.False(t, *called)
}

func TestCreateRejectsAnInvalidDomainBeforeAuthenticating(t *testing.T) {
	called := testutil.RefuseAuth(t)

	_, _, err := testutil.RunCommand(t, NewCreateCommand(), "json", "", "invalid..domain.com")

	require.Error(t, err)
	assert.False(t, *called)
}

// verifiedDomainBody is a domain whose DNS records are configured.
const verifiedDomainBody = `{"object":"domain","id":"11111111-1111-4111-8111-111111111111","domain":"example.com",` +
	`"dns_valid":true,"dns_records":[{"type":"TXT","host":"_dmarc.example.com","content":"v=DMARC1; p=none;","required":true,"propagated":true}]}`

// TestShowRecordsPrintsTheSetupInstructionsOfAValidDomain checks what
// --show-records does and that its help says so: the records are listed on
// stdout either way, and the flag adds their setup instructions on stderr.
func TestShowRecordsPrintsTheSetupInstructionsOfAValidDomain(t *testing.T) {
	for _, tt := range dnsHelpCommands[1:] {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(tt.method, tt.arg).Return(mocks.APIResponse[responses.Domain](t, verifiedDomainBody), nil)
			testutil.InjectClient(t, mockClient)

			listed, quiet, err := testutil.RunCommand(t, tt.command(), "plain", "", tt.args...)
			require.NoError(t, err)
			_, shown, err := testutil.RunCommand(t, tt.command(), "plain", "", append(tt.args, "--show-records")...)
			require.NoError(t, err)

			assert.Contains(t, listed, "_dmarc.example.com")
			assert.NotContains(t, quiet, "_dmarc.example.com")
			assert.Contains(t, shown, "Add the required DNS records to your domain")
			command := tt.command()
			assert.Equal(t, "Print the DNS setup instructions even when the DNS is valid", command.Flags().Lookup("show-records").Usage)
			assert.Contains(t, strings.Join(strings.Fields(command.Long), " "),
				"--show-records prints the setup instructions for a valid domain too.")
		})
	}
}

// TestEditIsAHiddenDeprecatedUpdate checks that the old name edit is hidden,
// prints a deprecation notice on stderr and runs update.
func TestEditIsAHiddenDeprecatedUpdate(t *testing.T) {
	command, _, err := NewCommand().Find([]string{"edit"})
	require.NoError(t, err)
	assert.True(t, command.Hidden)

	domain := "example.com"
	sendingType := "marketing"
	mockClient := &mocks.MockClient{}
	mockClient.On("UpdateDomain", domain, requests.UpdateDomainRequest{SendingType: &sendingType}).
		Return(&responses.Domain{Domain: domain}, nil)
	testutil.InjectClient(t, mockClient)

	stdout, stderr, err := testutil.RunCommand(t, newEditCommand(), "plain", "", domain, "--sending-type", sendingType)
	require.NoError(t, err)
	assert.Equal(t, "Command \"edit\" is deprecated, use \"domains update\" instead\n", stderr)
	assert.NotContains(t, stdout, "deprecated")
	mockClient.AssertExpectations(t)
}

// apiRecordsDomainBody is a domain with records as the API returns them: the
// SPF TXT record and the MX record share the domain as host, the MX content
// is the mail server alone, and the MX record, for inbound email, is
// optional.
const apiRecordsDomainBody = `{"object":"domain","id":"11111111-1111-4111-8111-111111111111","domain":"example.com",` +
	`"dns_valid":false,"dns_records":[` +
	`{"type":"TXT","host":"example.com","content":"v=spf1 include:_spf.ahasend.com ~all","required":true,"propagated":false},` +
	`{"type":"MX","host":"example.com","content":"mx.ahasend.com","required":false,"propagated":false}]}`

// The notes of the records of apiRecordsDomainBody.
const (
	spfNote = "required: a name can have only one SPF record: if example.com has a TXT record that starts with v=spf1, " +
		"add include:_spf.ahasend.com to it instead of adding this one"
	mxNote = "optional: receives email sent to example.com through AhaSend, for inbound routes; " +
		"add it only if you want that, since it changes where email to example.com is delivered"
)

// TestCreateFormatWritesTheRecordsInTheProviderFormat checks the stderr of
// --format in each provider format: a header in table output, the records
// alone in plain output.
func TestCreateFormatWritesTheRecordsInTheProviderFormat(t *testing.T) {
	tests := map[string]string{
		"bind": "; " + spfNote + "\n" +
			"example.com.\t3600\tIN\tTXT\t\"v=spf1 include:_spf.ahasend.com ~all\"\n" +
			"; " + mxNote + "\n" +
			"example.com.\t3600\tIN\tMX\t10 mx.ahasend.com.\n",
		"cloudflare": "# " + spfNote + "\n" +
			"Type: TXT, Name: example.com, Content: v=spf1 include:_spf.ahasend.com ~all, TTL: 3600\n" +
			"# " + mxNote + "\n" +
			"Type: MX, Name: example.com, Mail server: mx.ahasend.com, Priority: 10, TTL: 3600\n",
		"terraform": "# " + spfNote + "\n" + `resource "cloudflare_dns_record" "example_com_txt" {
  zone_id = var.zone_id
  name    = "example.com"
  type    = "TXT"
  content = "v=spf1 include:_spf.ahasend.com ~all"
  ttl     = 3600
}
` + "# " + mxNote + "\n" + `resource "cloudflare_dns_record" "example_com_mx" {
  zone_id  = var.zone_id
  name     = "example.com"
  type     = "MX"
  content  = "mx.ahasend.com"
  ttl      = 3600
  priority = 10
}
`,
	}
	for format, records := range tests {
		for output, header := range map[string]string{
			"plain": "",
			"table": "\nDNS Records (" + strings.ToUpper(format) + " format):\n" + strings.Repeat("-", 40) + "\n",
		} {
			t.Run(format+"/"+output, func(t *testing.T) {
				mockClient := &mocks.MockClient{}
				mockClient.On("CreateDomain", requests.CreateDomainRequest{Domain: "example.com"}).
					Return(mocks.APIResponse[responses.Domain](t, apiRecordsDomainBody), nil)
				testutil.InjectClient(t, mockClient)

				_, stderr, err := testutil.RunCommand(t, NewCreateCommand(), output, "", "example.com", "--format", format)

				require.NoError(t, err)
				assert.Equal(t, header+records, stderr)
			})
		}
	}
}

// TestCreateFormatRejectsFlagsThatPrintNoRecords checks that --format with
// --no-dns-help or --output json fails before authenticating.
func TestCreateFormatRejectsFlagsThatPrintNoRecords(t *testing.T) {
	tests := map[string]struct {
		output string
		args   []string
		want   string
	}{
		"no-dns-help": {"plain", []string{"--format", "bind", "--no-dns-help"}, "--format and --no-dns-help cannot be used together"},
		"json": {"json", []string{"--format", "bind"},
			"--format and --output json cannot be used together: JSON output prints the API's response unchanged"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)

			_, _, err := testutil.RunCommand(t, NewCreateCommand(), test.output, "", append([]string{"example.com"}, test.args...)...)

			require.Error(t, err)
			assert.Equal(t, test.want, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}

func TestDNSHelpEscapesTheRecords(t *testing.T) {
	const hostileBody = `{"object":"domain","id":"11111111-1111-4111-8111-111111111111","domain":"example.com",` +
		`"dns_valid":false,"dns_records":[{"type":"TXT","host":"_dmarc.example.com","content":"v=DMARC1\u001b]0;x\u0007","required":true,"propagated":false}]}`
	for _, tt := range dnsHelpCommands {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mocks.MockClient{}
			mockClient.On(tt.method, tt.arg).Return(mocks.APIResponse[responses.Domain](t, hostileBody), nil)
			testutil.InjectClient(t, mockClient)

			_, stderr, err := testutil.RunCommand(t, tt.command(), "plain", "", tt.args...)

			require.NoError(t, err)
			assert.Contains(t, stderr, `v=DMARC1\u001b]0;x\u0007`)
			assert.NotContains(t, stderr, "\x1b")
		})
	}
}
