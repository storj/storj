// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
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

	mud.Provide[*SuccessTrackers](ball, func(log *zap.Logger, cfg Config) (*SuccessTrackers, error) {
		newTracker, ok := GetNewSuccessTracker(cfg.SuccessTrackerKind)
		if !ok {
			return nil, errs.New("Unknown success tracker kind %q", cfg.SuccessTrackerKind)
		}
		monkit.ScopeNamed(mon.Name() + ".success_trackers").Chain(newTracker())
		return NewSuccessTrackers(cfg.SuccessTrackerTrustedUplinks, newTracker), nil

	})

	mud.Provide[SuccessTracker](ball, func(log *zap.Logger, cfg Config) SuccessTracker {
		tracker := NewPercentSuccessTracker()
		monkit.ScopeNamed(mon.Name() + ".failure_tracker").Chain(tracker)
		return tracker
	})

}
