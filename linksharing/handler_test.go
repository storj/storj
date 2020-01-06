// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package linksharing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/testplanet"
)

func TestNewHandler(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	uplink := newUplink(ctx, t)
	defer ctx.Check(uplink.Close)

	testCases := []struct {
		name   string
		config HandlerConfig
		err    string
	}{
		{
			name: "missing uplink",
			config: HandlerConfig{
				URLBase: "http://localhost",
			},
			err: "uplink is required",
		},
		{
			name: "URL base must be http or https",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "gopher://chunks",
			},
			err: "URL base must be http:// or https://",
		},
		{
			name: "URL base must contain host",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://",
			},
			err: "URL base must contain host",
		},
		{
			name: "URL base can have a port",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://host:99",
			},
		},
		{
			name: "URL base can have a path",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://host/gopher",
			},
		},
		{
			name: "URL base must not contain user info",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://joe@host",
			},
			err: "URL base must not contain user info",
		},
		{
			name: "URL base must not contain query values",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://host/?gopher=chunks",
			},
			err: "URL base must not contain query values",
		},
		{
			name: "URL base must not contain a fragment",
			config: HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://host/#gopher-chunks",
			},
			err: "URL base must not contain a fragment",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {

			handler, err := NewHandler(zaptest.NewLogger(t), testCase.config)
			if testCase.err != "" {
				require.EqualError(t, err, testCase.err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, handler)
		})
	}
}

func TestHandlerRequests(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, testHandlerRequests)
}

func testHandlerRequests(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/foo", []byte("FOO"))
	require.NoError(t, err)

	apiKey, err := uplink.ParseAPIKey(planet.Uplinks[0].APIKey[planet.Satellites[0].ID()].Serialize())
	require.NoError(t, err)

	scope, err := (&uplink.Scope{
		SatelliteAddr:    planet.Satellites[0].Addr(),
		APIKey:           apiKey,
		EncryptionAccess: uplink.NewEncryptionAccessWithDefaultKey(storj.Key{}),
	}).Serialize()
	require.NoError(t, err)

	testCases := []struct {
		name   string
		method string
		path   string
		status int
		header http.Header
		body   string
	}{
		{
			name:   "invalid method",
			method: "PUT",
			status: http.StatusMethodNotAllowed,
			body:   "method not allowed\n",
		},
		{
			name:   "GET missing scope",
			method: "GET",
			status: http.StatusBadRequest,
			body:   "invalid request: missing scope\n",
		},
		{
			name:   "GET malformed scope",
			method: "GET",
			path:   path.Join("BADSCOPE", "testbucket", "test/foo"),
			status: http.StatusBadRequest,
			body:   "invalid request: invalid scope format\n",
		},
		{
			name:   "GET missing bucket",
			method: "GET",
			path:   scope,
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket\n",
		},
		{
			name:   "GET bucket not found",
			method: "GET",
			path:   path.Join(scope, "someotherbucket", "test/foo"),
			status: http.StatusNotFound,
			body:   "bucket not found\n",
		},
		{
			name:   "GET missing bucket path",
			method: "GET",
			path:   path.Join(scope, "testbucket"),
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket path\n",
		},
		{
			name:   "GET object not found",
			method: "GET",
			path:   path.Join(scope, "testbucket", "test/bar"),
			status: http.StatusNotFound,
			body:   "object not found\n",
		},
		{
			name:   "GET success",
			method: "GET",
			path:   path.Join(scope, "testbucket", "test/foo"),
			status: http.StatusOK,
			body:   "FOO",
		},
		{
			name:   "HEAD missing scope",
			method: "HEAD",
			status: http.StatusBadRequest,
			body:   "invalid request: missing scope\n",
		},
		{
			name:   "HEAD malformed scope",
			method: "HEAD",
			path:   path.Join("BADSCOPE", "testbucket", "test/foo"),
			status: http.StatusBadRequest,
			body:   "invalid request: invalid scope format\n",
		},
		{
			name:   "HEAD missing bucket",
			method: "HEAD",
			path:   scope,
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket\n",
		},
		{
			name:   "HEAD bucket not found",
			method: "HEAD",
			path:   path.Join(scope, "someotherbucket", "test/foo"),
			status: http.StatusNotFound,
			body:   "bucket not found\n",
		},
		{
			name:   "HEAD missing bucket path",
			method: "HEAD",
			path:   path.Join(scope, "testbucket"),
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket path\n",
		},
		{
			name:   "HEAD object not found",
			method: "HEAD",
			path:   path.Join(scope, "testbucket", "test/bar"),
			status: http.StatusNotFound,
			body:   "object not found\n",
		},
		{
			name:   "HEAD success",
			method: "HEAD",
			path:   path.Join(scope, "testbucket", "test/foo"),
			status: http.StatusFound,
			header: http.Header{
				"Location": []string{"http://localhost/" + path.Join(scope, "testbucket", "test/foo")},
			},
			body: "",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			uplink := newUplink(ctx, t)
			defer ctx.Check(uplink.Close)

			handler, err := NewHandler(zaptest.NewLogger(t), HandlerConfig{
				Uplink:  uplink,
				URLBase: "http://localhost",
			})
			require.NoError(t, err)

			url := "http://localhost/" + testCase.path
			w := httptest.NewRecorder()
			r, err := http.NewRequest(testCase.method, url, nil)
			require.NoError(t, err)
			handler.ServeHTTP(w, r)

			assert.Equal(t, testCase.status, w.Code, "status code does not match")
			for h, v := range testCase.header {
				assert.Equal(t, v, w.Header()[h], "%q header does not match", h)
			}
			assert.Equal(t, testCase.body, w.Body.String(), "body does not match")
		})
	}
}

func newUplink(ctx context.Context, tb testing.TB) *uplink.Uplink {
	cfg := new(uplink.Config)
	cfg.Volatile.Log = zaptest.NewLogger(tb)
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	up, err := uplink.NewUplink(ctx, cfg)
	require.NoError(tb, err)
	return up
}
