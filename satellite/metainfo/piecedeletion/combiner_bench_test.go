// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/piecedeletion"
)

type SleepyHandler struct {
	Min, Max time.Duration

	TotalHandled int64
}

func (handler *SleepyHandler) Handle(ctx context.Context, node storj.NodeURL, queue piecedeletion.Queue) {
	if !sync2.Sleep(ctx, handler.Min) {
		return
	}

	for {
		list, ok := queue.PopAll()
		if !ok {
			return
		}
		for _, job := range list {
			atomic.AddInt64(&handler.TotalHandled, int64(len(job.Pieces)))
			job.Resolve.Success()
		}

		span := int(handler.Max - handler.Min)
		wait := testrand.Intn(span)

		if !sync2.Sleep(ctx, handler.Min+time.Duration(wait)) {
			return
		}
	}
}

func BenchmarkCombiner(b *testing.B) {
	const (
		// currently nodes are picked uniformly, however total piece distribution is not
		// hence we use a lower number to simulate frequent nodes
		nodeCount    = 500
		requestCount = 100
		// assume each request has ~2 segments
		callsPerRequest = 160
		// we cannot use realistic values here due to sleep granularity
		minWait = 1 * time.Millisecond
		maxWait = 20 * time.Millisecond
	)

	var activeLimits []int
	var queueSizes []int

	if testing.Short() {
		activeLimits = []int{8, 64, -1}
		queueSizes = []int{8, 128, -1}
	} else {
		activeLimits = []int{8, 32, 64, -1}
		queueSizes = []int{1, 8, 64, 128, -1}
	}

	nodes := []storj.NodeURL{}
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, storj.NodeURL{
			ID: testrand.NodeID(),
		})
	}

	for _, activeLimit := range activeLimits {
		for _, queueSize := range queueSizes {
			activeLimit, queueSize := activeLimit, queueSize
			name := fmt.Sprintf("active=%d,queue=%d", activeLimit, queueSize)
			b.Run(name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					func() {
						sleeper := &SleepyHandler{Min: minWait, Max: maxWait}
						limited := piecedeletion.NewLimitedHandler(sleeper, activeLimit)

						ctx := testcontext.New(b)
						defer ctx.Cleanup()

						newQueue := func() piecedeletion.Queue {
							return piecedeletion.NewLimitedJobs(queueSize)
						}

						var combiner *piecedeletion.Combiner
						if activeLimit > 0 {
							combiner = piecedeletion.NewCombiner(ctx, limited, newQueue)
						} else {
							combiner = piecedeletion.NewCombiner(ctx, sleeper, newQueue)
						}

						for request := 0; request < requestCount; request++ {
							ctx.Go(func() error {
								done, err := sync2.NewSuccessThreshold(callsPerRequest, 0.999999)
								if err != nil {
									return err
								}
								for call := 0; call < callsPerRequest; call++ {
									i := testrand.Intn(nodeCount)
									combiner.Enqueue(nodes[i], piecedeletion.Job{
										Pieces:  []storj.PieceID{testrand.PieceID()},
										Resolve: done,
									})
								}
								done.Wait(ctx)
								return nil
							})
						}

						ctx.Wait()
						combiner.Close()

						totalRequests := int64(callsPerRequest * requestCount)
						if sleeper.TotalHandled != totalRequests {
							b.Fatalf("handled only %d expected %d", sleeper.TotalHandled, totalRequests)
						}
					}()
				}
			})
		}
	}
}
