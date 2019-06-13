// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package filters

// Filter is an interface for filters
type Filter interface {
	Contains(pieceID []byte) bool
	Add(pieceID []byte)
	Encode() []byte
}
