// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testdata

import (
	"storj.io/storj/storagenode/storagenodedb"
)

var v42 = MultiDBState{
	Version: 42,
	DBStates: DBStates{
		storagenodedb.UsedSerialsDBName:     &DBState{},
		storagenodedb.StorageUsageDBName:    v41.DBStates[storagenodedb.StorageUsageDBName],
		storagenodedb.ReputationDBName:      v41.DBStates[storagenodedb.ReputationDBName],
		storagenodedb.PieceSpaceUsedDBName:  v41.DBStates[storagenodedb.PieceSpaceUsedDBName],
		storagenodedb.PieceInfoDBName:       v41.DBStates[storagenodedb.PieceInfoDBName],
		storagenodedb.PieceExpirationDBName: v41.DBStates[storagenodedb.PieceExpirationDBName],
		storagenodedb.OrdersDBName:          v41.DBStates[storagenodedb.OrdersDBName],
		storagenodedb.BandwidthDBName:       v41.DBStates[storagenodedb.BandwidthDBName],
		storagenodedb.SatellitesDBName:      v41.DBStates[storagenodedb.SatellitesDBName],
		storagenodedb.DeprecatedInfoDBName:  v41.DBStates[storagenodedb.DeprecatedInfoDBName],
		storagenodedb.NotificationsDBName:   v41.DBStates[storagenodedb.NotificationsDBName],
		storagenodedb.HeldAmountDBName:      v41.DBStates[storagenodedb.HeldAmountDBName],
		storagenodedb.PricingDBName:         v41.DBStates[storagenodedb.PricingDBName],
	},
}
