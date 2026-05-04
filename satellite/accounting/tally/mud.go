// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tally

import (
	"storj.io/storj/satellite/payments"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, New)
	config.RegisterConfig[Config](ball, "tally")
	mud.Provide[map[int32]ProductUsagePriceModel](ball, func(productPrices map[int32]payments.ProductUsagePriceModel) map[int32]ProductUsagePriceModel {
		tallyProductPrices := make(map[int32]ProductUsagePriceModel)
		for id, price := range productPrices {
			tallyProductPrices[id] = ProductUsagePriceModel{
				ProductID:             price.ProductID,
				StorageRemainderBytes: price.StorageRemainderBytes,
			}
		}
		return tallyProductPrices
	})
	mud.Provide[PlacementProductMap](ball, func(m payments.PlacementProductIdMap) PlacementProductMap {
		globalPlacementMap := make(PlacementProductMap)
		for placement, productID := range m {
			globalPlacementMap[placement] = productID
		}
		return globalPlacementMap
	})
}
