// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/payouts"
)

var _ multinodepb.DRPCPayoutServer = (*PayoutEndpoint)(nil)

// PayoutEndpoint implements multinode payouts endpoint.
//
// architecture: Endpoint
type PayoutEndpoint struct {
	multinodepb.DRPCPayoutUnimplementedServer

	log     *zap.Logger
	apiKeys *apikeys.Service
	db      payouts.DB
}

// NewPayoutEndpoint creates new multinode payouts endpoint.
func NewPayoutEndpoint(log *zap.Logger, apiKeys *apikeys.Service, db payouts.DB) *PayoutEndpoint {
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

// EarnedPerSatellite returns total earned amount per satellite.
func (payout *PayoutEndpoint) EarnedPerSatellite(ctx context.Context, req *multinodepb.EarnedPerSatelliteRequest) (_ *multinodepb.EarnedPerSatelliteResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, payout.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	var resp multinodepb.EarnedPerSatelliteResponse
	satelliteIDs, err := payout.db.GetPayingSatellitesIDs(ctx)
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	for i := 0; i < len(satelliteIDs); i++ {
		earned, err := payout.db.GetEarnedAtSatellite(ctx, satelliteIDs[i])
		if err != nil {
			return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
		}

		resp.EarnedSatellite = append(resp.EarnedSatellite, &multinodepb.EarnedSatellite{
			Total:       earned,
			SatelliteId: satelliteIDs[i],
		})
	}

	return &resp, nil
}
