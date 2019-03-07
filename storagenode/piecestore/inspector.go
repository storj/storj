// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
)

// Inspector does inspectory things
type Inspector struct {
	pieces   *pieces.Store
	kademlia *kademlia.Kademlia
	orders   orders.Table
}
