// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
)

var (
	mon = monkit.Package()
)

type Endpoint struct {
	log *zap.Logger
}

func NewEndpoint(log *zap.Logger) *Endpoint {
	return &Endpoint{log: log}
}

func (endpoint *Endpoint) Notify(ctx context.Context, notification *pb.Notification) (_ *pb.NotifyResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	endpoint.log.Sugar().Debug("notification", notification)
	return new(pb.NotifyResponse), nil
}

func (endpoint *Endpoint) Report(ctx context.Context, req *pb.ReportRequest) (_ *pb.ReportResponse, err error) {
	defer mon.Task()(&ctx)(&err)
	endpoint.log.Sugar().Debug("report", req)
	return new(pb.ReportResponse), nil
}
