// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"sync"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/metabase"
)

// Verify verifies a collection of segments.
func (service *Service) Verify(ctx context.Context, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	for _, segment := range segments {
		segment.Status.Retry = int32(service.config.Check)
	}

	batches, err := service.CreateBatches(ctx, segments)
	if err != nil {
		return Error.Wrap(err)
	}

	err = service.VerifyBatches(ctx, batches)
	if err != nil {
		return Error.Wrap(err)
	}

	retrySegments := []*Segment{}
	for _, segment := range segments {
		if segment.Status.Retry > 0 {
			retrySegments = append(retrySegments, segment)
		}
	}

	if len(retrySegments) == 0 {
		return nil
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

	err = service.VerifyBatches(ctx, retryBatches)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// VerifyBatches verifies batches.
func (service *Service) VerifyBatches(ctx context.Context, batches []*Batch) error {
	defer mon.Task()(&ctx)(nil)

	var mu sync.Mutex

	limiter := sync2.NewLimiter(service.config.Concurrency)
	for _, batch := range batches {
		batch := batch

		nodeURL, err := service.convertAliasToNodeURL(ctx, batch.Alias)
		if err != nil {
			return Error.Wrap(err)
		}

		ignoreThrottle := service.priorityNodes.Contains(batch.Alias)

		limiter.Go(ctx, func() {
			err := service.verifier.Verify(ctx, nodeURL, batch.Items, ignoreThrottle)
			if err != nil {
				if ErrNodeOffline.Has(err) {
					mu.Lock()
					service.onlineNodes.Remove(batch.Alias)
					mu.Unlock()
				}
				service.log.Error("verifying a batch failed", zap.Error(err))
			}
		})
	}
	limiter.Wait()

	return nil
}

// convertAliasToNodeURL converts a node alias to node url, using a cache if needed.
func (service *Service) convertAliasToNodeURL(ctx context.Context, alias metabase.NodeAlias) (_ storj.NodeURL, err error) {
	nodeURL, ok := service.aliasToNodeURL[alias]
	if !ok {
		// not in cache, use the slow path
		nodeIDs, err := service.metabase.ConvertAliasesToNodes(ctx, []metabase.NodeAlias{alias})
		if err != nil {
			return storj.NodeURL{}, Error.Wrap(err)
		}

		info, err := service.overlay.Get(ctx, nodeIDs[0])
		if err != nil {
			return storj.NodeURL{}, Error.Wrap(err)
		}

		nodeURL = storj.NodeURL{
			ID:      info.Id,
			Address: info.Address.Address,
		}

		service.aliasToNodeURL[alias] = nodeURL
	}
	return nodeURL, nil
}
