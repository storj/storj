// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_mud

// TODO: this package is separated as we have circular dependencies between due to the usage of metainfo.Config.

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/mud"
)

// Module is a mud Module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*trust.TrustedPeersList](ball, func(logger *zap.Logger, config metainfo.Config) (*trust.TrustedPeersList, error) {
		var uplinks []storj.NodeID
		for _, u := range config.SuccessTrackerTrustedUplinks {
			nodeID, err := storj.NodeIDFromString(u)
			if err != nil {
				return nil, errs.Wrap(err)
			}
			uplinks = append(uplinks, nodeID)
		}
		return trust.NewTrustedPeerList(uplinks), nil
	})

}
