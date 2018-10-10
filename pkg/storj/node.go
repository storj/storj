// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import "encoding/hex"

// NodeID is a unique node identifier
type NodeID [32]byte

// HexString returns NodeID as hex encoded string
func (id *NodeID) HexString() string { return hex.EncodeToString(id[:]) }

// Bytes returns raw bytes of the id
func (id NodeID) Bytes() []byte { return id[:] }
