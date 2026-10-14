package domains

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/models/requests"
	"github.com/AhaSend/ahasend-go/models/responses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	clierrors "github.com/AhaSend/ahasend-cli/internal/errors"
	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

// smallRSAKey is a 512-bit RSA key, too small for a DKIM key. It is test data
// only.
const smallRSAKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBOgIBAAJBAL2JkIa/pNDII6qq7Et+6CocFZ4+hxB4AqMwoy70DH3MTTAXQxtE
9S1SvIzAZc3V/WJqmCCNF8xAz13uzDluFzkCAwEAAQJAPrIW/yO8kp1pguZoJG71
wBwrIH3Bcran9LA9gmYTDbcpVyGU//e8nXenc9FIoNFR7tst95egSxdypTmN0FXT
UwIhAOrubhLdm2HkADLZb5P9uaOR+DG/1wGSgdXwdunIVukPAiEAzoj52QsI4l/V
PXlfU9cFA2+Kmrq/VaYXEnRhTKubqzcCIQC4gGoH/OxD07joNmVvd0ssj7247G2h
uXqoFwJ8la7TtwIgXt4OHYLzDsBEO5t/jQwsElruyInVDyteV6y7cLM2qtsCIAd0
IoD3Qi1Qj2YI54M13PydS1NYUOVlwo7EGvi4nNT0
-----END RSA PRIVATE KEY-----
`

// pkcs8PEM returns key as a PKCS #8 PEM block.
func pkcs8PEM(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestCreateSendsTheSettingsUpdateTakes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	keyPEM := pkcs8PEM(t, key)
	mockClient := &mocks.MockClient{}
	var request requests.CreateDomainRequest
	testutil.CaptureRequest(mockClient, "CreateDomain", &request, &responses.Domain{Domain: "example.com"})
	testutil.InjectClient(t, mockClient)

	_, err = testutil.RunPlain(t, NewCreateCommand(), "example.com", "--no-dns-help",
		"--tracking-subdomain", "click", "--return-path-subdomain", "bounce",
		"--subscription-subdomain", "prefs", "--media-subdomain", "media",
		"--dkim-rotation-interval", "45", "--dkim-selector", "News",
		"--dkim-private-key-file", testutil.WriteTempFile(t, "dkim.pem", keyPEM))

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"domain":                      "example.com",
		"tracking_subdomain":          "click",
		"return_path_subdomain":       "bounce",
		"subscription_subdomain":      "prefs",
		"media_subdomain":             "media",
		"dkim_rotation_interval_days": float64(45),
		"dkim_selector":               "News",
		"dkim_private_key":            keyPEM,
	}, testutil.WireJSON(t, request))
}

func TestCreateRejectsADKIMKeyTheAPIDoesNotTakeBeforeAuthenticating(t *testing.T) {
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tests := map[string]struct {
		content string
		reason  string
	}{
		"not PEM":      {"not a key", "no PEM private key found"},
		"not RSA":      {pkcs8PEM(t, ecKey), "not an RSA private key"},
		"too few bits": {smallRSAKey, "the key has 512 bits; the API takes RSA keys of at least 1024"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)
			path := testutil.WriteTempFile(t, "dkim.pem", test.content)

			_, err := testutil.RunPlain(t, NewCreateCommand(), "example.com", "--dkim-private-key-file", path)

			require.Error(t, err)
			assert.Equal(t, "invalid --dkim-private-key-file "+path+": "+test.reason, err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}

func TestCreateRejectsADKIMKeyFileWithMoreThanTheKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	keyPEM := pkcs8PEM(t, key)
	other, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	tests := map[string]string{
		"a second key":        keyPEM + pkcs8PEM(t, other),
		"text after the key":  keyPEM + "notes\n",
		"text before the key": "notes\n" + keyPEM,
		"a certificate after": keyPEM + string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("x")})),
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			called := testutil.RefuseAuth(t)
			path := testutil.WriteTempFile(t, "dkim.pem", content)

			_, err := testutil.RunPlain(t, NewCreateCommand(), "example.com", "--dkim-private-key-file", path)

			require.Error(t, err)
			assert.Equal(t, "--dkim-private-key-file "+path+" must hold one private key", err.Error())
			assert.Equal(t, 4, clierrors.GetExitCode(err))
			assert.False(t, *called)
		})
	}
}

func TestCreateTakesADKIMKeyWithWhitespaceAroundIt(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	content := " \n\n" + pkcs8PEM(t, key) + "\n \t\n"
	mockClient := &mocks.MockClient{}
	var request requests.CreateDomainRequest
	testutil.CaptureRequest(mockClient, "CreateDomain", &request, &responses.Domain{Domain: "example.com"})
	testutil.InjectClient(t, mockClient)

	_, err = testutil.RunPlain(t, NewCreateCommand(), "example.com", "--no-dns-help",
		"--dkim-private-key-file", testutil.WriteTempFile(t, "dkim.pem", content))

	require.NoError(t, err)
	assert.Equal(t, content, testutil.WireJSON(t, request)["dkim_private_key"])
}

func TestDKIMSelectorsTheAPITakesPass(t *testing.T) {
	for _, selector := range []string{"news", "News-2026", "a", "", "  ", "s1"} {
		assert.NoError(t, checkLabel("dkim-selector", selector, dkimSelectorRule), selector)
	}
}

func TestSubdomainsTheAPITakesPass(t *testing.T) {
	for _, subdomain := range []string{"click", "Click", " t ", "t.", "a-b", strings.Repeat("a", 63)} {
		assert.NoError(t, checkLabel("tracking-subdomain", subdomain, subdomainRule), subdomain)
	}
}

// TestEmptySubdomainsAreRejectedBeforeAuthenticating checks the subdomain
// values the API refuses as empty, which it trims of spaces and dots.
func TestEmptySubdomainsAreRejectedBeforeAuthenticating(t *testing.T) {
	for _, flag := range []string{"tracking-subdomain", "return-path-subdomain", "subscription-subdomain", "media-subdomain"} {
		for _, value := range []string{"", " ", "."} {
			for name, command := range map[string]func() *cobra.Command{"create": NewCreateCommand, "update": NewUpdateCommand} {
				t.Run(name+" --"+flag+"="+value, func(t *testing.T) {
					called := testutil.RefuseAuth(t)

					_, err := testutil.RunPlain(t, command(), "example.com", "--"+flag, value)

					require.Error(t, err)
					assert.Equal(t, "--"+flag+" cannot be empty", err.Error())
					assert.Equal(t, 4, clierrors.GetExitCode(err))
					assert.False(t, *called)
				})
			}
		}
	}
}
