// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair/checker"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*ECRepairer](ball, func(dialer rpc.Dialer, satelliteSignee signing.Signee, cfg Config) *ECRepairer {
		// The injected dialer is shared with every other component in the ball and
		// carries the pool from rpc.NewDefaultPooledDialer (capacity 100), so
		// repairer.connection-pool.* was silently ignored here -- it was only ever
		// honoured by the non-modular peer in satellite/repairer.go. A pool that
		// small is well below the number of piece transfers a repairer runs at once
		// (max-repair x the segment's required count), so returned connections get
		// evicted and closed before the next transfer can reuse them, and every
		// piece download pays for a fresh dial on the critical path.
		//
		// rpc.Dialer is a value, so replacing the pool on our copy scopes the
		// config to repair without affecting anything else in the ball. The
		// assignment is unconditional: Capacity 0 means "no pooling", and leaving
		// the ball's shared pool in place would hand the operator a 100-entry pool
		// instead. A nil Pool is supported -- rpcpool.Pool.Get has a nil receiver
		// check -- and is exactly what the standalone peer does in that case.
		dialer.Pool = cfg.ConnectionPool.NewPool()

		ec := NewECRepairer(dialer, satelliteSignee, cfg.DialTimeout, cfg.DownloadTimeout, cfg.InMemoryRepair, cfg.InMemoryUpload, cfg.DownloadLongTail, cfg.DownloadChunkSize)
		// the pool was built here, so it is ours to close; mud picks up
		// ECRepairer.Close automatically.
		ec.ownedPool = dialer.Pool
		return ec
	})
	mud.Provide[*SegmentRepairer](ball, func(log *zap.Logger, metabase *metabase.DB, orders *orders.Service, overlay *overlay.Service, reporter audit.Reporter, ecRepairer *ECRepairer, placements nodeselection.PlacementDefinitions, config Config, checkerConfig checker.Config) (*SegmentRepairer, error) {
		return NewSegmentRepairer(log, metabase, orders, overlay, reporter, ecRepairer, placements, checkerConfig.RepairThresholdOverrides, checkerConfig.RepairTargetOverrides, config)
	})
	config.RegisterConfig[Config](ball, "repairer")
	mud.Provide[*Service](ball, NewService)
	mud.Provide[*QueueStat](ball, NewQueueStat)
	config.RegisterConfig[QueueStatConfig](ball, "repair-queue-check")
}
