// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Package fastpb contains generated marshaling for Pointers.
package fastpb

import (
	"storj.io/common/storj"
)

//go:generate protoc -I=. --gogofaster_out=Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types,paths=source_relative:. pointerdb.proto

// NodeID is a unique node identifier.
type NodeID = storj.NodeID

// PieceID is a unique identifier for pieces.
type PieceID = storj.PieceID
