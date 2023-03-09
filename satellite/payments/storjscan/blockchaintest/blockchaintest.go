// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package blockchaintest

import (
	"storj.io/common/testrand"
	"storj.io/storj/private/blockchain"
)

// NewAddress creates new blockchain address for testing.
func NewAddress() blockchain.Address {
	var address blockchain.Address
	b := testrand.BytesInt(blockchain.AddressLength)
	copy(address[:], b)
	return address
}

// NewHash creates new blockchain hash for testing.
func NewHash() blockchain.Hash {
	var h blockchain.Hash
	b := testrand.BytesInt(blockchain.HashLength)
	copy(h[:], b)
	return h
}
