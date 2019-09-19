// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

// initiate graceful exit doesn't explode
func TestInitiateGracefulExitDoesNotExplode(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx context.Context, t *testing.T, db storagenode.DB) {
		assert.NoError(t, db.Satellites().InitiateGracefulExit(ctx, storj.NodeID{}, time.Now(), 5000))
	})
}

// increment graceful exit bytes deleted doesn't explode
func TestUpdateGracefulExitDoesNotExplode(t *testing.T) { // satelliteID storj.NodeID, bytesDeleted int64) (err error) {
	storagenodedbtest.Run(t, func(ctx context.Context, t *testing.T, db storagenode.DB) {
		assert.NoError(t, db.Satellites().InitiateGracefulExit(ctx, storj.NodeID{}, time.Now(), 5000))
		assert.NoError(t, db.Satellites().UpdateGracefulExit(ctx, storj.NodeID{}, 1000))
	})
}

// complete graceful exit doesn't explode
func TestCompleteGracefulExitDoesNotExplode(t *testing.T) { //satelliteID storj.NodeID, finishedAt time.Time, exitStatus satelliteStatus, completionReceipt []byte) (err error) {
	storagenodedbtest.Run(t, func(ctx context.Context, t *testing.T, db storagenode.DB) {
		assert.NoError(t, db.Satellites().InitiateGracefulExit(ctx, storj.NodeID{}, time.Now(), 5000))
		assert.NoError(t, db.Satellites().CompleteGracefulExit(ctx, storj.NodeID{}, time.Now(), satellites.ExitedOk, []byte{0, 0, 0}))
	})
}
