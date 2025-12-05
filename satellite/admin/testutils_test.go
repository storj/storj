// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

func assertGet(ctx context.Context, t *testing.T, link string, expected string, authToken string) {
	t.Helper()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", authToken)

	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	require.Equal(t, http.StatusOK, response.StatusCode, string(data))
	require.Equal(t, expected, string(data))
}

// assertReq asserts the request and it's OK it returns the response body.
func assertReq(
	ctx *testcontext.Context, t *testing.T, link string, method string, body string,
	expectedStatus int, expectedBody string, authToken string, queryParams ...[2]string,
) []byte {
	t.Helper()

	var (
		req *http.Request
		err error
	)
	if body == "" {
		req, err = http.NewRequestWithContext(ctx, method, link, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, link, strings.NewReader(body))
	}
	require.NoError(t, err)

	req.Header.Set("Authorization", authToken)
	req.Header.Set("Content-Type", "application/json")

	query := req.URL.Query()
	for _, q := range queryParams {
		query.Add(q[0], q[1])
	}

	req.URL.RawQuery = query.Encode()

	res, err := http.DefaultClient.Do(req) //nolint:bodyclose
	require.NoError(t, err)
	defer ctx.Check(res.Body.Close)

	require.Equal(t, expectedStatus, res.StatusCode, "response status code")

	resBody, err := io.ReadAll(res.Body)
	if expectedBody != "" {
		require.NoError(t, err)
		require.Equal(t, expectedBody, string(resBody), "response body")
	}

	if len(resBody) > 0 {
		require.Equal(t, "application/json", res.Header.Get("Content-Type"))
	} else {
		require.Equal(t, "", res.Header.Get("Content-Type"))
	}

	return resBody
}
