// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
)

func TestMetainfoLoop(t *testing.T) {
	// upload 5 remote files with 1 segment
	// upload 3 remote files with 2 segments
	// upload 2 inline files
	// connect two observers to the metainfo loop
	// run the metainfo loop
	// expect that each observer has seen 8 remote files, 11 remote segments, and 2 inline files/segments
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfoloop.CoalesceDuration = 1 * time.Second
				config.Metainfoloop.Interval = 1 * time.Second
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		metaLoop := satellite.Metainfoloop.Service

		// upload 5 remote files with 1 segment
		for i := 0; i < 5; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 3 remote files with 2 segments
		for i := 0; i < 3; i++ {
			testData := testrand.Bytes(16 * memory.KiB)
			path := "/some/other/remote/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 2 inline files
		for i := 0; i < 3; i++ {
			testData := testrand.Bytes(1 * memory.KiB)
			path := "/some/inline/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create 2 observers
		obs1 := newTestObserver(t)
		obs2 := newTestObserver(t)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			err := metaLoop.Join(ctx, obs1)
			require.NoError(t, err)
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs2)
			require.NoError(t, err)
			wg.Done()
		}()

		wg.Wait()
		for _, obs := range []*testObserver{obs1, obs2} {
			assert.EqualValues(t, obs.remoteSegCount, 11)
			assert.EqualValues(t, obs.remoteFileCount, 8)
			assert.EqualValues(t, obs.inlineSegCount, 2)
			assert.EqualValues(t, len(obs.uniquePaths), 13)
		}
	})
}

type testObserver struct {
	remoteSegCount  int
	remoteFileCount int
	inlineSegCount  int
	uniquePaths     map[string]struct{}
	t               *testing.T
}

func newTestObserver(t *testing.T) *testObserver {
	return &testObserver{
		remoteSegCount:  0,
		remoteFileCount: 0,
		inlineSegCount:  0,
		uniquePaths:     make(map[string]struct{}),
		t:               t,
	}
}

func (obs *testObserver) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) error {
	obs.remoteSegCount++
	if _, ok := obs.uniquePaths[path]; ok {
		obs.t.Error("Expected unique path in observer.RemoteSegment")
	}
	obs.uniquePaths[path] = struct{}{}
	return nil
}

func (obs *testObserver) RemoteObject(ctx context.Context, path storj.Path, pointer *pb.Pointer) error {
	obs.remoteFileCount++
	return nil
}

func (obs *testObserver) InlineSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) error {
	obs.inlineSegCount++
	if _, ok := obs.uniquePaths[path]; ok {
		obs.t.Error("Expected unique path in observer.InlineSegment")
	}
	obs.uniquePaths[path] = struct{}{}
	return nil
}
