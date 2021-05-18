// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
)

var mon = monkit.Package()

type Endpoint struct {
	log     *zap.Logger
	service *Service
}

func NewEndpoint(log *zap.Logger, service *Service) *Endpoint {
	return &Endpoint{
		log:     log,
		service: service,
	}
}

// Maybe we don't want this
// func (endpoint *Endpoint) GetScheduled(ctx context.Context, req *pb.GetScheduledRequest) (_ *pb.GetScheduledResponse, err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	peer, err := identity.PeerIdentityFromContext(ctx)
// 	if err != nil {
// 		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
// 	}

// 	return endpoint.service.GetScheduled(ctx, peer.ID)
// }

// Maybe we don't want this
// func (endpoint *Endpoint) GetAvailable(ctx context.Context, req *pb.GetAvailableRequest) (_ *pb.GetAvailableResponse, err error) {
// 	defer mon.Task()(&ctx)(&err)

// 	peer, err := identity.PeerIdentityFromContext(ctx)
// 	if err != nil {
// 		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
// 	}

// 	return endpoint.service.GetAvailable(ctx, peer.ID, req)
// }

func (endpoint *Endpoint) ScheduleDowntime(ctx context.Context, req *pb.ScheduleDowntimeRequest) (_ *pb.ScheduleDowntimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	return endpoint.service.ScheduleDowntime(ctx, peer.ID, req)
}

// Cancel deletes a scheduled timeframe from the DB
func (endpoint *Endpoint) Cancel(ctx context.Context, req *pb.CancelRequest) (_ *pb.CancelResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	peer, err := identity.PeerIdentityFromContext(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Unauthenticated, err.Error())
	}

	return endpoint.service.Cancel(ctx, peer.ID, req)
}
