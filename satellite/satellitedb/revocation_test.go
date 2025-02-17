// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestRevocation(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {

		check, err := db.Revocation().Check(ctx, [][]byte{
			[]byte("tail1"),
			[]byte("tail2"),
			[]byte("tail3"),
		})
		require.NoError(t, err)
		require.False(t, check)

		err = db.Revocation().Revoke(ctx, []byte("tail1"), []byte("api"))
		require.NoError(t, err)

		check, err = db.Revocation().Check(ctx, [][]byte{
			[]byte("tail1"),
			[]byte("tail2"),
			[]byte("tail3"),
		})
		require.NoError(t, err)
		require.True(t, check)

	})
}
