// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/payout"
)

var _ multinodepb.DRPCPayoutServer = (*PayoutEndpoint)(nil)

// PayoutEndpoint implements multinode payout endpoint.
//
// architecture: Endpoint
type PayoutEndpoint struct {
	log     *zap.Logger
	apiKeys *apikeys.Service
	db      payout.DB
}

// NewPayoutEndpoint creates new multinode payout endpoint.
func NewPayoutEndpoint(log *zap.Logger, apiKeys *apikeys.Service, db payout.DB) *PayoutEndpoint {
	return &PayoutEndpoint{
		log:     log,
		apiKeys: apiKeys,
		db:      db,
	}
}

// Earned returns total earned amount.
func (payout *PayoutEndpoint) Earned(ctx context.Context, req *multinodepb.EarnedRequest) (_ *multinodepb.EarnedResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	earned, err := payout.db.GetTotalEarned(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.EarnedResponse{
		Total: earned,
	}, nil
}
