// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package segmentloop_test

import (
	"context"
	"errors"
	"strconv"
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
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/segmentloop"
)

// TestSegmentsLoop does the following
// * upload 5 remote files with 1 segment
// * upload 2 remote files with 2 segments
// * upload 2 inline files
// * connect two observers to the segments loop
// * run the segments loop.
func TestSegmentsLoop(t *testing.T) {
	segmentSize := 50 * memory.KiB

	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.SegmentLoop.CoalesceDuration = 1 * time.Second
				config.Metainfo.MaxSegmentSize = segmentSize
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]
		segmentLoop := satellite.Metainfo.SegmentLoop

		// upload 5 remote objects with 1 segment
		for i := 0; i < 5; i++ {
			testData := testrand.Bytes(8 * memory.KiB)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 2 remote objects with 2 segment each
		for i := 0; i < 2; i++ {
			// exact 2*segmentSize will make inline segment at the end of object
			testData := testrand.Bytes(2*segmentSize - 1000)
			path := "/some/other/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// upload 2 inline files
		for i := 0; i < 2; i++ {
			testData := testrand.Bytes(1 * memory.KiB)
			path := "/some/inline/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create 2 observers
		obs1 := newTestObserver(nil)
		obs2 := newTestObserver(nil)

		var group errgroup.Group
		group.Go(func() error {
			return segmentLoop.Join(ctx, obs1)
		})
		group.Go(func() error {
			return segmentLoop.Join(ctx, obs2)
		})

		err := group.Wait()
		require.NoError(t, err)

		for _, obs := range []*testObserver{obs1, obs2} {
			assert.EqualValues(t, 9, obs.remoteSegCount)
			assert.EqualValues(t, 2, obs.inlineSegCount)
			assert.EqualValues(t, 11, len(obs.uniqueKeys))
		}
	})
}

func TestSegmentsLoop_AllData(t *testing.T) {
	segmentSize := 8 * memory.KiB
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 4,
		UplinkCount:      3,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Metainfo.SegmentLoop.CoalesceDuration = 1 * time.Second
				config.Metainfo.SegmentLoop.ListLimit = 2
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		bucketNames := strings.Split("abc", "")

		data := testrand.Bytes(segmentSize)
		for _, up := range planet.Uplinks {
			for _, bucketName := range bucketNames {
				err := up.Upload(ctx, planet.Satellites[0], "zzz"+bucketName, "1", data)
				require.NoError(t, err)
			}
		}

		loop := planet.Satellites[0].Metainfo.SegmentLoop

		obs := newTestObserver(nil)
		err := loop.Join(ctx, obs)
		require.NoError(t, err)

		gotItems := len(obs.uniqueKeys)
		require.Equal(t, len(bucketNames)*len(planet.Uplinks), gotItems)
	})
}

// TestsegmentsLoopObserverCancel does the following:
// * upload 3 remote segments
// * hook three observers up to segments loop
// * let observer 1 run normally
// * let observer 2 return an error from one of its handlers
// * let observer 3's context be canceled
// * expect observer 1 to see all segments
// * expect observers 2 and 3 to finish with errors.
func TestSegmentsLoopObserverCancel(t *testing.T) {
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
		loop := satellite.Metainfo.SegmentLoop

		// upload 3 remote files with 1 segment
		for i := 0; i < 3; i++ {
			testData := testrand.Bytes(segmentSize)
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		// create 1 "good" observer
		obs1 := newTestObserver(nil)
		mon1 := newTestObserver(nil)

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
			return loop.Join(ctx, obs1)
		})
		group.Go(func() error {
			return loop.Monitor(ctx, mon1)
		})
		group.Go(func() error {
			err := loop.Join(ctx, obs2)
			if err == nil {
				return errors.New("got no error")
			}
			if !strings.Contains(err.Error(), "test error") {
				return errors.New("expected to find error")
			}
			return nil
		})
		group.Go(func() error {
			err := loop.Join(obs3Ctx, obs3)
			if !errs2.IsCanceled(err) {
				return errors.New("expected canceled")
			}
			return nil
		})

		err := group.Wait()
		require.NoError(t, err)

		// expect that obs1 saw all three segments, but obs2 and obs3 only saw the first one
		assert.EqualValues(t, 3, obs1.remoteSegCount)
		assert.EqualValues(t, 3, mon1.remoteSegCount)
		assert.EqualValues(t, 1, obs2.remoteSegCount)
		assert.EqualValues(t, 1, obs3.remoteSegCount)
	})
}

// TestSegmentsLoopCancel does the following:
// * upload 3 remote segments
// * hook two observers up to segments loop
// * cancel loop context partway through
// * expect both observers to exit with an error and see fewer than 3 remote segments
// * expect that a new observer attempting to join at this point receives a loop closed error.
func TestSegmentsLoopCancel(t *testing.T) {
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
			path := "/some/remote/path/" + strconv.Itoa(i)
			err := ul.Upload(ctx, satellite, "bucket", path, testData)
			require.NoError(t, err)
		}

		loop := segmentloop.New(segmentloop.Config{
			CoalesceDuration: 1 * time.Second,
			ListLimit:        10000,
		}, satellite.Metainfo.Metabase)

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
			err := loop.Run(loopCtx)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})
		group.Go(func() error {
			err := loop.Join(ctx, obs1)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})
		group.Go(func() error {
			err := loop.Join(ctx, obs2)
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})

		err := group.Wait()
		require.NoError(t, err)

		err = loop.Close()
		require.NoError(t, err)

		obs3 := newTestObserver(nil)
		err = loop.Join(ctx, obs3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loop closed")

		// expect that obs1 and obs2 each saw fewer than three remote segments
		assert.True(t, obs1.remoteSegCount < 3)
		assert.True(t, obs2.remoteSegCount < 3)
	})
}

func TestSegmentsLoop_MonitorCancel(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		loop := segmentloop.New(segmentloop.Config{
			CoalesceDuration: time.Nanosecond,
			ListLimit:        10000,
		}, satellite.Metainfo.Metabase)

		obs1 := newTestObserver(func(ctx context.Context) error {
			return errors.New("test error")
		})

		var group errgroup.Group

		loopCtx, loopCancel := context.WithCancel(ctx)
		group.Go(func() error {
			err := loop.Run(loopCtx)
			t.Log("segments loop stopped")
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})

		obsCtx, obsCancel := context.WithCancel(ctx)
		group.Go(func() error {
			defer loopCancel()
			err := loop.Monitor(obsCtx, obs1)
			t.Log("observer stopped")
			if !errs2.IsCanceled(err) {
				return errors.New("expected context canceled")
			}
			return nil
		})

		obsCancel()

		err := group.Wait()
		require.NoError(t, err)

		err = loop.Close()
		require.NoError(t, err)
	})
}

type testKey struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
}

type testObserver struct {
	remoteSegCount int
	inlineSegCount int
	uniqueKeys     map[testKey]struct{}
	onSegment      func(context.Context) error // if set, run this during RemoteSegment()
}

func newTestObserver(onSegment func(context.Context) error) *testObserver {
	return &testObserver{
		remoteSegCount: 0,
		inlineSegCount: 0,
		uniqueKeys:     make(map[testKey]struct{}),
		onSegment:      onSegment,
	}
}

// LoopStarted is called at each start of a loop.
func (obs *testObserver) LoopStarted(ctx context.Context, info segmentloop.LoopInfo) (err error) {
	return nil
}

func (obs *testObserver) RemoteSegment(ctx context.Context, segment *segmentloop.Segment) error {
	obs.remoteSegCount++

	key := testKey{
		StreamID: segment.StreamID,
		Position: segment.Position,
	}
	if _, ok := obs.uniqueKeys[key]; ok {
		// TODO: collect the errors and check in test
		panic("Expected unique pair StreamID/Position in observer.RemoteSegment")
	}
	obs.uniqueKeys[key] = struct{}{}

	if obs.onSegment != nil {
		return obs.onSegment(ctx)
	}

	return nil
}

func (obs *testObserver) InlineSegment(ctx context.Context, segment *segmentloop.Segment) error {
	obs.inlineSegCount++
	key := testKey{
		StreamID: segment.StreamID,
		Position: segment.Position,
	}
	if _, ok := obs.uniqueKeys[key]; ok {
		// TODO: collect the errors and check in test
		panic("Expected unique pair StreamID/Position in observer.InlineSegment")
	}
	obs.uniqueKeys[key] = struct{}{}
	return nil
}
