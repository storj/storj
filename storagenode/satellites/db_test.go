// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package satellites_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestSatellitesDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		satellitesDB := db.Satellites()
		id := testrand.NodeID()

		err := satellitesDB.SetAddress(ctx, id, "test_addr1")
		require.NoError(t, err)

		satellites, err := satellitesDB.GetSatellitesUrls(ctx)
		require.NoError(t, err)
		require.Equal(t, satellites[0].Address, "test_addr1")

		err = satellitesDB.SetAddress(ctx, id, "test_addr2")
		require.NoError(t, err)

		satellites, err = satellitesDB.GetSatellitesUrls(ctx)
		require.NoError(t, err)
		require.Equal(t, satellites[0].Address, "test_addr2")
	})
}
