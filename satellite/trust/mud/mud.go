// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_mud

// TODO: this package is separated as we have circular dependencies between due to the usage of metainfo.Config.

import (
	"go.uber.org/zap"

	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/mud"
)

// Module is a mud Module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*trust.TrustedPeersList](ball, func(logger *zap.Logger, config metainfo.Config) (*trust.TrustedPeersList, error) {
		return trust.NewTrustedPeerList(config.SuccessTrackerTrustedUplinks), nil
	})

}
