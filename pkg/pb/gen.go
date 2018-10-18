// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

//go:generate protoc --go_out=plugins=grpc:. meta.proto
//go:generate protoc --go_out=plugins=grpc:. overlay.proto
//go:generate protoc --go_out=plugins=grpc:. pointerdb.proto
//go:generate protoc --go_out=plugins=grpc:. piecestore.proto
//go:generate protoc --go_out=plugins=grpc:. bandwidth.proto
