// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package valdiclient_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/httpmock"
	"storj.io/storj/satellite/console/valdi/valdiclient"
)

func generateKey(t *testing.T, ctx *testcontext.Context) (f *os.File, path string) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)

	privateKeyPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	keyPath := ctx.File("key.pem")
	keyFile, err := os.Create(keyPath)
	require.NoError(t, err)

	require.NoError(t, pem.Encode(keyFile, privateKeyPEM))

	return keyFile, keyPath
}

var validURL = "http://localhost:1234"

func TestNew(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   "://example",
			SignRequests: true,
		})
		require.Error(t, err)
		require.True(t, valdiclient.ErrAPIURL.Has(err))
		require.Contains(t, err.Error(), "invalid APIBaseURL")
		require.Nil(t, c)
	})
	t.Run("url is not http", func(t *testing.T) {
		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   "abc://localhost:1234",
			SignRequests: true,
		})
		require.Error(t, err)
		require.True(t, valdiclient.ErrAPIURL.Has(err))
		require.Contains(t, err.Error(), "APIBaseURL must be http or https")
		require.Nil(t, c)
	})
	t.Run("url has no host", func(t *testing.T) {
		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   "http://",
			SignRequests: true,
		})
		require.Error(t, err)
		require.True(t, valdiclient.ErrAPIURL.Has(err))
		require.Contains(t, err.Error(), "APIBaseURL must have a host")
		require.Nil(t, c)
	})
	t.Run("valid api url, doesn't sign requests", func(t *testing.T) {
		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL: validURL,
		})
		require.NoError(t, err)
		require.NotNil(t, c)
	})
	t.Run("can't read key", func(t *testing.T) {
		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   validURL,
			SignRequests: true,
		})
		require.Error(t, err)
		require.True(t, valdiclient.ErrPrivateKey.Has(err))
		require.Contains(t, err.Error(), "failed to read key file")
		require.Nil(t, c)
	})
	t.Run("can't parse key data", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		keyPath := ctx.File("key.pem")
		keyFile, err := os.Create(keyPath)
		require.NoError(t, err)
		ctx.Check(keyFile.Close)

		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   validURL,
			SignRequests: true,
			RSAKeyPath:   keyPath,
		})
		require.Error(t, err)
		require.True(t, valdiclient.ErrPrivateKey.Has(err))
		require.Contains(t, err.Error(), "failed to parse key data")
		require.Nil(t, c)
	})
	t.Run("valid api url, signs requests", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		keyFile, keyPath := generateKey(t, ctx)
		defer ctx.Check(keyFile.Close)

		c, err := valdiclient.New(zaptest.NewLogger(t), http.DefaultClient, valdiclient.Config{
			APIBaseURL:   validURL,
			SignRequests: true,
			RSAKeyPath:   keyPath,
		})
		require.NoError(t, err)
		require.NotNil(t, c)
	})
}

func TestCreateUser(t *testing.T) {
	userEndpoint, err := url.JoinPath(validURL, valdiclient.UserPath)
	require.NoError(t, err)

	mockClient, transport := httpmock.NewClient()

	createUser := valdiclient.UserCreationData{
		Email:    "test@storj.test",
		Username: "teststorj",
		Country:  "USA",
	}

	userJSONData, err := json.Marshal(createUser)
	require.NoError(t, err)

	valdiErr := valdiclient.ErrorMessage{
		Detail: "valdi error message",
	}

	errJSONData, err := json.Marshal(valdiErr)
	require.NoError(t, err)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	keyFile, keyPath := generateKey(t, ctx)
	defer ctx.Check(keyFile.Close)

	keyPaths := []string{"", keyPath}

	for _, kp := range keyPaths {
		signRequests := kp != ""

		testSuffix := ""
		if signRequests {
			testSuffix = " with request signing"
		}

		vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
			APIBaseURL:   validURL,
			SignRequests: signRequests,
			RSAKeyPath:   kp,
		})
		require.NoError(t, err)

		t.Run("returns no error when status 201"+testSuffix, func(t *testing.T) {
			expectStatus := http.StatusCreated
			transport.AddResponse(userEndpoint, httpmock.Response{
				StatusCode: expectStatus,
				Body:       string(userJSONData),
			})

			status, err := vClient.CreateUser(testcontext.New(t), createUser)
			require.NoError(t, err)
			require.Equal(t, expectStatus, status)
		})
		t.Run("returns error when status not 201"+testSuffix, func(t *testing.T) {
			expectStatus := http.StatusConflict
			transport.AddResponse(userEndpoint, httpmock.Response{
				StatusCode: expectStatus,
				Body:       string(errJSONData),
			})

			status, err := vClient.CreateUser(testcontext.New(t), createUser)
			require.Error(t, err)
			require.Contains(t, err.Error(), valdiErr.Detail)
			require.Equal(t, expectStatus, status)
		})
	}
}

func TestCreateAPIKey(t *testing.T) {
	apiKeysEndpoint, err := url.JoinPath(validURL, valdiclient.APIKeyPath)
	require.NoError(t, err)

	mockClient, transport := httpmock.NewClient()

	keyResp := valdiclient.CreateAPIKeyResponse{
		APIKey:            "1234",
		SecretAccessToken: "abc123",
	}

	keyJSONData, err := json.Marshal(keyResp)
	require.NoError(t, err)

	valdiErr := valdiclient.ErrorMessage{
		Detail: "valdi error message",
	}

	errJSONData, err := json.Marshal(valdiErr)
	require.NoError(t, err)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	keyFile, keyPath := generateKey(t, ctx)
	defer ctx.Check(keyFile.Close)

	keyPaths := []string{"", keyPath}

	for _, kp := range keyPaths {
		signRequests := kp != ""

		testSuffix := ""
		if signRequests {
			testSuffix = " with request signing"
		}

		vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
			APIBaseURL:   validURL,
			SignRequests: signRequests,
			RSAKeyPath:   kp,
		})
		require.NoError(t, err)

		t.Run("returns api key when status 200"+testSuffix, func(t *testing.T) {
			expectStatus := http.StatusOK
			transport.AddResponse(apiKeysEndpoint, httpmock.Response{
				StatusCode: expectStatus,
				Body:       string(keyJSONData),
			})

			apiKey, status, err := vClient.CreateAPIKey(ctx, "test@storj.test")
			require.NoError(t, err)
			require.Equal(t, expectStatus, status)
			require.NotNil(t, apiKey)
			require.Equal(t, keyResp, *apiKey)
		})
		t.Run("returns client error when status not 200"+testSuffix, func(t *testing.T) {
			expectStatus := http.StatusForbidden
			transport.AddResponse(apiKeysEndpoint, httpmock.Response{
				StatusCode: expectStatus,
				Body:       string(errJSONData),
			})

			apiKey, status, err := vClient.CreateAPIKey(ctx, "test@storj.test")
			require.Error(t, err)
			require.Contains(t, err.Error(), valdiErr.Detail)
			require.Equal(t, expectStatus, status)
			require.Nil(t, apiKey)
		})
	}
}
