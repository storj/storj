// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinode

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/private/multinodepb"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/bandwidth"
)

var _ multinodepb.DRPCBandwidthServer = (*BandwidthEndpoint)(nil)

// BandwidthEndpoint implements multinode bandwidth endpoint.
//
// architecture: Endpoint
type BandwidthEndpoint struct {
	log     *zap.Logger
	apiKeys *apikeys.Service
	db      bandwidth.DB
}

// NewBandwidthEndpoint creates new multinode bandwidth endpoint.
func NewBandwidthEndpoint(log *zap.Logger, apiKeys *apikeys.Service, db bandwidth.DB) *BandwidthEndpoint {
	return &BandwidthEndpoint{
		log:     log,
		apiKeys: apiKeys,
		db:      db,
	}
}

// MonthSummary returns bandwidth used current month.
func (bandwidth *BandwidthEndpoint) MonthSummary(ctx context.Context, req *multinodepb.BandwidthMonthSummaryRequest) (_ *multinodepb.BandwidthMonthSummaryResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if err = authenticate(ctx, bandwidth.apiKeys, req.GetHeader()); err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Unauthenticated, err)
	}

	used, err := bandwidth.db.MonthSummary(ctx, time.Now())
	if err != nil {
		return nil, rpcstatus.Wrap(rpcstatus.Internal, err)
	}

	return &multinodepb.BandwidthMonthSummaryResponse{
		Used: used,
	}, nil
}
