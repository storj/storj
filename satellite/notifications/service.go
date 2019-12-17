// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/overlay"
)

var (
	mon = monkit.Package()

	Error = errs.Class("notification service error")
)

type Service struct {
	log     *zap.Logger
	dialer  rpc.Dialer
	overlay *overlay.Service
	sending sync.WaitGroup
}

func NewService(log *zap.Logger, dialer rpc.Dialer, overlay *overlay.Service) *Service {
	return &Service{
		log:     log,
		dialer:  dialer,
		overlay: overlay,
		sending: sync.WaitGroup{},
	}
}

func (service *Service) Notify(ctx context.Context, nodeID storj.NodeID, notification *pb.Notification) {
	defer mon.Task()(&ctx)(nil)

	// TODO: add limits
	service.sending.Add(1)
	go func() {
		defer service.sending.Done()

		if err := service.notify(ctx, nodeID, notification); err != nil {
			service.log.Debug(
				"failed to send notification",
				zap.String("nodeID", nodeID.String()),
				zap.Error(err),
			)
		}
	}()
}

func (service *Service) Close() error {
	service.sending.Wait()
	return nil
}

func (service *Service) notify(ctx context.Context, nodeID storj.NodeID, notification *pb.Notification) (err error) {
	defer mon.Task()(&ctx)(&err)

	node, err := service.overlay.Get(ctx, nodeID)
	if err != nil {
		return Error.Wrap(err)
	}

	client, err := NewClient(ctx, service.dialer, nodeID, node.GetAddress().GetAddress())
	if err != nil {
		return Error.Wrap(err)
	}

	defer func() {
		err = Error.Wrap(errs.Combine(err, client.Close()))
	}()

	if _, err = client.Notify(ctx, notification); err != nil {
		return Error.Wrap(err)
	}

	return nil
}
