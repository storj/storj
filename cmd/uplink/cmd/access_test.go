// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/private/testplanet"
)

func TestRegisterAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// mock the auth service
		ts := httptest.NewServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, `{"access_key_id":"1", "secret_key":"2", "endpoint":"3"}`)
			}))
		defer ts.Close()
		// make sure we get back things
		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		accessKey, secretKey, endpoint, err := cmd.RegisterAccess(access, ts.URL, true)
		require.NoError(t, err)
		assert.Equal(t, "1", accessKey)
		assert.Equal(t, "2", secretKey)
		assert.Equal(t, "3", endpoint)
	})
}
