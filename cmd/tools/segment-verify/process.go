// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Verify verifies a collection of segments.
func (service *Service) Verify(ctx context.Context, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, segment := range segments {
		segment.Status.Retry = VerifyPieces
	}

	batches, err := service.CreateBatches(ctx, segments)
	if err != nil {
		return Error.Wrap(err)
	}

	service.VerifyBatches(ctx, batches)

	retrySegments := []*Segment{}
	for _, segment := range segments {
		if segment.Status.Retry > 0 {
			retrySegments = append(retrySegments, segment)
		}
	}

	// Reverse the pieces slice to ensure we pick different nodes this time.
	for _, segment := range retrySegments {
		xs := segment.AliasPieces
		for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
			xs[i], xs[j] = xs[j], xs[i]
		}
		// Also remove priority nodes, because we have already checked them.
		service.removePriorityPieces(segment)
	}

	retryBatches, err := service.CreateBatches(ctx, retrySegments)
	if err != nil {
		return Error.Wrap(err)
	}

	service.VerifyBatches(ctx, retryBatches)

	return nil
}

// VerifyBatches verifies batches.
func (service *Service) VerifyBatches(ctx context.Context, batches []*Batch) {
	defer mon.Task()(&ctx)(nil)

	var mu sync.Mutex

	limiter := sync2.NewLimiter(ConcurrentRequests)
	for _, batch := range batches {
		batch := batch
		limiter.Go(ctx, func() {
			err := service.VerifyBatch(ctx, batch)
			if err != nil {
				if ErrNodeOffline.Has(err) {
					mu.Lock()
					service.OfflineNodes.Add(batch.Alias)
					mu.Unlock()
				}

				service.log.Error("verifying a batch failed", zap.Error(err))
			}
		})
	}
	limiter.Wait()
}
