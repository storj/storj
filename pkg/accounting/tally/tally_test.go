// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/storj"
)

func TestDeleteRawBefore(t *testing.T) {
	tests := []struct {
		createdAt    time.Time
		eraseBefore  time.Time
		expectedRaws int
	}{
		{
			createdAt:    time.Now(),
			eraseBefore:  time.Now(),
			expectedRaws: 1,
		},
		{
			createdAt:    time.Now(),
			eraseBefore:  time.Now().Add(24 * time.Hour),
			expectedRaws: 0,
		},
	}

	for _, tt := range tests {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			id := teststorj.NodeIDFromBytes([]byte{})
			nodeData := make(map[storj.NodeID]float64)
			nodeData[id] = float64(1000)

			err := planet.Satellites[0].DB.Accounting().SaveAtRestRaw(ctx, tt.createdAt, tt.createdAt, nodeData)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.Accounting().DeleteRawBefore(ctx, tt.eraseBefore)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.Accounting().GetRaw(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, tt.expectedRaws)
		})
	}
}
