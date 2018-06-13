// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package dtypes

import (
	"time"
)

type Path []string
type NodeID string
type PieceID string

type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Data       []byte
}

type EncKey string

type Node struct {
	NodeID  NodeID
	Address *Address
}

type Address struct {
	Network string
	Host    string
	Port    int
}
