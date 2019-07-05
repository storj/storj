// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package gc

import (
	"context"

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
	pieceIDs, err := endpoint.pieceinfo.GetPiecesID(ctx, peer.ID, retainReq.GetCreationDate())
	if err != nil {
		return nil, EndpointError.Wrap(err)
	}

	for _, pieceID := range pieceIDs {
		if !filter.Contains(pieceID) {
			err = errs.Combine(err, endpoint.store.Delete(ctx, peer.ID, pieceID))
			err = errs.Combine(endpoint.pieceinfo.Delete(ctx, peer.ID, pieceID))
		}
	}
	return &pb.RetainResponse{}, EndpointError.Wrap(err)
}

// PieceInfo returns pieces info db
func (endpoint *Endpoint) PieceInfo() pieces.DB {
	return endpoint.pieceinfo
}

// Store returns pieces store
func (endpoint *Endpoint) Store() *pieces.Store {
	return endpoint.store
}
