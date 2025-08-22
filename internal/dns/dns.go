// Package dns provides DNS record formatting and display utilities.
//
// This package handles the formatting and display of DNS records required
// for AhaSend domain verification across different DNS providers and formats:
//
//   - DNS record parsing from AhaSend domain responses
//   - Multi-format output (BIND, Cloudflare, Terraform)
//   - Interactive DNS configuration instructions
//   - Provider-specific formatting and syntax
//   - Copy-paste friendly record formats
//
// The package supports common DNS providers and infrastructure-as-code
// tools, making it easy for users to configure their domains regardless
// of their DNS setup.
package dns

import (
	"fmt"
	"strings"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// DNSRecord represents a DNS record with formatting options
type DNSRecord struct {
	Type     string
	Name     string
	Value    string
	Priority int
	TTL      int
}

// DNSRecordSet represents a collection of DNS records for a domain
type DNSRecordSet struct {
	Domain  string
	Records []DNSRecord
}

// FormatDNSRecords formats DNS records for display
func FormatDNSRecords(domain *responses.Domain) *DNSRecordSet {
	if domain == nil || domain.DNSRecords == nil {
		return &DNSRecordSet{
			Domain:  "",
			Records: []DNSRecord{},
		}
	}

	records := make([]DNSRecord, 0)

	// Process each DNS record from the domain response
	for _, record := range domain.DNSRecords {
		dnsRecord := DNSRecord{
			TTL:      3600, // Default TTL
			Type:     record.Type,
			Name:     record.Host,
			Value:    record.Content,
			Priority: 0, // Default priority
		}

		records = append(records, dnsRecord)
	}

	return &DNSRecordSet{
		Domain:  domain.Domain,
		Records: records,
	}
}

// PrintDNSInstructions prints DNS configuration instructions
func PrintDNSInstructions(recordSet *DNSRecordSet) {
	if len(recordSet.Records) == 0 {
		fmt.Println("No DNS records to configure.")
		return
	}

	fmt.Printf("\n📋 DNS Configuration for %s\n", recordSet.Domain)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("\nAdd the following DNS records to your domain:")
	fmt.Println()

	// Use expanded format for better readability of long DNS values
	for i, record := range recordSet.Records {
		if i > 0 {
			fmt.Println() // Add spacing between records
		}

		fmt.Printf("Record #%d:\n", i+1)
		fmt.Printf("  Type:  %s\n", record.Type)
		fmt.Printf("  Name:  %s\n", record.Name)
		fmt.Printf("  TTL:   %d\n", record.TTL)

		if record.Type == "MX" && record.Priority > 0 {
			fmt.Printf("  Priority: %d\n", record.Priority)
		}

		// Always show DNS values as single lines for safe copy-paste
		fmt.Printf("  Value: %s\n", record.Value)
	}

	fmt.Println()
	fmt.Println("⏱️  DNS propagation can take up to 48 hours, but usually completes within a few minutes.")
	fmt.Printf("✅ Once configured, run: ahasend domains verify %s\n", recordSet.Domain)
}

// FormatDNSRecordForProvider formats a DNS record for a specific provider
func FormatDNSRecordForProvider(record DNSRecord, provider string) string {
	switch strings.ToLower(provider) {
	case "bind", "zone":
		return formatBINDRecord(record)
	case "cloudflare":
		return formatCloudflareRecord(record)
	case "terraform":
		return formatTerraformRecord(record)
	default:
		return formatGenericRecord(record)
	}
}

// formatBINDRecord formats a record in BIND zone file format
func formatBINDRecord(record DNSRecord) string {
	if record.Type == "MX" && record.Priority > 0 {
		return fmt.Sprintf("%s\t%d\tIN\t%s\t%d %s", record.Name, record.TTL, record.Type, record.Priority, record.Value)
	}
	return fmt.Sprintf("%s\t%d\tIN\t%s\t%s", record.Name, record.TTL, record.Type, record.Value)
}

// formatCloudflareRecord formats a record for Cloudflare
func formatCloudflareRecord(record DNSRecord) string {
	result := fmt.Sprintf("Type: %s, Name: %s, Content: %s, TTL: %d", record.Type, record.Name, record.Value, record.TTL)
	if record.Type == "MX" && record.Priority > 0 {
		result = fmt.Sprintf("Type: %s, Name: %s, Priority: %d, Content: %s, TTL: %d", record.Type, record.Name, record.Priority, record.Value, record.TTL)
	}
	return result
}

// formatTerraformRecord formats a record for Terraform
func formatTerraformRecord(record DNSRecord) string {
	recordName := strings.ReplaceAll(record.Name, ".", "_")
	recordName = strings.ReplaceAll(recordName, "-", "_")

	if record.Type == "MX" && record.Priority > 0 {
		return fmt.Sprintf(`resource "cloudflare_record" "%s" {
  zone_id = var.zone_id
  name    = "%s"
  value   = "%s"
  type    = "%s"
  priority = %d
  ttl     = %d
}`, recordName, record.Name, record.Value, record.Type, record.Priority, record.TTL)
	}

	return fmt.Sprintf(`resource "cloudflare_record" "%s" {
  zone_id = var.zone_id
  name    = "%s"
  value   = "%s"
  type    = "%s"
  ttl     = %d
}`, recordName, record.Name, record.Value, record.Type, record.TTL)
}

// formatGenericRecord formats a record generically
func formatGenericRecord(record DNSRecord) string {
	if record.Type == "MX" && record.Priority > 0 {
		return fmt.Sprintf("%s %s %d %s (TTL: %d)", record.Type, record.Name, record.Priority, record.Value, record.TTL)
	}
	return fmt.Sprintf("%s %s %s (TTL: %d)", record.Type, record.Name, record.Value, record.TTL)
}
