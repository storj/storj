// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/drpc"
)

type replayRouter struct {
	replaySafe, all drpc.Mux
}

func newReplayRouter(replaySafe, all drpc.Mux) drpc.Mux {
	return &replayRouter{
		replaySafe: replaySafe,
		all:        all,
	}
}

// Register implements drpc.Mux.
func (r *replayRouter) Register(srv interface{}, desc drpc.Description) error {
	allErr := r.all.Register(srv, desc)
	switch desc.(type) {
	case pb.DRPCPiecestoreDescription:
		return errs.Combine(allErr, r.replaySafe.Register(srv, replaySafePiecestore{}))
	default:
		return allErr
	}
}

type replaySafePiecestore struct{}

func (replaySafePiecestore) NumMethods() int { return 2 }

func (replaySafePiecestore) Method(n int) (string, drpc.Encoding, drpc.Receiver, interface{}, bool) {
	switch n {
	case 0:
		rpc, enc, receiver, method, ok := (pb.DRPCPiecestoreDescription{}).Method(0)
		return rpc, enc, receiver, method, ok && rpc == "/piecestore.Piecestore/Upload"

	case 1:
		rpc, enc, receiver, method, ok := (pb.DRPCPiecestoreDescription{}).Method(1)
		return rpc, enc, receiver, method, ok && rpc == "/piecestore.Piecestore/Download"
	default:
		return "", nil, nil, nil, false
	}
}
