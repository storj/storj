// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector

import (
	"context"

	"github.com/golang/protobuf/ptypes"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/console"
)

type PaymentsEndpoint struct {
	log     *zap.Logger
	console *console.Service
}

func NewPaymentsEndpoint(log *zap.Logger, console *console.Service) *PaymentsEndpoint {
	return &PaymentsEndpoint{
		log:     log,
		console: console,
	}
}

func (srv *PaymentsEndpoint) CreateInvoices(ctx context.Context, req *pb.CreateInvoicesRequest) (*pb.CreateInvoicesResponse, error) {
	baseDate, err := ptypes.Timestamp(req.GetBaseDate())
	if err != nil {
		return nil, err
	}

	err = srv.console.CreateMonthlyProjectInvoices(ctx, baseDate)
	if err != nil {
		return nil, err
	}

	return &pb.CreateInvoicesResponse{}, nil
}
