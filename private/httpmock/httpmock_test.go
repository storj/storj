// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package httpmock_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/httpmock"
)

var url = "https://example.test"

func TestAddResponseAndRoundTrip(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	client, transport := httpmock.NewClient()

	// Add a mock response
	response := httpmock.Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: `{"message": "success"}`,
	}
	transport.AddResponse(url, response)

	// Make a request to the mock URL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer require.NoError(t, resp.Body.Close())

	// Validate the response
	require.Equal(t, response.StatusCode, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, response.Body, strings.TrimSpace(string(body)))
	require.Equal(t, response.Headers["Content-Type"], resp.Header.Get("Content-Type"))
}

func TestDefaultNotFoundResponse(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	client, _ := httpmock.NewClient()

	// Make a request to a URL without a mock response
	unknownPath := url + "/unknown"
	req, err := http.NewRequestWithContext(testcontext.New(t), http.MethodGet, unknownPath, nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer require.NoError(t, resp.Body.Close())

	// Validate the default 404 response
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", strings.TrimSpace(string(body)))
}

func TestSequentialResponses(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	client, transport := httpmock.NewClient()

	// Add multiple responses for the same URL
	transport.AddResponse(url, httpmock.Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"message": "first response"}`,
	})
	transport.AddResponse(url, httpmock.Response{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       `{"message": "second response"}`,
	})

	// First request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer require.NoError(t, resp.Body.Close())
	require.Equal(t, 200, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message": "first response"}`, strings.TrimSpace(string(body)))

	// Second request
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer require.NoError(t, resp.Body.Close())
	require.Equal(t, 201, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message": "second response"}`, strings.TrimSpace(string(body)))

	// Third request (no more responses, expect 404)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	require.NoError(t, err)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer require.NoError(t, resp.Body.Close())
	require.Equal(t, 404, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, "Not Found", strings.TrimSpace(string(body)))
}

func TestThreadSafety(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	_, transport := httpmock.NewClient()

	response := httpmock.Response{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "text/plain"},
		Body:       "OK",
	}

	// Use a goroutine to add responses concurrently
	go func() {
		for i := 0; i < 100; i++ {
			transport.AddResponse(url, response)
		}
	}()

	// Simulate concurrent requests
	for i := 0; i < 100; i++ {
		go func() {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			require.NoError(t, err)

			resp, err := transport.RoundTrip(req)
			require.NoError(t, err)
			if resp != nil {
				defer require.NoError(t, resp.Body.Close())
			}
		}()
	}
}
