// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metadata

import (
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/peertls/extensions"
	"storj.io/common/version"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/orders"
)

// API is the satellite API process.
//
// architecture: Peer
type API struct{}

type Config struct{}

// NewAPI creates a new satellite API process.
func NewAPI(log *zap.Logger, full *identity.FullIdentity, db satellite.DB,
	metabaseDB *metabase.DB, revocationDB extensions.RevocationDB,
	liveAccounting accounting.Cache, rollupsWriteCache *orders.RollupsWriteCache,
	config *Config, versionInfo version.Info, atomicLogLevel *zap.AtomicLevel) (*API, error) {
	peer := &API{}

	return peer, nil
}
