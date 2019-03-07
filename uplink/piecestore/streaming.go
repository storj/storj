// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"storj.io/storj/pkg/pb"
)

type Upload struct {
	Signer Signer
	Client pb.PiecestoreClient
}

func (client *Upload) Write(data []byte) (n int64, err error) {
	// these correspond to piecestore.Endpoint methods
	panic("TODO")
}

func (client *Upload) Commit() (*pb.PieceHash, error) {
	panic("TODO")
}

type Download struct {
	Client pb.PiecestoreClient
}

func (client *Download) Read(data []byte) error {
	panic("TODO")
	// these correspond to piecestore.Endpoint methods
}

func (client *Download) Close() error {
	panic("TODO")
}
