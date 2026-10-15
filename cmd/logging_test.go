package cmd

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AhaSend/ahasend-go/api"
	"github.com/AhaSend/ahasend-go/models/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AhaSend/ahasend-cli/internal/mocks"
	"github.com/AhaSend/ahasend-cli/internal/testutil"
)

func TestDebugLogsGoToStderrInJSONMode(t *testing.T) {
	const body = `{"message":"OK"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	testutil.WriteProfiles(t, testutil.UseNewHome(t), server.URL, rawTestID, "default")

	stdout, stderr, code := runRoot(t, "ping", "--debug", "--output", "json")

	assert.Equal(t, 0, code)
	assert.Equal(t, testutil.IndentedJSON(t, body), stdout)
	assert.Contains(t, stderr, "HTTP Response")
	assert.NotContains(t, stderr, "aha-sk-default")
	assert.NotContains(t, stderr, "\x1b[", "no colors on a non-terminal stderr")
}

func TestFailedItemsPrintNoLogLinesUnlessAsked(t *testing.T) {
	mockClient := &mocks.MockClient{}
	mockClient.On("GetAccountID").Return(rawTestID)
	mockClient.On("CancelMessage", rawTestID).Return(&common.SuccessResponse{Message: "Canceled"}, nil)
	mockClient.On("CancelMessage", rawTestOtherID).Return(nil, &api.APIError{StatusCode: 404, Message: "not found"})
	testutil.InjectClient(t, mockClient)

	_, stderr, code := runRoot(t, "messages", "cancel", rawTestID, rawTestOtherID, "--force")

	assert.Equal(t, 1, code)
	assert.Equal(t, "1 of 2 messages failed\n", stderr)
}

func TestVerboseLogsOneAPICallPerRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/accounts/"+rawTestID+"/messages" {
			_, _ = w.Write([]byte(`{"object":"list","data":[],"pagination":{"has_more":false}}`))
			return
		}
		_, _ = w.Write([]byte(`{"object":"message","id":"` + rawTestOtherID + `"}`))
	}))
	t.Cleanup(server.Close)

	for name, args := range map[string][]string{
		"messages list": {"messages", "list"},
		"messages get":  {"messages", "get", rawTestOtherID},
	} {
		t.Run(name, func(t *testing.T) {
			testutil.WriteProfiles(t, testutil.UseNewHome(t), server.URL, rawTestID, "default")

			_, stderr, code := runRoot(t, append(args, "--verbose", "--output", "json")...)

			assert.Equal(t, 0, code)
			assert.Equal(t, 1, strings.Count(stderr, "API Call"), stderr)
		})
	}
}

func TestDebugLogsNeverShowTheDKIMPrivateKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	keyLine := strings.Split(keyPEM, "\n")[1]
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"domain","domain":"example.com"}`))
	}))
	t.Cleanup(server.Close)
	testutil.WriteProfiles(t, testutil.UseNewHome(t), server.URL, rawTestID, "default")

	_, stderr, code := runRoot(t, "domains", "create", "example.com", "--no-dns-help", "--debug",
		"--dkim-private-key-file", testutil.WriteTempFile(t, "dkim.pem", keyPEM))

	assert.Equal(t, 0, code)
	assert.Contains(t, received, keyLine, "the API gets the key")
	assert.Contains(t, stderr, "Request Body")
	assert.NotContains(t, stderr, keyLine)
}
