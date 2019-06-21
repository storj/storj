// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package linksharing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"gotest.tools/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

func TestHandler(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 1,
		UplinkCount:      1,
	}, testHandler)
}

func testHandler(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
	err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/foo", []byte("FOO"))
	require.NoError(t, err)

	apiKey, err := uplink.ParseAPIKey(planet.Uplinks[0].APIKey[planet.Satellites[0].ID()])
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
		body   string
	}{
		{
			name:   "success",
			path:   path.Join(scope, "testbucket", "test/foo"),
			status: http.StatusOK,
			body:   "FOO",
		},
		{
			name:   "invalid method",
			method: "PUT",
			status: http.StatusMethodNotAllowed,
			body:   "method not allowed\n",
		},
		{
			name:   "missing scope",
			status: http.StatusBadRequest,
			body:   "invalid request: missing scope\n",
		},
		{
			name:   "malformed scope",
			path:   path.Join("BADSCOPE", "testbucket", "test/foo"),
			status: http.StatusBadRequest,
			body:   "invalid request: invalid scope format\n",
		},
		{
			name:   "missing bucket",
			path:   scope,
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket\n",
		},
		{
			name:   "bucket not found",
			path:   path.Join(scope, "someotherbucket", "test/foo"),
			status: http.StatusNotFound,
			body:   "bucket not found\n",
		},
		{
			name:   "missing bucket path",
			path:   path.Join(scope, "testbucket"),
			status: http.StatusBadRequest,
			body:   "invalid request: missing bucket path\n",
		},
		{
			name:   "object not found",
			path:   path.Join(scope, "testbucket", "test/bar"),
			status: http.StatusNotFound,
			body:   "object not found\n",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			handler := NewHandler(zaptest.NewLogger(t), newUplink(ctx, t))
			url := "http://localhost/" + testCase.path
			w := httptest.NewRecorder()
			r, err := http.NewRequest(testCase.method, url, nil)
			require.NoError(t, err)
			handler.ServeHTTP(w, r)

			assert.Equal(t, testCase.status, w.Code, "status code does not match")
			assert.Equal(t, testCase.body, w.Body.String(), "body does not match")
		})
	}
}

func newUplink(ctx context.Context, tb testing.TB) *uplink.Uplink {
	cfg := new(uplink.Config)
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	up, err := uplink.NewUplink(ctx, cfg)
	require.NoError(tb, err)
	return up
}
