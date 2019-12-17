// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/rpc/rpcstatus"
	"storj.io/storj/satellite/overlay"
)

type Endpoint struct {
	log     *zap.Logger
	dialer  rpc.Dialer
	overlay *overlay.Service
}

func NewEndpoint(log *zap.Logger, dialer rpc.Dialer, overlay *overlay.Service) *Endpoint {
	return &Endpoint{
		log:     log,
		dialer:  dialer,
		overlay: overlay,
	}
}

func (endpoint *Endpoint) Notify(ctx context.Context, req *pb.NotifyRequest) (_ *pb.NotifyResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	overlayErr := func(err error) (*pb.NotifyResponse, error) {
		switch {
		case err == overlay.ErrEmptyNode:
			return nil, rpcstatus.Error(rpcstatus.InvalidArgument, err.Error())
		case overlay.ErrNodeNotFound.Has(err):
			return nil, rpcstatus.Error(rpcstatus.NotFound, err.Error())
		default:
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	node, err := endpoint.overlay.Get(ctx, req.NodeId)
	if err != nil {
		return overlayErr(err)
	}

	client, err := NewClient(ctx, endpoint.dialer, req.NodeId, node.GetAddress().GetAddress())
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	defer func() {
		if err = errs.Combine(err, client.Close()); err != nil {
			err = rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}()

	if _, err = client.Notify(ctx, req.GetNotification()); err != nil {
		return nil, err
	}

	return &pb.NotifyResponse{}, nil
}

func (endpoint *Endpoint) Broadcast(ctx context.Context, notification *pb.Notification) (_ *pb.BroadcastResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if notification == nil {
		return nil, rpcstatus.Error(rpcstatus.InvalidArgument, "notification can not be nil")
	}

	nodes, err := endpoint.overlay.ReliableWithAddress(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	response := new(pb.BroadcastResponse)

	for nodeID, address := range nodes {
		if err = ctx.Err(); err != nil {
			return response, rpcstatus.Error(rpcstatus.Canceled, err.Error())
		}

		client, err := NewClient(ctx, endpoint.dialer, nodeID, address)
		if err != nil {
			response.Offline = append(response.Offline, nodeID)
			continue
		}

		if _, err = client.Notify(ctx, notification); err != nil {
			response.Failed = append(response.Failed, nodeID)
			continue
		}

		response.SuccessCount += 1

		if err = client.Close(); err != nil {
			return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
		}
	}

	return response, nil
}
