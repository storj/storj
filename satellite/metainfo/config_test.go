// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
)

func TestRSConfigValidation(t *testing.T) {
	tests := []struct {
		description    string
		configString   string
		expectedConfig metainfo.RSConfig
		expectError    bool
	}{
		{
			description:  "valid rs config",
			configString: "4/8/10/20-256B",
			expectedConfig: metainfo.RSConfig{
				ErasureShareSize: 256 * memory.B, Min: 4, Repair: 8, Success: 10, Total: 20,
			},
			expectError: false,
		},
		{
			description:  "invalid rs config - numbers decrease",
			configString: "4/8/5/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - starts at 0",
			configString: "0/2/4/6-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - strings",
			configString: "4/a/b/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - floating-point numbers",
			configString: "4/5.2/7/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - not enough items",
			configString: "4/5/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - too many items",
			configString: "4/5/20/30/50-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty numbers",
			configString: "-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty size",
			configString: "1/2/3/4-",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty",
			configString: "",
			expectError:  true,
		},
		{
			description:  "invalid valid rs config - invalid share size",
			configString: "4/8/10/20-256A",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Log(tt.description)

		rsConfig := metainfo.RSConfig{}
		err := rsConfig.Set(tt.configString)
		if tt.expectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedConfig.ErasureShareSize, rsConfig.ErasureShareSize)
			require.EqualValues(t, tt.expectedConfig.Min, rsConfig.Min)
			require.EqualValues(t, tt.expectedConfig.Repair, rsConfig.Repair)
			require.EqualValues(t, tt.expectedConfig.Success, rsConfig.Success)
			require.EqualValues(t, tt.expectedConfig.Total, rsConfig.Total)
		}
	}
}

func TestUUIDsFlag(t *testing.T) {
	var UUIDs metainfo.UUIDsFlag
	err := UUIDs.Set("")
	require.NoError(t, err)
	require.Len(t, UUIDs, 0)

	testIDA := testrand.UUID()
	err = UUIDs.Set(testIDA.String())
	require.NoError(t, err)
	require.Equal(t, metainfo.UUIDsFlag{
		testIDA: {},
	}, UUIDs)

	testIDB := testrand.UUID()
	err = UUIDs.Set(testIDA.String() + "," + testIDB.String())
	require.NoError(t, err)
	require.Equal(t, metainfo.UUIDsFlag{
		testIDA: {},
		testIDB: {},
	}, UUIDs)
}

func TestMigrationModeFlag(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Debug.Addr = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		debugAddr := planet.Satellites[0].API.Debug.Listener.Addr().String()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/metainfo/flags/migration-mode", debugAddr), &bytes.Buffer{})
		require.NoError(t, err)

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, res.Body.Close())
		}()

		require.Equal(t, http.StatusOK, res.StatusCode)
		content, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, "false", string(content))

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://%s/metainfo/flags/migration-mode", debugAddr), bytes.NewBufferString("true"))
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()

		// we are rejecting write requests
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(1*memory.KiB))
		require.True(t, errs2.IsRPC(err, rpcstatus.ResourceExhausted))

		// download should still work
		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "testobject")
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://%s/metainfo/flags/migration-mode", debugAddr), bytes.NewBufferString("false"))
		require.NoError(t, err)

		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()

		// uploads are working again
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "testobject", testrand.Bytes(1*memory.KiB))
		require.NoError(t, err)
	})
}
