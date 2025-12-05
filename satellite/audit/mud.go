// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {

	mud.Provide[*Verifier](ball, func(log *zap.Logger, metabase *metabase.DB, dialer rpc.Dialer, overlay *overlay.Service, containment Containment, orders *orders.Service, id *identity.FullIdentity, cfg Config) *Verifier {
		return NewVerifier(log, metabase, dialer, overlay, containment, orders, id, cfg.MinBytesPerSecond, cfg.MinDownloadTimeout)
	})
	mud.Provide[*Worker](ball, NewWorker)
	mud.Provide[*ReverifyWorker](ball, NewReverifyWorker)
	mud.Provide[*Reverifier](ball, NewReverifier)

	mud.Provide[*DBReporter](ball, NewReporter)
	mud.Provide[NoReport](ball, func() NoReport {
		return NoReport{}
	})
	mud.RegisterInterfaceImplementation[Reporter, *DBReporter](ball)

	mud.Provide[*NoContainment](ball, func() *NoContainment {
		return &NoContainment{}
	})
	mud.RegisterInterfaceImplementation[Containment, WrappedContainment](ball)

	mud.Provide[*RunOnce](ball, NewRunOnce)
	config.RegisterConfig[Config](ball, "audit")
	config.RegisterConfig[RunOnceConfig](ball, "audit")

}
