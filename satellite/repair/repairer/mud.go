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
		return NewECRepairer(dialer, satelliteSignee, cfg.DialTimeout, cfg.DownloadTimeout, cfg.InMemoryRepair, cfg.InMemoryUpload, cfg.DownloadLongTail)
	})
	mud.Provide[*SegmentRepairer](ball, func(log *zap.Logger, metabase *metabase.DB, orders *orders.Service, overlay *overlay.Service, reporter audit.Reporter, ecRepairer *ECRepairer, placements nodeselection.PlacementDefinitions, config Config, checkerConfig checker.Config) (*SegmentRepairer, error) {
		return NewSegmentRepairer(log, metabase, orders, overlay, reporter, ecRepairer, placements, checkerConfig.RepairThresholdOverrides, checkerConfig.RepairTargetOverrides, config)
	})
	config.RegisterConfig[Config](ball, "repairer")
	mud.Provide[*Service](ball, NewService)

}
