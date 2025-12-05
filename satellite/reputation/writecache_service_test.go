// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/reputation"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestHeavyLockContention(t *testing.T) {
	const (
		nodeCount  = 10
		rounds     = 200
		windowSize = 10 * time.Minute
	)

	// construct random test data
	timeStart := time.Now().Add(-rounds * windowSize)
	nodes := make([]storj.NodeID, nodeCount)
	testData := make([][]reputation.Mutations, nodeCount)
	for nodeNum := 0; nodeNum < nodeCount; nodeNum++ {
		nodes[nodeNum] = testrand.NodeID()
		testData[nodeNum] = make([]reputation.Mutations, rounds)
		for roundNum := 0; roundNum < rounds; roundNum++ {
			mutations := &testData[nodeNum][roundNum]
			mutations.FailureResults = testrand.Intn(10)
			mutations.OfflineResults = testrand.Intn(10)
			mutations.UnknownResults = testrand.Intn(10)
			mutations.PositiveResults = testrand.Intn(10)
			mutations.OnlineHistory = &pb.AuditHistory{
				Windows: []*pb.AuditWindow{
					{
						WindowStart: timeStart.Add(windowSize * time.Duration(roundNum)),
						OnlineCount: int32(mutations.FailureResults + mutations.UnknownResults + mutations.PositiveResults),
						TotalCount:  int32(mutations.OfflineResults),
					},
				},
			}
		}
	}

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		config := reputation.Config{
			AuditLambda:        1,
			AuditWeight:        1,
			InitialAlpha:       1000,
			InitialBeta:        0,
			UnknownAuditDQ:     0.6,
			UnknownAuditLambda: 0.95,
			AuditHistory: reputation.AuditHistoryConfig{
				WindowSize:     windowSize,
				TrackingPeriod: 1 * time.Hour,
			},
			FlushInterval:      2 * time.Hour,
			ErrorRetryInterval: 0,
		}
		reputationDB := db.Reputation()
		writecacheDB := reputation.NewCachingDB(zaptest.NewLogger(t), reputationDB, config)
		var group errgroup.Group

		// Make room for results ahead of time, so we don't need to use any
		// extra locks or atomics while the goroutines are goroutine-ing. Extra
		// synchronization primitives might synchronize things that otherwise
		// wouldn't have been synchronized.
		resultInfos := make([][]*reputation.Info, nodeCount)

		for nodeNum := 0; nodeNum < nodeCount; nodeNum++ {
			nodeNum := nodeNum
			group.Go(func() error {
				resultInfos[nodeNum] = make([]*reputation.Info, rounds)
				for roundNum := 0; roundNum < rounds; roundNum++ {
					now := testData[nodeNum][roundNum].OnlineHistory.Windows[0].WindowStart
					now = now.Add(time.Duration(testrand.Int63n(int64(windowSize))))
					info, err := writecacheDB.ApplyUpdates(ctx, nodes[nodeNum], testData[nodeNum][roundNum], config, now)
					if err != nil {
						return fmt.Errorf("node[%d] in round[%d]: %w", nodeNum, roundNum, err)
					}
					resultInfos[nodeNum][roundNum] = info.Copy()
				}
				return nil
			})
		}

		err := group.Wait()
		require.NoError(t, err)

		// verify each step along the way had the expected running totals
		for nodeNum := 0; nodeNum < nodeCount; nodeNum++ {
			totalAudits := 0
			totalSuccess := 0
			for roundNum := 0; roundNum < rounds; roundNum++ {
				input := testData[nodeNum][roundNum]
				output := resultInfos[nodeNum][roundNum]

				totalAudits += input.UnknownResults
				totalAudits += input.OfflineResults
				totalAudits += input.FailureResults
				totalAudits += input.PositiveResults
				totalSuccess += input.PositiveResults

				require.NotNil(t, output)
				require.Equalf(t, int64(totalAudits), output.TotalAuditCount,
					"node[%d] in round[%d]: expected %d, but TotalAuditCount=%d", nodeNum, roundNum, totalAudits, output.TotalAuditCount)
				require.Equal(t, int64(totalSuccess), output.AuditSuccessCount)
				expectLen := roundNum + 1
				if windowSize*time.Duration(roundNum) > config.AuditHistory.TrackingPeriod {
					expectLen = int(config.AuditHistory.TrackingPeriod/windowSize) + 1
				}
				require.Lenf(t, output.AuditHistory.Windows, expectLen,
					"node[%d] in round[%d]", nodeNum, roundNum)
				require.Equal(t, input.OnlineHistory.Windows[0].OnlineCount, output.AuditHistory.Windows[expectLen-1].OnlineCount)
				require.Equal(t, input.OnlineHistory.Windows[0].TotalCount, output.AuditHistory.Windows[expectLen-1].TotalCount)
				require.Equal(t, input.OnlineHistory.Windows[0].WindowStart.UTC().Round(0), output.AuditHistory.Windows[expectLen-1].WindowStart.UTC().Round(0))
			}
		}
	})
}

func TestFetchingInfoWhileEntryIsSyncing(t *testing.T) {
	const (
		windowSize = 10 * time.Minute
		numRounds  = 20
	)

	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		config := reputation.Config{
			AuditLambda:        1,
			AuditWeight:        1,
			InitialAlpha:       1000,
			InitialBeta:        0,
			UnknownAuditDQ:     0.6,
			UnknownAuditLambda: 0.95,
			AuditHistory: reputation.AuditHistoryConfig{
				WindowSize:     windowSize,
				TrackingPeriod: 1 * time.Hour,
			},
			FlushInterval:      2 * time.Hour,
			ErrorRetryInterval: 0,
		}
		logger := zaptest.NewLogger(t)
		reputationDB := db.Reputation()
		writecache := reputation.NewCachingDB(logger.Named("writecache"), reputationDB, config)
		const positiveAudits = 123

		for i := 0; i < numRounds; i++ {
			group, ctx := errgroup.WithContext(ctx)
			startTime := time.Now()
			managerCtx, managerCancel := context.WithCancel(ctx)

			nodeID := testrand.NodeID()
			// get node into the cache
			_, err := writecache.ApplyUpdates(ctx, nodeID, reputation.Mutations{}, config, startTime)
			require.NoError(t, err)

			// The best we can do without an attached debugger framework is
			// to try and time these calls at about the same time, and hope
			// that everything doesn't all happen in the same order in every
			// round. We'd like to test things happening in a few different
			// orders.

			// In this goroutine "A", we apply two different sets of updates,
			// one after the other. Then we request a sync to the database.
			group.Go(func() error {
				// when A is done, let manager 1 know it can quit too
				defer managerCancel()

				info, err := writecache.ApplyUpdates(ctx, nodeID, reputation.Mutations{
					OnlineHistory: &pb.AuditHistory{
						Windows: []*pb.AuditWindow{
							{
								WindowStart: startTime.Round(windowSize),
								OnlineCount: 0,
								TotalCount:  10,
							},
						},
						Score: 0,
					},
				}, config, startTime.Round(windowSize).Add(time.Second))
				if err != nil {
					return err
				}
				// the mutation should be visible now
				if len(info.AuditHistory.Windows) != 1 {
					return fmt.Errorf("assertion error: windows slice is %v (expected len 1)", info.AuditHistory.Windows)
				}

				// wait (very) briefly and make a second change. other satellite worker
				// processes (such as goroutine "C") should not ever see the above change
				// without this one (otherwise, that means we're writing everything to
				// the database instead of caching writes).
				time.Sleep(10 * time.Millisecond)
				info, err = writecache.ApplyUpdates(ctx, nodeID, reputation.Mutations{
					PositiveResults: positiveAudits,
				}, config, startTime.Round(windowSize).Add(2*time.Second))
				if err != nil {
					return err
				}
				if info.TotalAuditCount != positiveAudits {
					return fmt.Errorf("assertion error: TotalAuditCount is %d (expected %d)", info.TotalAuditCount, positiveAudits)
				}

				// now trigger a flush
				err = writecache.RequestSync(ctx, nodeID)
				if err != nil {
					return err
				}
				return nil
			})

			// In this goroutine "B", we repeatedly request the info for the
			// node from the cache, until we see that the changes from "A"
			// have been made. We count the number of results we get in the
			// interim state (the audit history has been extended, but the
			// positive audits have not been recorded) just to get an idea
			// of how effective this test is.
			var (
				queriesFromB            int
				queriesFromBSeeingDirty int
			)
			group.Go(func() error {
				for {
					queriesFromB++
					info, err := writecache.Get(ctx, nodeID)
					if err != nil {
						return err
					}
					firstChangeSeen := (len(info.AuditHistory.Windows) > 0)
					secondChangeSeen := (info.TotalAuditCount == positiveAudits)
					if firstChangeSeen != secondChangeSeen {
						queriesFromBSeeingDirty++
					}
					if secondChangeSeen == true {
						// test is done.
						return nil
					}
				}
			})

			// And goroutine "C" acts like a separate server with its own
			// connection to the reputation db. We expect never to see the
			// interim dirty state; just before-the-changes and
			// after-the-changes.
			var (
				queriesFromC            int
				queriesFromCSeeingDirty int
			)
			group.Go(func() error {
				for {
					queriesFromC++
					info, err := reputationDB.Get(ctx, nodeID)
					if err != nil {
						return err
					}
					firstChangeSeen := (len(info.AuditHistory.Windows) > 0)
					secondChangeSeen := (info.TotalAuditCount == positiveAudits)
					if firstChangeSeen != secondChangeSeen {
						queriesFromCSeeingDirty++
					}
					if secondChangeSeen == true {
						// test is done.
						return nil
					}
				}
			})

			// Goroutine "D" manages syncing for writecache.
			group.Go(func() error {
				err := writecache.Manage(managerCtx)
				return errs2.IgnoreCanceled(err)
			})

			require.NoError(t, group.Wait())
			require.Zero(t, queriesFromCSeeingDirty)

			logger.Debug("round complete",
				zap.Int("B queries", queriesFromB),
				zap.Int("B queries observing dirty state", queriesFromBSeeingDirty),
				zap.Int("C queries", queriesFromC))
		}
	})
}
