// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/trust"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	config.RegisterConfig[Config](ball, "metainfo")
	mud.View[Config, metabase.DatabaseConfig](ball, func(c Config) metabase.DatabaseConfig {
		m := c.Metabase("satellite")
		m.ConnParams = &c.ConnParams
		return metabase.DatabaseConfig{
			URL: c.DatabaseURL,
			// TODO: application name should come from a config.
			Config: m,
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

		retryTracker := NewPercentSuccessTracker()
		monkit.ScopeNamed(mon.Name() + ".retry_tracker").Chain(retryTracker)

		trackers := NewTrackers(cfg, approvedUplinks, func(uplink storj.NodeID) SuccessTracker {
			return newTracker()
		}, failureTracker, retryTracker, trustedUplinks)

		monitor.Register(trackers)
		return trackers, nil
	})

	mud.Provide[*TrackerInfoExtension](ball, func() *TrackerInfoExtension {
		return &TrackerInfoExtension{}
	})
	mud.Implementation[[]debug.Extension, *TrackerInfoExtension](ball)

	mud.Provide[*TrackerInfo](ball, func(trackers *Trackers, db overlay.DB, extension *TrackerInfoExtension) *TrackerInfo {
		info := NewTrackerInfo(trackers, db)
		extension.Set(info)
		return info
	})
	// only initialized when explicitly selected (e.g. by the api subcommand).
	mud.Tag[*TrackerInfo, mud.Optional](ball, mud.Optional{})

	mud.Provide[*NodeSelectionStats](ball, NewNodeSelectionStats)
}
