// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metainfo"
)

// TestLoop does the following
// * upload 5 remote files with 1 segment
// * (TODO) upload 3 remote files with 2 segments
// * upload 2 inline files
// * connect two observers to the metainfo loop
// * run the metainfo loop
// * expect that each observer has seen
//    - 5 remote files
//    - 5 remote segments
//    - 2 inline files/segments
//    - 7 unique path items
func TestLoop(t *testing.T) {
	// TODO: figure out how to configure testplanet so we can upload 2*segmentSize to get two segments
	segmentSize := 8 * memory.KiB

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.Loop.CoalesceDuration = 1 * time.Second
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		metaLoop := satellite.Metainfo.Loop

		// upload 5 remote files with 1 segment
		for i := 0; i < 5; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// (TODO) upload 3 remote files with 2 segments
		// for i := 0; i < 3; i++ {
		// 	testData := testrand.Bytes(2 * segmentSize)
		// 	path := "/some/other/remote/path/" + string(i)
		// 	err := ul.Upload(ctx, satellite, "bucket", path, testData)
		// 	require.NoError(t, err)
		// }

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(segmentSize / 8)
			path := "/some/inline/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create 2 observers
		obs1 := newTestObserver(nil)
		obs2 := newTestObserver(nil)

		var group errgroup.Group
		group.Go(func() error {
			return metaLoop.Join(ctx, obs1)
		})
		group.Go(func() error {
			return metaLoop.Join(ctx, obs2)
		})

		err := group.Wait()
		require.NoError(t, err)

		projectID := ul.ProjectID[satellite.ID()]
		for _, obs := range []*testObserver{obs1, obs2} {
			assert.EqualValues(t, 7, obs.objectCount)
			assert.EqualValues(t, 5, obs.remoteSegCount)
			assert.EqualValues(t, 2, obs.inlineSegCount)
			assert.EqualValues(t, 7, len(obs.uniquePaths))
			for _, path := range obs.uniquePaths {
				assert.EqualValues(t, path.BucketName, "bucket")
				assert.EqualValues(t, path.ProjectID, projectID)
			}
		}
	})
}

// TestLoopObserverCancel does the following:
// * upload 3 remote segments
// * hook three observers up to metainfo loop
// * let observer 1 run normally
// * let observer 2 return an error from one of its handlers
// * let observer 3's context be canceled
// * expect observer 1 to see all segments
// * expect observers 2 and 3 to finish with errors
func TestLoopObserverCancel(t *testing.T) {
	segmentSize := 8 * memory.KiB

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.Loop.CoalesceDuration = 1 * time.Second
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		metaLoop := satellite.Metainfo.Loop

		// upload 3 remote files with 1 segment
		for i := 0; i < 3; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create 1 "good" observer
		obs1 := newTestObserver(nil)

		// create observer that will return an error from RemoteSegment
		obs2 := newTestObserver(func(ctx context.Context) error {
			return errors.New("test error")
		})

		// create observer that will cancel its own context from RemoteSegment
		obs3Ctx, cancel := context.WithCancel(ctx)
		var once int64
		obs3 := newTestObserver(func(ctx context.Context) error {
			if atomic.AddInt64(&once, 1) == 1 {
				cancel()
				<-obs3Ctx.Done() // ensure we wait for cancellation to propagate
			} else {
				panic("multiple calls to observer after loop cancel")
			}
			return nil
		})

		var group errgroup.Group
		group.Go(func() error {
			return metaLoop.Join(ctx, obs1)
		})
		group.Go(func() error {
			err := metaLoop.Join(ctx, obs2)
			if err == nil {
				return errors.New("got no error")
			}
			if !strings.Contains(err.Error(), "test error") {
				return errors.New("expected to find error")
			}
			return nil
		})
		group.Go(func() error {
			err := metaLoop.Join(obs3Ctx, obs3)
			if !errs2.IsCanceled(err) {
				return errors.New("expected canceled")
			}
			return nil
		})

		err := group.Wait()
		require.NoError(t, err)

		// expect that obs1 saw all three segments, but obs2 and obs3 only saw the first one
		assert.EqualValues(t, 3, obs1.remoteSegCount)
		assert.EqualValues(t, 1, obs2.remoteSegCount)
		assert.EqualValues(t, 1, obs3.remoteSegCount)
	})
}

// TestLoopCancel does the following:
// * upload 3 remote segments
// * hook two observers up to metainfo loop
// * cancel loop context partway through
// * expect both observers to exit with an error and see fewer than 3 remote segments
// * expect that a new observer attempting to join at this point receives a loop closed error
func TestLoopCancel(t *testing.T) {
	segmentSize := 8 * memory.KiB

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// upload 3 remote files with 1 segment
		for i := 0; i < 3; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + string(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create a new metainfo loop
		metaLoop := metainfo.NewLoop(metainfo.LoopConfig{
			CoalesceDuration: 1 * time.Second,
		}, satellite.Metainfo.Database)

		// create a cancelable context to pass into metaLoop.Run
		loopCtx, cancel := context.WithCancel(ctx)

		// create 1 normal observer
		obs1 := newTestObserver(nil)

		var once int64
		// create another normal observer that will wait before returning during RemoteSegment so we can sync with context cancelation
		obs2 := newTestObserver(func(ctx context.Context) error {
			// cancel context during call to obs2.RemoteSegment inside loop
			if atomic.AddInt64(&once, 1) == 1 {
				cancel()
				<-ctx.Done() // ensure we wait for cancellation to propagate
			} else {
				panic("multiple calls to observer after loop cancel")
			}
			return nil
		})

		var group errgroup.Group

		// start loop with cancelable context
		group.Go(func() error {
			err := metaLoop.Run(loopCtx)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})
		group.Go(func() error {
			err := metaLoop.Join(ctx, obs1)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})
		group.Go(func() error {
			err := metaLoop.Join(ctx, obs2)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})

		err := group.Wait()
		require.NoError(t, err)

		err = metaLoop.Close()
		require.NoError(t, err)

		obs3 := newTestObserver(nil)
		err = metaLoop.Join(ctx, obs3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loop closed")

		// expect that obs1 and obs2 each saw fewer than three remote segments
		assert.True(t, obs1.remoteSegCount < 3)
		assert.True(t, obs2.remoteSegCount < 3)
	})
}

type testObserver struct {
	objectCount    int
	remoteSegCount int
	inlineSegCount int
	uniquePaths    map[string]metainfo.ScopedPath
	onSegment      func(context.Context) error // if set, run this during RemoteSegment()
}

func newTestObserver(onSegment func(context.Context) error) *testObserver {
	return &testObserver{
		objectCount:    0,
		remoteSegCount: 0,
		inlineSegCount: 0,
		uniquePaths:    make(map[string]metainfo.ScopedPath),
		onSegment:      onSegment,
	}
}

func (obs *testObserver) RemoteSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	obs.remoteSegCount++

	if _, ok := obs.uniquePaths[path.Raw]; ok {
		// TODO: collect the errors and check in test
		panic("Expected unique path in observer.RemoteSegment")
	}
	obs.uniquePaths[path.Raw] = path

	if obs.onSegment != nil {
		return obs.onSegment(ctx)
	}

	return nil
}

func (obs *testObserver) Object(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	obs.objectCount++
	return nil
}

func (obs *testObserver) InlineSegment(ctx context.Context, path metainfo.ScopedPath, pointer *pb.Pointer) error {
	obs.inlineSegCount++
	if _, ok := obs.uniquePaths[path.Raw]; ok {
		// TODO: collect the errors and check in test
		panic("Expected unique path in observer.InlineSegment")
	}
	obs.uniquePaths[path.Raw] = path
	return nil
}
