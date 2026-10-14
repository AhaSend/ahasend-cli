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
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/AhaSend/ahasend-go/models/responses"
)

// DNSRecord represents a DNS record with formatting options
type DNSRecord struct {
	Type  string
	Name  string
	Value string
	TTL   int
	// Required is whether the domain needs the record to send email. An
	// optional record enables a feature.
	Required bool
	// Note says what an optional record is for, or how to add a required
	// record of a kind a name can have only one of; "" when there is
	// nothing to say.
	Note string
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
			Required: record.Required,
		}
		dnsRecord.Note = recordNote(dnsRecord, domain)

		records = append(records, dnsRecord)
	}

	return &DNSRecordSet{
		Domain:  domain.Domain,
		Records: records,
	}
}

// Default subdomains of the optional records, which the API uses when the
// domain sets none.
const (
	defaultTrackingSubdomain     = "t"
	defaultMediaSubdomain        = "media"
	defaultSubscriptionSubdomain = "subs"
)

// recordNote returns the note of record, one of domain's records: what an
// optional record is for, judged by its type and name, or how to add an SPF
// or DMARC record to a name that may have one already.
func recordNote(record DNSRecord, domain *responses.Domain) string {
	name := strings.TrimSuffix(strings.ToLower(record.Name), ".")
	isSubdomain := func(subdomain *string, fallback string) bool {
		label := fallback
		if subdomain != nil && strings.Trim(strings.TrimSpace(*subdomain), ".") != "" {
			label = strings.Trim(strings.TrimSpace(*subdomain), ".")
		}
		return label != "" && name == strings.ToLower(label+"."+domain.Domain)
	}
	recordType := strings.ToUpper(record.Type)

	if record.Required {
		switch {
		case recordType == "TXT" && strings.HasPrefix(strings.ToLower(record.Value), "v=spf1"):
			return fmt.Sprintf("a name can have only one SPF record: if %s has a TXT record that starts with v=spf1, "+
				"add %s to it instead of adding this one", record.Name, spfMechanisms(record.Value))
		case recordType == "TXT" && strings.HasPrefix(name, "_dmarc."):
			return fmt.Sprintf("a name can have only one DMARC record: if %s has one, keep it instead of adding this one", record.Name)
		}
		return ""
	}

	switch {
	case recordType == "MX":
		return fmt.Sprintf("receives email sent to %s through AhaSend, for inbound routes; add it only if you want that, "+
			"since it changes where email to %s is delivered", record.Name, record.Name)
	case recordType == "CNAME" && strings.Contains(name, "._domainkey."):
		return "the second DKIM key, which DKIM key rotation switches to"
	case recordType == "CNAME" && isSubdomain(domain.ReturnPathSubdomain, ""):
		return "the return path: bounces come back to this subdomain, which aligns SPF with your domain for DMARC"
	case recordType == "CNAME" && isSubdomain(domain.TrackingSubdomain, defaultTrackingSubdomain):
		return "branded open and click tracking links"
	case recordType == "CNAME" && isSubdomain(domain.MediaSubdomain, defaultMediaSubdomain):
		return "branded links to media AhaSend hosts, such as images"
	case recordType == "CNAME" && isSubdomain(domain.SubscriptionSubdomain, defaultSubscriptionSubdomain):
		return "branded unsubscribe and subscription preference pages"
	}
	return "a branded subdomain for AhaSend links or bounces"
}

// spfMechanisms returns the terms of an SPF record's value other than its
// version and its "all" term, such as "include:spf.ahasend.com": the part to
// add to an SPF record a name already has.
func spfMechanisms(value string) string {
	var terms []string
	for _, term := range strings.Fields(value) {
		lower := strings.ToLower(term)
		if lower == "v=spf1" || strings.TrimLeft(lower, "+-~?") == "all" {
			continue
		}
		terms = append(terms, term)
	}
	return strings.Join(terms, " ")
}

// requiredLabel is how the instructions mark whether a record is required.
func requiredLabel(record DNSRecord) string {
	if record.Required {
		return "required"
	}
	return "optional"
}

// PrintDNSInstructions writes DNS configuration instructions to w
func PrintDNSInstructions(w io.Writer, recordSet *DNSRecordSet) {
	if len(recordSet.Records) == 0 {
		fmt.Fprintln(w, "No DNS records to configure.")
		return
	}

	fmt.Fprintf(w, "\nDNS Configuration for %s\n", recordSet.Domain)
	fmt.Fprintln(w, strings.Repeat("=", 50))
	fmt.Fprintln(w, "\nAdd the required DNS records to your domain, and each optional one only for")
	fmt.Fprintln(w, "what its note says it does:")
	fmt.Fprintln(w)

	// Use expanded format for better readability of long DNS values
	for i, record := range recordSet.Records {
		if i > 0 {
			fmt.Fprintln(w) // Add spacing between records
		}

		fmt.Fprintf(w, "Record #%d (%s):\n", i+1, requiredLabel(record))
		fmt.Fprintf(w, "  Type:  %s\n", record.Type)
		fmt.Fprintf(w, "  Name:  %s\n", record.Name)
		fmt.Fprintf(w, "  TTL:   %d\n", record.TTL)

		// Always show DNS values as single lines for safe copy-paste
		fmt.Fprintf(w, "  Value: %s\n", record.Value)
		if record.Note != "" {
			fmt.Fprintf(w, "  Note:  %s\n", record.Note)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "DNS propagation can take up to 48 hours, but usually completes within a few minutes.")
	fmt.Fprintf(w, "Once configured, run: ahasend domains check-dns %s\n", recordSet.Domain)
}

// ProviderComment returns the comment line that goes before record in
// provider, one of Formats: whether the record is optional or required,
// with its note, or "" for a required record without a note. Bind comments
// start with ";", the others with "#".
func ProviderComment(record DNSRecord, provider string) string {
	if record.Required && record.Note == "" {
		return ""
	}
	marker := "#"
	if provider == "bind" {
		marker = ";"
	}
	comment := fmt.Sprintf("%s %s", marker, requiredLabel(record))
	if record.Note != "" {
		comment += ": " + record.Note
	}
	return comment
}

// Formats are the provider formats FormatDNSRecordsForProvider writes.
var Formats = []string{"bind", "cloudflare", "terraform"}

// FormatDNSRecordsForProvider formats each record in provider, one of
// Formats. Terraform resources get labels unique among records.
func FormatDNSRecordsForProvider(records []DNSRecord, provider string) []string {
	formatted := make([]string, len(records))
	labels := map[string]bool{}
	for i, record := range records {
		switch provider {
		case "cloudflare":
			formatted[i] = formatCloudflareRecord(record)
		case "terraform":
			formatted[i] = formatTerraformRecord(record, uniqueLabel(terraformLabel(record), labels))
		default:
			formatted[i] = formatBINDRecord(record)
		}
	}
	return formatted
}

// defaultMXPreference is the preference of an MX record whose value is the
// mail server alone, as the API returns it.
const defaultMXPreference = 10

// mxData returns the preference and the mail server of an MX record's value:
// "10 mx.example.com", or "mx.example.com" with defaultMXPreference.
func mxData(value string) (int, string) {
	fields := strings.Fields(value)
	if len(fields) == 2 {
		if preference, err := strconv.Atoi(fields[0]); err == nil {
			return preference, fields[1]
		}
	}
	return defaultMXPreference, value
}

// maxTXTStringLength is the most bytes one character-string of a TXT record
// holds; a longer value is split into several strings.
const maxTXTStringLength = 255

// formatBINDRecord formats a record as a zone file line. Names end with a dot,
// so the zone's origin is not appended to them, and a TXT value is quoted, so
// its spaces and semicolons stay part of it.
func formatBINDRecord(record DNSRecord) string {
	return fmt.Sprintf("%s\t%d\tIN\t%s\t%s", absoluteName(record.Name), record.TTL, record.Type, bindRecordData(record))
}

// bindRecordData returns the data of a record as a zone file writes it.
func bindRecordData(record DNSRecord) string {
	switch strings.ToUpper(record.Type) {
	case "TXT":
		return quoteTXT(record.Value)
	case "CNAME":
		return absoluteName(record.Value)
	case "MX":
		preference, server := mxData(record.Value)
		return fmt.Sprintf("%d %s", preference, absoluteName(server))
	}
	return record.Value
}

// absoluteName returns name with the trailing dot that marks it absolute in a
// zone file.
func absoluteName(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

// quoteTXT returns value as the quoted character-strings of a TXT record:
// pieces of up to 255 bytes, each in double quotes with its quotes and
// backslashes escaped.
func quoteTXT(value string) string {
	var pieces []string
	for len(value) > maxTXTStringLength {
		pieces = append(pieces, quoteString(value[:maxTXTStringLength]))
		value = value[maxTXTStringLength:]
	}
	pieces = append(pieces, quoteString(value))
	return strings.Join(pieces, " ")
}

// quoteString returns s in double quotes, with its quotes and backslashes
// escaped by a backslash.
func quoteString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// formatCloudflareRecord formats a record as the fields of Cloudflare's DNS
// dashboard, which takes the content as it is and an MX record's mail server
// and priority apart.
func formatCloudflareRecord(record DNSRecord) string {
	if strings.EqualFold(record.Type, "MX") {
		preference, server := mxData(record.Value)
		return fmt.Sprintf("Type: %s, Name: %s, Mail server: %s, Priority: %d, TTL: %d", record.Type, record.Name, server, preference, record.TTL)
	}
	return fmt.Sprintf("Type: %s, Name: %s, Content: %s, TTL: %d", record.Type, record.Name, record.Value, record.TTL)
}

// formatTerraformRecord formats a record as a cloudflare_dns_record
// resource of version 5 of the Cloudflare Terraform provider, labeled label,
// with its strings escaped as HCL requires and its arguments aligned as
// terraform fmt aligns them.
func formatTerraformRecord(record DNSRecord, label string) string {
	arguments := [][2]string{
		{"zone_id", "var.zone_id"},
		{"name", hclString(record.Name)},
		{"type", hclString(record.Type)},
	}
	if strings.EqualFold(record.Type, "MX") {
		preference, server := mxData(record.Value)
		arguments = append(arguments,
			[2]string{"content", hclString(server)},
			[2]string{"ttl", strconv.Itoa(record.TTL)},
			[2]string{"priority", strconv.Itoa(preference)})
	} else {
		arguments = append(arguments,
			[2]string{"content", hclString(record.Value)},
			[2]string{"ttl", strconv.Itoa(record.TTL)})
	}
	return terraformResource("cloudflare_dns_record", label, arguments)
}

// terraformResource formats a resource block of type kind labeled label with
// arguments, name and value pairs, in order, their equals signs aligned.
func terraformResource(kind, label string, arguments [][2]string) string {
	width := 0
	for _, argument := range arguments {
		width = max(width, len(argument[0]))
	}
	var block strings.Builder
	fmt.Fprintf(&block, "resource %q %q {\n", kind, label)
	for _, argument := range arguments {
		fmt.Fprintf(&block, "  %-*s = %s\n", width, argument[0], argument[1])
	}
	block.WriteString("}")
	return block.String()
}

// terraformLabel returns a resource label for record: its name and type, with
// every character but letters, digits and underscores made an underscore,
// and an underscore before a leading digit, which a label cannot start with.
func terraformLabel(record DNSRecord) string {
	label := strings.Map(func(r rune) rune {
		if r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_') {
			return r
		}
		return '_'
	}, strings.TrimSuffix(record.Name, ".")+"_"+strings.ToLower(record.Type))
	if label[0] >= '0' && label[0] <= '9' {
		label = "_" + label
	}
	return label
}

// uniqueLabel returns label, or label with the first free counter of _2, _3
// and so on when used holds it, and adds the result to used.
func uniqueLabel(label string, used map[string]bool) string {
	unique := label
	for n := 2; used[unique]; n++ {
		unique = fmt.Sprintf("%s_%d", label, n)
	}
	used[unique] = true
	return unique
}

// hclString returns s as an HCL string literal: quoted, with its quotes,
// backslashes and line breaks escaped, and its ${ and %{ sequences kept from
// starting a template.
func hclString(s string) string {
	return `"` + strings.NewReplacer(
		`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "${", "$${", "%{", "%%{",
	).Replace(s) + `"`
}
