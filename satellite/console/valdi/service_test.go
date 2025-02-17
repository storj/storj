// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package valdi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/httpmock"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
)

var validURL = "http://localhost:1234"

func TestNewService(t *testing.T) {
	mockClient, _ := httpmock.NewClient()

	vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
		APIBaseURL: validURL,
	})
	require.NoError(t, err)

	t.Run("invalid email", func(t *testing.T) {
		s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
			SatelliteEmail: "invalidEmail",
		}, vClient)
		require.Error(t, err)
		require.True(t, valdi.ErrEmail.Has(err))
		require.Nil(t, s)
	})
	t.Run("nil client", func(t *testing.T) {
		s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
			SatelliteEmail: "satellite@storj.test",
		}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "valdi client cannot be nil")
		require.Nil(t, s)
	})
	t.Run("success", func(t *testing.T) {
		s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
			SatelliteEmail: "satellite@storj.test",
		}, vClient)
		require.NoError(t, err)
		require.NotNil(t, s)
	})
}

func TestCreateUserEmail(t *testing.T) {
	satelliteEmail := "satellite@storj.test"
	projectID := testrand.UUID()

	expect := fmt.Sprintf("satellite+%s@storj.test", projectID.String())

	mockClient, _ := httpmock.NewClient()

	vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
		APIBaseURL: validURL,
	})
	require.NoError(t, err)

	s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
		SatelliteEmail: satelliteEmail,
	}, vClient)
	require.NoError(t, err)

	require.Equal(t, expect, s.CreateUserEmail(projectID))
}

func TestServiceCreateAPIKey(t *testing.T) {
	ctx := testcontext.New(t)

	mockClient, transport := httpmock.NewClient()

	vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
		APIBaseURL: validURL,
	})
	require.NoError(t, err)

	s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
		SatelliteEmail: "satellite@storj.test",
	}, vClient)
	require.NoError(t, err)

	id := testrand.UUID()

	keySuccess := &valdiclient.CreateAPIKeyResponse{
		APIKey:            "1234",
		SecretAccessToken: "abc123",
	}

	jsonKey, err := json.Marshal(keySuccess)
	require.NoError(t, err)

	apiKeyEndpoint, err := url.JoinPath(validURL, valdiclient.APIKeyPath)
	require.NoError(t, err)

	transport.AddResponse(apiKeyEndpoint, httpmock.Response{
		StatusCode: http.StatusOK,
		Body:       string(jsonKey),
	})

	apiKey, status, err := s.CreateAPIKey(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, keySuccess, apiKey)

	valdiErr := &valdiclient.ErrorMessage{
		Detail: "user not found",
	}

	jsonErr, err := json.Marshal(valdiErr)
	require.NoError(t, err)

	transport.AddResponse(apiKeyEndpoint, httpmock.Response{
		StatusCode: http.StatusNotFound,
		Body:       string(jsonErr),
	})

	apiKey, status, err = s.CreateAPIKey(ctx, id)
	require.Error(t, err)
	require.Contains(t, err.Error(), valdiErr.Detail)
	require.Equal(t, http.StatusNotFound, status)
	require.Nil(t, apiKey)
}

func TestServiceCreateUser(t *testing.T) {
	ctx := testcontext.New(t)

	mockClient, transport := httpmock.NewClient()

	vClient, err := valdiclient.New(zaptest.NewLogger(t), mockClient, valdiclient.Config{
		APIBaseURL: validURL,
	})
	require.NoError(t, err)

	s, err := valdi.NewService(zaptest.NewLogger(t), valdi.Config{
		SatelliteEmail: "satellite@storj.test",
	}, vClient)
	require.NoError(t, err)

	id := testrand.UUID()

	userEndpoint, err := url.JoinPath(validURL, valdiclient.UserPath)
	require.NoError(t, err)

	transport.AddResponse(userEndpoint, httpmock.Response{
		StatusCode: http.StatusCreated,
	})

	status, err := s.CreateUser(ctx, id)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	valdiErr := &valdiclient.ErrorMessage{
		Detail: "username already exists",
	}

	jsonErr, err := json.Marshal(valdiErr)
	require.NoError(t, err)

	transport.AddResponse(userEndpoint, httpmock.Response{
		StatusCode: http.StatusConflict,
		Body:       string(jsonErr),
	})

	status, err = s.CreateUser(ctx, id)
	require.Error(t, err)
	require.Contains(t, err.Error(), valdiErr.Detail)
	require.Equal(t, http.StatusConflict, status)
}
