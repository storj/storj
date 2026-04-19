// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "metainfo")
	mud.View[Config, metabase.DatabaseConfig](ball, func(c Config) metabase.DatabaseConfig {
		return metabase.DatabaseConfig{
			URL: c.DatabaseURL,
			// TODO: application name should come from a config.
			Config: c.Metabase("satellite"),
		}
	})
	mud.Provide[*Endpoint](ball, NewEndpoint)

	mud.Provide[*SuccessTrackerMonitor](ball, NewSuccessTrackerMonitor)
	mud.Provide[*Trackers](ball, func(log *zap.Logger, monitor *SuccessTrackerMonitor, trustedUplinks *trust.TrustedPeersList, cfg Config) (*Trackers, error) {
		var approvedUplinks []storj.NodeID
		for _, uplinkIDString := range cfg.SuccessTrackerTrustedUplinks {
			uplinkID, err := storj.NodeIDFromString(uplinkIDString)
			if err != nil {
				log.Warn("Wrong uplink ID for the trusted list of the success trackers", zap.String("uplink", uplinkIDString), zap.Error(err))
			}
			approvedUplinks = append(approvedUplinks, uplinkID)
		}
		newTracker, ok := GetNewSuccessTracker(cfg.SuccessTrackerKind)
		if !ok {
			return nil, errs.New("Unknown success tracker kind %q", cfg.SuccessTrackerKind)
		}
		monkit.ScopeNamed(mon.Name() + ".success_trackers").Chain(newTracker())

		failureTracker := NewPercentSuccessTracker()
		monkit.ScopeNamed(mon.Name() + ".failure_tracker").Chain(failureTracker)

		trackers := NewTrackers(cfg, approvedUplinks, func(uplink storj.NodeID) SuccessTracker {
			return newTracker()
		}, failureTracker, trustedUplinks)

		monitor.Register(trackers)
		return trackers, nil
	})

	mud.Provide[*NodeSelectionStats](ball, NewNodeSelectionStats)

}
