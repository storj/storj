// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/bloomfilter"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/storagenode/pieces"
)

// EndpointError defines errors class for Endpoint
var EndpointError = errs.Class("garbage collector endpoint error")

// Endpoint implements the garbage collector endpoint
type Endpoint struct {
	log       *zap.Logger
	store     *pieces.Store
	pieceinfo pieces.DB
}

// NewEndpoint creates a new endpoint
func NewEndpoint(log *zap.Logger, store *pieces.Store, pieceinfo pieces.DB) *Endpoint {
	return &Endpoint{
		log:       log,
		store:     store,
		pieceinfo: pieceinfo,
	}
}

// Retain keeps only piece ids specified in the request
func (endpoint *Endpoint) Retain(ctx context.Context, retainReq *pb.RetainRequest) (*pb.RetainResponse, error) {
	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, EndpointError.Wrap(err)
	}

	filter, err := bloomfilter.NewFromBytes(retainReq.GetFilter())
	if err != nil {
		return nil, EndpointError.Wrap(err)
	}

	infos, err := endpoint.pieceinfo.GetAll(ctx, peer.ID)
	if err != nil {
		return nil, EndpointError.Wrap(err)
	}

	count := 0
	for _, info := range infos {
		if !filter.Contains(info.PieceID) {
			count++
			err = endpoint.store.Delete(ctx, peer.ID, info.PieceID)
			if err != nil {
				return nil, EndpointError.Wrap(err)
			}
			err = endpoint.pieceinfo.Delete(ctx, peer.ID, info.PieceID)
			if err != nil {
				return nil, EndpointError.Wrap(err)
			}
		}
	}
	fmt.Println("size = ", len(infos), " - count = ", count)
	return nil, nil
}

// PieceInfo returns pieces info db
func (endpoint *Endpoint) PieceInfo() pieces.DB {
	return endpoint.pieceinfo
}

// Store returns pieces store
func (endpoint *Endpoint) Store() *pieces.Store {
	return endpoint.store
}
