// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"errors"
	"fmt"
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
	"storj.io/storj/satellite/metainfo"
)

// TestMetainfoLoop does the following
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
func TestMetainfoLoop(t *testing.T) {
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
		obs1 := newTestObserver(t, nil)
		obs2 := newTestObserver(t, nil)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			err := metaLoop.Join(ctx, obs1)
			assert.NoError(t, err)
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs2)
			assert.NoError(t, err)
			wg.Done()
		}()

		wg.Wait()
		for _, obs := range []*testObserver{obs1, obs2} {
			assert.EqualValues(t, 5, obs.remoteSegCount)
			assert.EqualValues(t, 5, obs.remoteFileCount)
			assert.EqualValues(t, 2, obs.inlineSegCount)
			assert.EqualValues(t, 7, len(obs.uniquePaths))
		}
	})
}

// TestMetainfoLoopObserverCancel does the following:
// * upload 3 remote segments
// * hook three observers up to metainfo loop
// * let observer 1 run normally
// * let observer 2 return an error from one of its handlers
// * let observer 3's context be canceled
// * expect observer 1 to see all segments
// * expect observers 2 and 3 to finish with errors
func TestMetainfoLoopObserverCancel(t *testing.T) {
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
		obs1 := newTestObserver(t, nil)

		// create observer that will return an error from RemoteSegment
		obs2 := newTestObserver(t, func() error {
			return errors.New("test error")
		})

		// create observer that will cancel its own context from RemoteSegment
		obs3Ctx, cancel := context.WithCancel(ctx)
		obs3 := newTestObserver(t, func() error {
			cancel()
			return nil
		})

		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			err := metaLoop.Join(ctx, obs1)
			assert.NoError(t, err)
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs2)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "test error")
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(obs3Ctx, obs3)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
			wg.Done()
		}()

		wg.Wait()

		// expect that obs1 saw all three segments, but obs2 and obs3 only saw the first one
		assert.EqualValues(t, 3, obs1.remoteSegCount)
		assert.EqualValues(t, 1, obs2.remoteSegCount)
		assert.EqualValues(t, 1, obs3.remoteSegCount)
	})
}

// TestMetainfoLoopCancel does the following:
// * upload 3 remote segments
// * hook two observers up to metainfo loop
// * cancel loop context partway through
// * expect both observers to exit with an error and see fewer than 3 remote segments
// * expect that a new observer attempting to join at this point receives a loop closed error
func TestMetainfoLoopCancel(t *testing.T) {
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
		}, satellite.Metainfo.Service)
		// create a cancelable context to pass into metaLoop.Run
		loopCtx, cancel := context.WithCancel(ctx)

		// create 1 normal observer
		obs1 := newTestObserver(t, nil)

		// create another normal observer that will wait before returning during RemoteSegment so we can sync with context cancelation
		obs2 := newTestObserver(t, func() error {
			// cancel context during call to obs2.RemoteSegment inside loop
			fmt.Println("WE ARE CANCELING THE Context")
			cancel()
			return nil
		})

		var wg sync.WaitGroup
		wg.Add(3)

		// start loop with cancelable context
		go func() {
			err := metaLoop.Run(loopCtx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs2)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "context canceled")
			wg.Done()
		}()

		wg.Wait()

		obs3 := newTestObserver(t, nil)
		err := metaLoop.Join(ctx, obs3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loop closed")

		// expect that obs1 and obs2 each saw fewer than three remote segments
		fmt.Println(obs1.remoteSegCount)
		assert.True(t, obs1.remoteSegCount < 3)
		assert.True(t, obs2.remoteSegCount < 3)
	})
}

// TestMetainfoLoopClose does the following:
// * upload 3 remote segments
// * hook two observers up to metainfo loop
// * close loop partway through
// * expect both observers to exit with an error and see fewer than 3 remote segments
// * expect that a new observer attempting to join at this point receives a loop closed error
func TestMetainfoLoopClose(t *testing.T) {
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
		}, satellite.Metainfo.Service)
		// create a channel that allows us to sync closing the loop with loop iteration
		loopSync := make(chan struct{})

		// create 1 normal observer
		obs1 := newTestObserver(t, nil)

		// create another normal observer that will wait before returning during RemoteSegment so we can sync with context cancelation
		obs2 := newTestObserver(t, func() error {
			<-loopSync
			return nil
		})

		var wg sync.WaitGroup
		wg.Add(3)

		// start loop
		go func() {
			err := metaLoop.Run(ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "loop closed")
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs1)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "loop closed")
			wg.Done()
		}()
		go func() {
			err := metaLoop.Join(ctx, obs2)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "loop closed")
			wg.Done()
		}()

		// iterate over first segment, then close loop
		loopSync <- struct{}{}
		err := metaLoop.Close()
		assert.NoError(t, err)
		close(loopSync)

		wg.Wait()

		obs3 := newTestObserver(t, nil)
		err = metaLoop.Join(ctx, obs3)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "loop closed")

		// expect that obs1 and obs2 each saw fewer than three remote segments
		assert.True(t, obs1.remoteSegCount < 3)
		assert.True(t, obs2.remoteSegCount < 3)
	})
}

type testObserver struct {
	remoteSegCount  int
	remoteFileCount int
	inlineSegCount  int
	uniquePaths     map[string]struct{}
	t               *testing.T
	mockRemoteFunc  func() error // if set, run this during RemoteSegment()
}

func newTestObserver(t *testing.T, mockRemoteFunc func() error) *testObserver {
	return &testObserver{
		remoteSegCount:  0,
		remoteFileCount: 0,
		inlineSegCount:  0,
		uniquePaths:     make(map[string]struct{}),
		t:               t,
		mockRemoteFunc:  mockRemoteFunc,
	}
}

func (obs *testObserver) RemoteSegment(ctx context.Context, path storj.Path, pointer *pb.Pointer) error {
	obs.remoteSegCount++
	if _, ok := obs.uniquePaths[path]; ok {
		obs.t.Error("Expected unique path in observer.RemoteSegment")
	}
	obs.uniquePaths[path] = struct{}{}

	if obs.mockRemoteFunc != nil {
		return obs.mockRemoteFunc()
	}

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
