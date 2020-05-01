// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
)

// Dialer implements dialing piecestores and sending delete requests with batching and redial threshold.
type Dialer struct {
	log    *zap.Logger
	dialer rpc.Dialer

	requestTimeout   time.Duration
	failThreshold    time.Duration
	piecesPerRequest int

	mu         sync.RWMutex
	dialFailed map[storj.NodeID]time.Time
}

// NewDialer returns a new Dialer.
func NewDialer(log *zap.Logger, dialer rpc.Dialer, requestTimeout, failThreshold time.Duration, piecesPerRequest int) *Dialer {
	return &Dialer{
		log:    log,
		dialer: dialer,

		requestTimeout:   requestTimeout,
		failThreshold:    failThreshold,
		piecesPerRequest: piecesPerRequest,

		dialFailed: map[storj.NodeID]time.Time{},
	}
}

// Handle tries to send the deletion requests to the specified node.
func (dialer *Dialer) Handle(ctx context.Context, node *pb.Node, queue Queue) {
	defer FailPending(queue)

	if dialer.recentlyFailed(ctx, node) {
		return
	}

	client, conn, err := dialPieceStore(ctx, dialer.dialer, node)
	if err != nil {
		dialer.log.Debug("failed to dial", zap.Stringer("id", node.Id), zap.Error(err))
		dialer.markFailed(ctx, node)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			dialer.log.Debug("closing connection failed", zap.Stringer("id", node.Id), zap.Error(err))
		}
	}()

	for {
		if err := ctx.Err(); err != nil {
			return
		}

		jobs, ok := queue.PopAll()
		if !ok {
			return
		}

		for len(jobs) > 0 {
			batch, promises, rest := batchJobs(jobs, dialer.piecesPerRequest)
			jobs = rest

			requestCtx, cancel := context.WithTimeout(ctx, dialer.requestTimeout)
			resp, err := client.DeletePieces(requestCtx, &pb.DeletePiecesRequest{
				PieceIds: batch,
			})
			cancel()

			for _, promise := range promises {
				if err != nil {
					promise.Failure()
				} else {
					promise.Success()
				}
			}

			if err != nil {
				dialer.log.Debug("deletion request failed", zap.Stringer("id", node.Id), zap.Error(err))
				// don't try to send to this storage node a bit, when the deletion times out
				if errs2.IsCanceled(err) {
					dialer.markFailed(ctx, node)
				}
				break
			} else {
				mon.IntVal("deletion pieces unhandled count").Observe(resp.UnhandledCount)
			}

			jobs = append(jobs, queue.PopAllWithoutClose()...)
		}

		// if we failed early, remaining jobs should be marked as failures
		for _, job := range jobs {
			job.Resolve.Failure()
		}
	}
}

// markFailed marks node as something failed recently, so we shouldn't try again,
// for some time.
func (dialer *Dialer) markFailed(ctx context.Context, node *pb.Node) {
	dialer.mu.Lock()
	defer dialer.mu.Unlock()

	now := time.Now()

	lastFailed, ok := dialer.dialFailed[node.Id]
	if !ok || lastFailed.Before(now) {
		dialer.dialFailed[node.Id] = now
	}
}

// recentlyFailed checks whether a request to node recently failed.
func (dialer *Dialer) recentlyFailed(ctx context.Context, node *pb.Node) bool {
	dialer.mu.RLock()
	lastFailed, ok := dialer.dialFailed[node.Id]
	dialer.mu.RUnlock()

	// when we recently failed to dial, then fail immediately
	return ok && time.Since(lastFailed) < dialer.failThreshold
}

func batchJobs(jobs []Job, maxBatchSize int) (pieces []storj.PieceID, promises []Promise, rest []Job) {
	for i, job := range jobs {
		if len(pieces) >= maxBatchSize {
			return pieces, promises, jobs[i:]
		}

		pieces = append(pieces, job.Pieces...)
		promises = append(promises, job.Resolve)
	}

	return pieces, promises, nil
}

func dialPieceStore(ctx context.Context, dialer rpc.Dialer, target *pb.Node) (pb.DRPCPiecestoreClient, *rpc.Conn, error) {
	conn, err := dialer.DialNode(ctx, target)
	if err != nil {
		return nil, nil, err
	}

	return pb.NewDRPCPiecestoreClient(conn), conn, nil
}
