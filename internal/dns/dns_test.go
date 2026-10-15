package dns

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
)

func TestFormatDNSRecords(t *testing.T) {
	domain := mocks.NewDomain()

	recordSet := FormatDNSRecords(domain)

	assert.NotNil(t, recordSet)
	assert.Equal(t, domain.Domain, recordSet.Domain)
	assert.Len(t, recordSet.Records, 2) // Based on test fixture

	// Verify first record (TXT)
	txtRecord := recordSet.Records[0]
	assert.Equal(t, "TXT", txtRecord.Type)
	assert.Equal(t, "_dmarc.example.com", txtRecord.Name)
	assert.Equal(t, "v=DMARC1; p=none;", txtRecord.Value)
	assert.Equal(t, 3600, txtRecord.TTL)

	// Verify second record (CNAME)
	cnameRecord := recordSet.Records[1]
	assert.Equal(t, "CNAME", cnameRecord.Type)
	assert.Equal(t, "link.example.com", cnameRecord.Name)
	assert.Equal(t, "track.ahasend.com", cnameRecord.Value)
	assert.Equal(t, 3600, cnameRecord.TTL)
}

func TestFormatDNSRecordsForProvider_FormatsEachProvider(t *testing.T) {
	record := DNSRecord{
		Type:  "TXT",
		Name:  "_dmarc.example.com",
		Value: "v=DMARC1; p=none;",
		TTL:   3600,
	}

	tests := []struct {
		name     string
		provider string
		expected string
	}{
		{
			name:     "bind format",
			provider: "bind",
			expected: "_dmarc.example.com.\t3600\tIN\tTXT\t\"v=DMARC1; p=none;\"",
		},
		{
			name:     "cloudflare format",
			provider: "cloudflare",
			expected: "Type: TXT, Name: _dmarc.example.com, Content: v=DMARC1; p=none;, TTL: 3600",
		},
		{
			name:     "terraform format",
			provider: "terraform",
			expected: `resource "cloudflare_dns_record" "_dmarc_example_com_txt" {
  zone_id = var.zone_id
  name    = "_dmarc.example.com"
  type    = "TXT"
  content = "v=DMARC1; p=none;"
  ttl     = 3600
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatRecord(record, tt.provider)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrintDNSInstructions(t *testing.T) {
	recordSet := &DNSRecordSet{
		Domain: "example.com",
		Records: []DNSRecord{
			{
				Type:  "TXT",
				Name:  "_dmarc.example.com",
				Value: "v=DMARC1; p=none;",
				TTL:   3600,
			},
			{
				Type:  "CNAME",
				Name:  "link.example.com",
				Value: "track.ahasend.com",
				TTL:   3600,
			},
		},
	}

	var buffer bytes.Buffer
	PrintDNSInstructions(&buffer, recordSet)
	output := buffer.String()

	assert.True(t, strings.HasPrefix(output, "\nDNS Configuration for example.com\n"), output)
	assert.True(t, strings.HasSuffix(output, "\nDNS propagation can take up to 48 hours, but usually completes within a few minutes.\n"+
		"Once configured, run: ahasend domains check-dns example.com\n"), output)
	assert.Contains(t, output, "TXT")
	assert.Contains(t, output, "CNAME")
	assert.Contains(t, output, "_dmarc.example.com")
	assert.Contains(t, output, "link.example.com")
	assert.Contains(t, output, "v=DMARC1; p=none;")
	assert.Contains(t, output, "track.ahasend.com")
}

func TestFormatDNSRecords_EmptyDomain(t *testing.T) {
	domain := &responses.Domain{
		Domain:     "empty.com",
		DNSValid:   false,
		CreatedAt:  time.Now(),
		DNSRecords: []responses.DNSRecord{}, // Empty records
	}

	recordSet := FormatDNSRecords(domain)

	assert.NotNil(t, recordSet)
	assert.Equal(t, "empty.com", recordSet.Domain)
	assert.Empty(t, recordSet.Records)
}

func TestFormatDNSRecords_NilDomain(t *testing.T) {
	recordSet := FormatDNSRecords(nil)

	assert.NotNil(t, recordSet)
	assert.Empty(t, recordSet.Domain)
	assert.Empty(t, recordSet.Records)
}

func TestFormatBindRecord(t *testing.T) {
	record := DNSRecord{
		Type:  "MX",
		Name:  "example.com",
		Value: "10 mail.example.com",
		TTL:   3600,
	}

	// Test the bind format logic (these are internal functions, so we test via public interface)
	result := formatRecord(record, "bind")
	expected := "example.com.\t3600\tIN\tMX\t10 mail.example.com."
	assert.Equal(t, expected, result)
}

func TestFormatBindRecordEndsNamesWithADot(t *testing.T) {
	record := DNSRecord{Type: "CNAME", Name: "link.example.com", Value: "track.ahasend.com.", TTL: 3600}

	result := formatRecord(record, "bind")

	assert.Equal(t, "link.example.com.\t3600\tIN\tCNAME\ttrack.ahasend.com.", result)
}

func TestFormatBindRecordEscapesQuotesAndBackslashesInTXT(t *testing.T) {
	record := DNSRecord{Type: "TXT", Name: "example.com", Value: `say "hi" \ bye`, TTL: 3600}

	result := formatRecord(record, "bind")

	assert.Equal(t, "example.com.\t3600\tIN\tTXT\t\"say \\\"hi\\\" \\\\ bye\"", result)
}

func TestFormatBindRecordSplitsALongTXTValue(t *testing.T) {
	value := "v=DKIM1; k=rsa; p=" + strings.Repeat("A", 300)
	record := DNSRecord{Type: "TXT", Name: "aha._domainkey.example.com", Value: value, TTL: 3600}

	result := formatRecord(record, "bind")

	assert.Equal(t, "aha._domainkey.example.com.\t3600\tIN\tTXT\t\""+value[:255]+"\" \""+value[255:]+"\"", result)
}

func TestFormatCloudflareRecord(t *testing.T) {
	record := DNSRecord{
		Type:  "A",
		Name:  "www.example.com",
		Value: "192.168.1.1",
		TTL:   300,
	}

	result := formatRecord(record, "cloudflare")
	expected := "Type: A, Name: www.example.com, Content: 192.168.1.1, TTL: 300"
	assert.Equal(t, expected, result)
}

func TestFormatTerraformRecord(t *testing.T) {
	record := DNSRecord{
		Type:  "AAAA",
		Name:  "ipv6.example.com",
		Value: "2001:db8::1",
		TTL:   1800,
	}

	result := formatRecord(record, "terraform")

	assert.Contains(t, result, `resource "cloudflare_dns_record"`)
	assert.Contains(t, result, `name    = "ipv6.example.com"`)
	assert.Contains(t, result, `type    = "AAAA"`)
	assert.Contains(t, result, `content = "2001:db8::1"`)
	assert.Contains(t, result, `ttl     = 1800`)
}

func TestFormatTerraformRecordEscapesHCLStrings(t *testing.T) {
	record := DNSRecord{Type: "TXT", Name: "example.com", Value: `a "b" \ ${c} %{d}`, TTL: 3600}

	result := formatRecord(record, "terraform")

	assert.Contains(t, result, `content = "a \"b\" \\ $${c} %%{d}"`)
}

// apiRecords are records as the API returns them: the SPF TXT record and the
// MX record share the domain as host, and the MX content is the mail server
// alone.
var apiRecords = []DNSRecord{
	{Type: "TXT", Name: "example.com", Value: "v=spf1 include:_spf.ahasend.com ~all", TTL: 3600},
	{Type: "MX", Name: "example.com", Value: "mx.ahasend.com", TTL: 3600},
}

func TestFormatDNSRecordsForProvider_BINDGivesMXAPreferenceAndAnAbsoluteServer(t *testing.T) {
	assert.Equal(t, []string{
		"example.com.\t3600\tIN\tTXT\t\"v=spf1 include:_spf.ahasend.com ~all\"",
		"example.com.\t3600\tIN\tMX\t10 mx.ahasend.com.",
	}, FormatDNSRecordsForProvider(apiRecords, "bind"))
}

func TestFormatDNSRecordsForProvider_CloudflareGivesMXAPriority(t *testing.T) {
	assert.Equal(t, []string{
		"Type: TXT, Name: example.com, Content: v=spf1 include:_spf.ahasend.com ~all, TTL: 3600",
		"Type: MX, Name: example.com, Mail server: mx.ahasend.com, Priority: 10, TTL: 3600",
	}, FormatDNSRecordsForProvider(apiRecords, "cloudflare"))
}

func TestFormatDNSRecordsForProvider_TerraformGivesUniqueLabelsAndMXAPriority(t *testing.T) {
	assert.Equal(t, []string{`resource "cloudflare_dns_record" "example_com_txt" {
  zone_id = var.zone_id
  name    = "example.com"
  type    = "TXT"
  content = "v=spf1 include:_spf.ahasend.com ~all"
  ttl     = 3600
}`, `resource "cloudflare_dns_record" "example_com_mx" {
  zone_id  = var.zone_id
  name     = "example.com"
  type     = "MX"
  content  = "mx.ahasend.com"
  ttl      = 3600
  priority = 10
}`}, FormatDNSRecordsForProvider(apiRecords, "terraform"))
}

func TestFormatDNSRecordsForProvider_KeepsAnMXPreferenceGiven(t *testing.T) {
	record := DNSRecord{Type: "MX", Name: "example.com", Value: "20 mx.ahasend.com", TTL: 3600}

	assert.Equal(t, "example.com.\t3600\tIN\tMX\t20 mx.ahasend.com.", formatRecord(record, "bind"))
	assert.Contains(t, formatRecord(record, "terraform"), "  content  = \"mx.ahasend.com\"\n  ttl      = 3600\n  priority = 20\n}")
}

func TestFormatDNSRecordsForProvider_TerraformLabelsAreValidAndUnique(t *testing.T) {
	records := []DNSRecord{
		{Type: "TXT", Name: "1.example.com", Value: "a", TTL: 3600},
		{Type: "TXT", Name: "1.example.com", Value: "b", TTL: 3600},
		{Type: "CNAME", Name: "ex-ample.com.", Value: "c", TTL: 3600},
		{Type: "TXT", Name: "é$x.com", Value: "d", TTL: 3600},
	}

	formatted := FormatDNSRecordsForProvider(records, "terraform")

	var labels []string
	for _, resource := range formatted {
		labels = append(labels, strings.Split(strings.SplitN(resource, "\n", 2)[0], `"`)[3])
	}
	assert.Equal(t, []string{"_1_example_com_txt", "_1_example_com_txt_2", "ex_ample_com_cname", "__x_com_txt"}, labels)
}

// serverRecordsDomain is a domain with the records the API returns for a
// domain without managed DNS, in its order, with the default subdomains
// except a custom return path.
func serverRecordsDomain() *responses.Domain {
	returnPath := "bounce"
	return &responses.Domain{
		Domain:              "example.com",
		ReturnPathSubdomain: &returnPath,
		DNSRecords: []responses.DNSRecord{
			{Type: "TXT", Host: "aha-sender._domainkey.example.com", Content: "v=DKIM1; k=rsa; p=MIGf", Required: true},
			{Type: "TXT", Host: "example.com", Content: "v=spf1 include:_spf.ahasend.com ~all", Required: true},
			{Type: "TXT", Host: "_dmarc.example.com", Content: "v=DMARC1; p=none;", Required: true},
			{Type: "CNAME", Host: "bounce.example.com", Content: "rp.ahasend.com"},
			{Type: "CNAME", Host: "t.example.com", Content: "track.ahasend.com"},
			{Type: "CNAME", Host: "media.example.com", Content: "media.ahasend.com"},
			{Type: "CNAME", Host: "subs.example.com", Content: "subscriptions.ahasend.com"},
			{Type: "MX", Host: "example.com", Content: "mx.ahasend.com"},
		},
	}
}

func TestFormatDNSRecordsMarksRequiredRecordsAndNotesTheOthers(t *testing.T) {
	records := FormatDNSRecords(serverRecordsDomain()).Records

	required := make([]bool, len(records))
	notes := make([]string, len(records))
	for i, record := range records {
		required[i] = record.Required
		notes[i] = record.Note
	}
	assert.Equal(t, []bool{true, true, true, false, false, false, false, false}, required)
	assert.Equal(t, []string{
		"",
		"a name can have only one SPF record: if example.com has a TXT record that starts with v=spf1, " +
			"add include:_spf.ahasend.com to it instead of adding this one",
		"a name can have only one DMARC record: if _dmarc.example.com has one, keep it instead of adding this one",
		"the return path: bounces come back to this subdomain, which aligns SPF with your domain for DMARC",
		"branded open and click tracking links",
		"branded links to media AhaSend hosts, such as images",
		"branded unsubscribe and subscription preference pages",
		"receives email sent to example.com through AhaSend, for inbound routes; add it only if you want that, " +
			"since it changes where email to example.com is delivered",
	}, notes)
}

func TestFormatDNSRecordsNotesTheSecondDKIMKeyOfManagedDNS(t *testing.T) {
	domain := &responses.Domain{Domain: "example.com", DNSRecords: []responses.DNSRecord{
		{Type: "CNAME", Host: "aha1._domainkey.example.com", Content: "aha1.dkim.ahasend.com", Required: true},
		{Type: "CNAME", Host: "aha2._domainkey.example.com", Content: "aha2.dkim.ahasend.com"},
	}}

	records := FormatDNSRecords(domain).Records

	assert.Equal(t, "", records[0].Note)
	assert.Equal(t, "the second DKIM key, which DKIM key rotation switches to", records[1].Note)
}

func TestPrintDNSInstructionsMarksEachRecordAndShowsItsNote(t *testing.T) {
	var buffer bytes.Buffer
	PrintDNSInstructions(&buffer, FormatDNSRecords(serverRecordsDomain()))
	output := buffer.String()

	assert.Contains(t, output, "Record #1 (required):\n  Type:  TXT\n  Name:  aha-sender._domainkey.example.com\n"+
		"  TTL:   3600\n  Value: v=DKIM1; k=rsa; p=MIGf\n\n")
	assert.Contains(t, output, "Record #8 (optional):\n  Type:  MX\n  Name:  example.com\n  TTL:   3600\n"+
		"  Value: mx.ahasend.com\n  Note:  receives email sent to example.com through AhaSend")
}

func TestProviderCommentMarksOptionalRecordsInEachFormat(t *testing.T) {
	records := FormatDNSRecords(serverRecordsDomain()).Records
	tests := map[string]string{"bind": ";", "cloudflare": "#", "terraform": "#"}
	for provider, marker := range tests {
		t.Run(provider, func(t *testing.T) {
			assert.Equal(t, "", ProviderComment(records[0], provider))
			assert.Equal(t, marker+" required: "+records[1].Note, ProviderComment(records[1], provider))
			assert.Equal(t, marker+" optional: branded open and click tracking links", ProviderComment(records[4], provider))
		})
	}
}

// formatRecord formats one record in provider.
func formatRecord(record DNSRecord, provider string) string {
	return FormatDNSRecordsForProvider([]DNSRecord{record}, provider)[0]
}
