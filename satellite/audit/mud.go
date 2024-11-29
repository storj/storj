// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc"
	"storj.io/storj/private/mud"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular/config"
)

// Module is a mud module.
func Module(ball *mud.Ball) {

	mud.Provide[*Verifier](ball, func(log *zap.Logger, metabase *metabase.DB, dialer rpc.Dialer, overlay *overlay.Service, containment Containment, orders *orders.Service, id *identity.FullIdentity, cfg Config) *Verifier {
		return NewVerifier(log, metabase, dialer, overlay, containment, orders, id, cfg.MinBytesPerSecond, cfg.MinDownloadTimeout)
	})

	// TODO: we need real containment for running service.
	mud.Provide[Containment](ball, func() Containment {
		return &noContainment{}
	})
	mud.Provide[*RunOnce](ball, NewRunOnce)
	config.RegisterConfig[Config](ball, "audit")
	config.RegisterConfig[RunOnceConfig](ball, "audit")

}
