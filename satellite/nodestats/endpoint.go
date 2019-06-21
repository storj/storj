// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package nodestats

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
)

var (
	// NodeStatsEndpointErr is endpoint error class
	NodeStatsEndpointErr = errs.Class("node stats endpoint error")

	mon = monkit.Package()
)

// Endpoint for querying node stats for the SNO
type Endpoint struct {
	log     *zap.Logger
	overlay overlay.DB
}

// NewEndpoint creates new endpoint
func NewEndpoint(log *zap.Logger, overlay overlay.DB) *Endpoint {
	return &Endpoint{
		log:     log,
		overlay: overlay,
	}
}

// UptimeCheck returns uptime checks ratio
func (e *Endpoint) UptimeCheck(ctx context.Context, req *pb.UptimeRequest) (_ *pb.UptimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, NodeStatsEndpointErr.Wrap(err)
	}

	node, err := e.overlay.Get(ctx, peer.ID)
	if err != nil {
		return nil, NodeStatsEndpointErr.Wrap(err)
	}

	return &pb.UptimeResponse{
		Ratio: node.Reputation.UptimeRatio,
	}, nil
}
