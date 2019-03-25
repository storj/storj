// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/inspector"
	"storj.io/storj/storage"
)

func TestInspectorStats(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	uplink := planet.Uplinks[0]
	testData := make([]byte, 1*memory.MiB)
	_, err = rand.Read(testData)
	assert.NoError(t, err)

	bucket := "testbucket"

	err = uplink.Upload(ctx, planet.Satellites[0], bucket, "test/path", testData)
	assert.NoError(t, err)

	log := zaptest.NewLogger(t)

	health, err := inspector.NewEndpoint(log, planet.Satellites[0].Overlay.Service, planet.Satellites[0].Metainfo.Service)
	assert.NoError(t, err)

	// Get path of random segment we just uploaded and check the health
	_ = planet.Satellites[0].Metainfo.Database.Iterate(storage.IterateOptions{Recurse: true, Reverse: false},
		func(it storage.Iterator) error {
			var item storage.ListItem
			it.Next(&item)

			req := &pb.SegmentHealthRequest{
				EncryptedPath: []byte(item.Key.String()),
				Bucket:        []byte(bucket),
				Segment:       0,
			}

			resp, err := health.SegmentStat(ctx, req)
			assert.NoError(t, err)

			assert.Equal(t, int32(0), resp.GetSuccessThreshold())
			assert.Equal(t, int32(1), resp.GetMinimumRequired())
			assert.Equal(t, int32(4), resp.GetTotal())
			assert.Equal(t, int32(0), resp.GetRepairThreshold())
			assert.Equal(t, int32(4), resp.GetOnlineNodes())

			return nil
		},
	)

}
