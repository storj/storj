// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import "storj.io/storj/pkg/storj"

// Path represents a object path
type Path = storj.Path
type NodeID = storj.NodeID
type NodeIDList = storj.NodeIDList

//go:generate protoc -I. --gogo_out=plugins=grpc:. meta.proto overlay.proto pointerdb.proto piecestore.proto bandwidth.proto kadcli.proto datarepair.proto node.proto
