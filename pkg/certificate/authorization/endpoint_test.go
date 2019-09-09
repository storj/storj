// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
)

func TestEndpoint_Create(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, listener)

	authorizationDB := newTestAuthDB(t, ctx)
	endpoint := NewEndpoint(zaptest.NewLogger(t), authorizationDB, listener)
	require.NotNil(t, endpoint)

	{ // new user, no existing authorization tokens
		userID := "new@mail.test"
		group, err := authorizationDB.Get(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, group)

		res, err := endpoint.Create(ctx, &pb.AuthorizationRequest{UserId: userID})
		require.NoError(t, err)
		require.NotNil(t, res)

		token, err := ParseToken(res.Token)
		require.NoError(t, err)
		require.NotNil(t, token)

		require.Equal(t, userID, token.UserID)
	}

	{ // existing user with unclaimed authorization token
		userID := "old@mail.test"
		group, err := authorizationDB.Create(ctx, userID, 1)
		require.NoError(t, err)
		require.NotEmpty(t, group)
		require.Len(t, group, 1)

		existingAuth := group[0]

		res, err := endpoint.Create(ctx, &pb.AuthorizationRequest{UserId: userID})
		require.NoError(t, err)
		require.NotNil(t, res)

		token, err := ParseToken(res.Token)
		require.NoError(t, err)
		require.NotNil(t, token)

		require.Equal(t, userID, token.UserID)
		require.Equal(t, existingAuth.Token, *token)
	}
}

func TestEndpoint_Run_httpSuccess(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, listener)

	endpoint := NewEndpoint(zaptest.NewLogger(t), newTestAuthDB(t, ctx), listener)
	require.NotNil(t, endpoint)

	ctx.Go(func() error {
		return errs2.IgnoreCanceled(endpoint.Run(ctx))
	})
	defer ctx.Check(endpoint.Close)

	userID := "user@mail.test"
	url := "http://" + listener.Addr().String() + "/v1/authorization"
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBufferString(userID))
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

	endpoint := NewEndpoint(zaptest.NewLogger(t), newTestAuthDB(t, ctx), listener)
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
			"/v1/authorization",
			http.MethodPut,
			http.StatusUnprocessableEntity,
		},
		{
			"unsupported http method (GET)",
			"user@mail.test",
			"/v1/authorization",
			http.MethodGet,
			http.StatusMethodNotAllowed,
		},
		{
			"unsupported http method (PUT)",
			"user@mail.test",
			"/v1/authorization",
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
		url := baseURL + testCase.urlPath
		req, err := http.NewRequest(
			testCase.httpMethod, url,
			bytes.NewBufferString(testCase.userID),
		)
		require.NoError(t, err)

		client := http.Client{}
		res, err := client.Do(req)
		require.NoError(t, err)
		require.NotNil(t, res)
		require.NoError(t, res.Body.Close())

		require.Equal(t, testCase.statusCode, res.StatusCode)
	}
}
