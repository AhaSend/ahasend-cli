package dns

import (
	"testing"
	"time"

	"github.com/AhaSend/ahasend-cli/internal/testutil"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/stretchr/testify/assert"
)

func TestFormatDNSRecords(t *testing.T) {
	domain := testutil.TestDomain()

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

func TestFormatDNSRecordForProvider(t *testing.T) {
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
			expected: "_dmarc.example.com\t3600\tIN\tTXT\tv=DMARC1; p=none;",
		},
		{
			name:     "cloudflare format",
			provider: "cloudflare",
			expected: "Type: TXT, Name: _dmarc.example.com, Content: v=DMARC1; p=none;, TTL: 3600",
		},
		{
			name:     "terraform format",
			provider: "terraform",
			expected: `resource "cloudflare_record" "_dmarc_example_com" {
  zone_id = var.zone_id
  name    = "_dmarc.example.com"
  value   = "v=DMARC1; p=none;"
  type    = "TXT"
  ttl     = 3600
}`,
		},
		{
			name:     "generic format",
			provider: "generic",
			expected: "TXT _dmarc.example.com v=DMARC1; p=none; (TTL: 3600)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDNSRecordForProvider(record, tt.provider)
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

	// Capture output to test the printing function
	output, _ := testutil.CaptureOutput(t, func() {
		PrintDNSInstructions(recordSet)
	})

	assert.Contains(t, output, "DNS Configuration for example.com")
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
	result := FormatDNSRecordForProvider(record, "bind")
	expected := "example.com\t3600\tIN\tMX\t10 mail.example.com"
	assert.Equal(t, expected, result)
}

func TestFormatCloudflareRecord(t *testing.T) {
	record := DNSRecord{
		Type:  "A",
		Name:  "www.example.com",
		Value: "192.168.1.1",
		TTL:   300,
	}

	result := FormatDNSRecordForProvider(record, "cloudflare")
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

	result := FormatDNSRecordForProvider(record, "terraform")

	assert.Contains(t, result, `resource "cloudflare_record"`)
	assert.Contains(t, result, `name    = "ipv6.example.com"`)
	assert.Contains(t, result, `type    = "AAAA"`)
	assert.Contains(t, result, `value   = "2001:db8::1"`)
	assert.Contains(t, result, `ttl     = 1800`)
}

func TestFormatGenericRecord(t *testing.T) {
	record := DNSRecord{
		Type:  "SRV",
		Name:  "_sip._tcp.example.com",
		Value: "10 5 5060 sip.example.com",
		TTL:   600,
	}

	result := FormatDNSRecordForProvider(record, "generic")
	expected := "SRV _sip._tcp.example.com 10 5 5060 sip.example.com (TTL: 600)"
	assert.Equal(t, expected, result)
}

func TestTerraformResourceNaming(t *testing.T) {
	tests := []struct {
		name             string
		record           DNSRecord
		expectedContains string
	}{
		{
			name: "TXT record",
			record: DNSRecord{
				Type: "TXT",
				Name: "_dmarc.example.com",
			},
			expectedContains: "_dmarc_example_com",
		},
		{
			name: "CNAME record",
			record: DNSRecord{
				Type: "CNAME",
				Name: "link.example.com",
			},
			expectedContains: "link_example_com",
		},
		{
			name: "A record",
			record: DNSRecord{
				Type: "A",
				Name: "www.example.com",
			},
			expectedContains: "www_example_com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDNSRecordForProvider(tt.record, "terraform")
			assert.Contains(t, result, tt.expectedContains)
		})
	}
}
