// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/sync2/combiner"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/shared/lrucache"
)

// keyTailsHandler is a handler for processing API key tails using a combiner queue.
type keyTailsHandler struct {
	combiner atomic.Pointer[combiner.Combiner[keyTailTask]]
	cache    *lrucache.ExpiringLRUOf[struct{}]
}

// keyTailTask represents a task for processing an API key tail.
type keyTailTask struct {
	rootKeyID  uuid.UUID
	serialized string
	raw        []byte
	secret     []byte
}

// initTailsCombiner initializes the combiner queue for processing API key tails.
func (endpoint *Endpoint) initTailsCombiner(parentCtx context.Context) {
	process := func(ctx context.Context, q *combiner.Queue[keyTailTask]) {
		for tasks := range q.Batches() {
			now := time.Now()
			var tailsToUpsert []console.APIKeyTail

			for _, task := range tasks {
				if _, cached := endpoint.keyTailsHandler.cache.GetCached(ctx, task.serialized); cached {
					continue
				}

				mac, err := macaroon.ParseMacaroon(task.raw)
				if err != nil {
					endpoint.log.Warn("failed to parse macaroon", zap.Error(err))
					continue
				}

				tails := mac.Tails(task.secret)
				caveats := mac.Caveats()

				// We start iteration from 1 because the first tail is the root key itself.
				for i := 1; i < len(tails); i++ {
					tail := console.APIKeyTail{
						RootKeyID:  task.rootKeyID,
						Tail:       tails[i],
						ParentTail: tails[i-1],
						Caveat:     caveats[i-1],
						LastUsed:   now,
					}

					tailsToUpsert = append(tailsToUpsert, tail)
				}

				endpoint.keyTailsHandler.cache.Add(ctx, task.serialized, struct{}{})
			}

			if err := endpoint.apiKeyTails.UpsertBatch(ctx, tailsToUpsert); err != nil {
				endpoint.log.Error("batched upsert failed", zap.Int("count", len(tailsToUpsert)), zap.Error(err))
			}
		}
	}

	fail := func(ctx context.Context, q *combiner.Queue[keyTailTask]) {
		for batch := range q.Batches() {
			endpoint.log.Error("combiner aborted; dropping APIKeyTail batch", zap.Int("count", len(batch)))
		}
	}

	c := combiner.New[keyTailTask](parentCtx, combiner.Options[keyTailTask]{
		Process:   process,
		Fail:      fail,
		QueueSize: endpoint.config.APIKeyTailsConfig.QueueSize,
	})
	endpoint.keyTailsHandler.combiner.Store(c)
}

// TestWaitForTailsCombinerWorkers waits for the combiner workers to finish processing.
func (endpoint *Endpoint) TestWaitForTailsCombinerWorkers(ctx context.Context) error {
	if !endpoint.config.APIKeyTailsConfig.CombinerQueueEnabled {
		return nil
	}

	c := endpoint.keyTailsHandler.combiner.Load()
	if c != nil {
		c.Stop()
		return c.Wait(ctx)
	}
	return nil
}
