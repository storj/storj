// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
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

	userID := "user@mail.example"

	endpoint := NewEndpoint(zaptest.NewLogger(t), newTestAuthDB(t, ctx), nil)
	require.NotNil(t, endpoint)

	res, err := endpoint.Create(ctx, &pb.AuthorizationRequest{UserId: userID})
	require.NoError(t, err)
	require.NotNil(t, res)

	token, err := ParseToken(res.Token)
	require.NoError(t, err)
	require.NotNil(t, token)

	require.Equal(t, userID, token.UserID)
}

func TestEndpoint_Run(t *testing.T) {
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
		statusCode int
	}{
		{
			"create success",
			"user@mail.example",
			"/v1/authorization/create",
			201,
		},
		{
			"missing user ID",
			"",
			"/v1/authorization/create",
			400,
		},
		{
			"not found",
			"",
			"/",
			404,
		},
	}

	for _, testCase := range testCases {
		url := baseURL + testCase.urlPath
		res, err := http.Post(
			url, "application/json",
			bytes.NewBuffer([]byte(testCase.userID)),
		)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, testCase.statusCode, res.StatusCode)
	}
}
