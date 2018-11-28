// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type NodeID = storj.NodeID
type NodeIDList = storj.NodeIDList
type Node = pb.Node
type NodeStats = pb.NodeStats

//go:generate protoc --gogo_out=plugins=grpc:. -I=. -I=$GOPATH/src/storj.io/storj/pkg/pb statdb.proto
