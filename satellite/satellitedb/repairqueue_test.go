// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/storage"
)

func TestRepairQueue(t *testing.T) {
	satellitedbtest.Run(t, func(t *testing.T, db satellite.DB) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		repairQueue := db.RepairQueue()

		// insert a bunch of segments
		pathsMap := make(map[string]int)
		for i := 0; i < 100; i++ {
			path := "/path/" + string(i)
			injuredSeg := &pb.InjuredSegment{Path: []byte(path)}
			err := repairQueue.Insert(ctx, injuredSeg)
			require.NoError(t, err)
			pathsMap[path] = 0
		}

		// select segments until no more are returned, and we should get each one exactly once
		for {
			injuredSeg, err := repairQueue.Select(ctx)
			if err != nil {
				require.True(t, storage.ErrEmptyQueue.Has(err))
				break
			}
			pathsMap[string(injuredSeg.Path)]++
		}

		for _, selectCount := range pathsMap {
			assert.Equal(t, selectCount, 1)
		}
	})
}
