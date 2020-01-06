// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/errs2"
	"storj.io/common/testcontext"
)

func TestEndpoint_Run_httpSuccess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, listener)

	log := zaptest.NewLogger(t)
	authDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authDB.Close)

	service := NewService(log, authDB)
	endpoint := NewEndpoint(log, service, listener)
	require.NotNil(t, endpoint)

	ctx.Go(func() error {
		return errs2.IgnoreCanceled(endpoint.Run(ctx))
	})
	defer ctx.Check(endpoint.Close)

	userID := "user@mail.test"
	url := "http://" + listener.Addr().String() + "/v1/authorizations/" + userID
	req, err := http.NewRequest(http.MethodPut, url, nil)
	require.NoError(t, err)
	require.NotNil(t, req)

	client := http.Client{}
	res, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, res)

	require.Equal(t, http.StatusCreated, res.StatusCode)

	tokenBytes, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.NotEmpty(t, tokenBytes)
	require.NoError(t, res.Body.Close())

	token, err := ParseToken(string(tokenBytes))
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Equal(t, userID, token.UserID)
}

func TestEndpoint_Run_httpErrors(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, listener)

	log := zaptest.NewLogger(t)
	authDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authDB.Close)

	service := NewService(log, authDB)
	endpoint := NewEndpoint(log, service, listener)
	require.NotNil(t, endpoint)

	ctx.Go(func() error {
		return errs2.IgnoreCanceled(endpoint.Run(ctx))
	})
	defer ctx.Check(endpoint.Close)

	baseURL := "http://" + listener.Addr().String()

	testCases := []struct {
		name       string
		userID     string
		urlPath    string
		httpMethod string
		statusCode int
	}{
		{
			"missing user ID",
			"",
			"/v1/authorizations/",
			http.MethodPut,
			http.StatusUnprocessableEntity,
		},
		{
			"unsupported http method (GET)",
			"user@mail.test",
			"/v1/authorizations/",
			http.MethodGet,
			http.StatusMethodNotAllowed,
		},
		{
			"unsupported http method (PUT)",
			"user@mail.test",
			"/v1/authorizations/",
			http.MethodPost,
			http.StatusMethodNotAllowed,
		},
		{
			"not found",
			"",
			"/",
			http.MethodPut,
			http.StatusNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Log(testCase.name)
		url := baseURL + testCase.urlPath + testCase.userID
		req, err := http.NewRequest(testCase.httpMethod, url, nil)
		require.NoError(t, err)

		client := http.Client{}
		res, err := client.Do(req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.NoError(t, res.Body.Close())

		require.Equal(t, testCase.statusCode, res.StatusCode)
	}
}
