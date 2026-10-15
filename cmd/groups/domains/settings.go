package domains

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"regexp"
	"strings"

	"github.com/AhaSend/ahasend-cli/internal/cmdutil"
	"github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/validation"
	"github.com/spf13/cobra"
)

// domainSettings are the settings domains create and update both send; a
// nil field is left out of the request.
type domainSettings struct {
	TrackingSubdomain        *string
	ReturnPathSubdomain      *string
	SubscriptionSubdomain    *string
	MediaSubdomain           *string
	DKIMRotationIntervalDays *int
	DKIMSelector             *string
	SendingType              *string
}

// empty reports whether no setting was given.
func (s domainSettings) empty() bool {
	return s == domainSettings{}
}

// addSettingsFlags adds the subdomain and DKIM rotation flags of domains
// create and update. Each command adds --dkim-selector and --sending-type
// itself, since their help differs.
func addSettingsFlags(cmd *cobra.Command) {
	cmd.Flags().String("tracking-subdomain", "", "Custom tracking subdomain")
	cmd.Flags().String("return-path-subdomain", "", "Custom return-path subdomain")
	cmd.Flags().String("subscription-subdomain", "", "Custom subscription management subdomain")
	cmd.Flags().String("media-subdomain", "", "Custom media subdomain")
	cmd.Flags().Int("dkim-rotation-interval", 0, "DKIM rotation interval in days (managed DNS domains on eligible plans only)")
}

// readSettings returns the settings flags given, checking the subdomains,
// --dkim-selector and --sending-type.
func readSettings(cmd *cobra.Command) (domainSettings, error) {
	settings := domainSettings{
		TrackingSubdomain:     cmdutil.ChangedString(cmd, "tracking-subdomain"),
		ReturnPathSubdomain:   cmdutil.ChangedString(cmd, "return-path-subdomain"),
		SubscriptionSubdomain: cmdutil.ChangedString(cmd, "subscription-subdomain"),
		MediaSubdomain:        cmdutil.ChangedString(cmd, "media-subdomain"),
		DKIMSelector:          cmdutil.ChangedString(cmd, "dkim-selector"),
	}
	if cmd.Flags().Changed("dkim-rotation-interval") {
		days, _ := cmd.Flags().GetInt("dkim-rotation-interval")
		settings.DKIMRotationIntervalDays = &days
	}
	labels := []struct {
		flag  string
		value *string
		rule  labelRule
	}{
		{"tracking-subdomain", settings.TrackingSubdomain, subdomainRule},
		{"return-path-subdomain", settings.ReturnPathSubdomain, subdomainRule},
		{"subscription-subdomain", settings.SubscriptionSubdomain, subdomainRule},
		{"media-subdomain", settings.MediaSubdomain, subdomainRule},
		{"dkim-selector", settings.DKIMSelector, dkimSelectorRule},
	}
	for _, label := range labels {
		if label.value != nil {
			if err := checkLabel(label.flag, *label.value, label.rule); err != nil {
				return domainSettings{}, err
			}
		}
	}
	sendingType, err := cmdutil.ReadOneOf(cmd, "sending-type")
	if err != nil {
		return domainSettings{}, err
	}
	settings.SendingType = sendingType
	return settings, nil
}

// labelRule is how the API takes a setting that is one DNS label: it
// lowercases the value and trims its spaces, and with TrimDots its dots too,
// and then takes 1 to MaxLength letters, digits and hyphens, without a
// leading or trailing hyphen. EmptyOK says whether a value that is empty
// after that means the default; otherwise the API refuses it.
type labelRule struct {
	MaxLength int
	TrimDots  bool
	EmptyOK   bool
}

var (
	// subdomainRule is the rule of the subdomain settings, which cannot be
	// empty.
	subdomainRule = labelRule{MaxLength: 63, TrimDots: true}
	// dkimSelectorRule is the rule of --dkim-selector, which is empty to use
	// the default selector.
	dkimSelectorRule = labelRule{MaxLength: 62, EmptyOK: true}
)

// checkLabel checks value, the value of the label setting flag, against rule.
func checkLabel(flag, value string, rule labelRule) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if rule.TrimDots {
		normalized = strings.Trim(normalized, ".")
	}
	if normalized == "" {
		if rule.EmptyOK {
			return nil
		}
		return errors.NewValidationError("--"+flag+" cannot be empty", nil)
	}
	if isDNSLabel(normalized, rule.MaxLength) {
		return nil
	}
	return validation.InvalidValueError("--"+flag, value, fmt.Sprintf(
		"must be one DNS label of 1-%d letters, digits and hyphens, without a leading or trailing hyphen", rule.MaxLength))
}

// labelPattern is a lowercase DNS label without a leading or trailing
// hyphen, of any length.
var labelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

// isDNSLabel reports whether s is a lowercase DNS label of at most maxLength
// characters.
func isDNSLabel(s string, maxLength int) bool {
	return len(s) <= maxLength && labelPattern.MatchString(s)
}

// minDKIMKeyBits is the smallest RSA key the API takes as a DKIM private key.
const minDKIMKeyBits = 1024

// readDKIMPrivateKey returns the PEM RSA private key in the file
// --dkim-private-key-file names, or nil when the flag is not set.
func readDKIMPrivateKey(cmd *cobra.Command) (*string, error) {
	path, _ := cmd.Flags().GetString("dkim-private-key-file")
	if path == "" {
		return nil, nil
	}
	data, err := cmdutil.ReadInputFile("dkim-private-key-file", path)
	if err != nil {
		return nil, err
	}
	key, err := parseRSAPrivateKey(data)
	if err != nil {
		return nil, errors.NewValidationError(fmt.Sprintf("invalid --dkim-private-key-file %s: %v", path, err), nil)
	}
	if bits := key.N.BitLen(); bits < minDKIMKeyBits {
		return nil, errors.NewValidationError(fmt.Sprintf(
			"invalid --dkim-private-key-file %s: the key has %d bits; the API takes RSA keys of at least %d", path, bits, minDKIMKeyBits), nil)
	}
	// The whole file is sent, so it must hold nothing but the key
	if !isOnePEMBlock(data) {
		return nil, errors.NewValidationError(fmt.Sprintf("--dkim-private-key-file %s must hold one private key", path), nil)
	}
	value := string(data)
	return &value, nil
}

// isOnePEMBlock reports whether data holds one PEM block and nothing else
// but whitespace around it.
func isOnePEMBlock(data []byte) bool {
	block, rest := pem.Decode(data)
	return block != nil && len(bytes.TrimSpace(rest)) == 0 &&
		bytes.HasPrefix(bytes.TrimSpace(data), []byte("-----BEGIN "))
}

// parseRSAPrivateKey parses the first PEM block of data as an RSA private
// key in PKCS #1 or PKCS #8 form.
func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM private key found")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("not an RSA private key")
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an RSA private key")
	}
	return key, nil
}
