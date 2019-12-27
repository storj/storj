// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"storj.io/common/pb"
	"storj.io/common/rpc/rpcstatus"
)

// Endpoint is stripecoinpayments private RPC server payments endpoint.
type Endpoint struct {
	service *Service
}

// NewEndpoint creates new endpoint.
func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{service: service}
}

// PrepareInvoiceRecords creates project invoice records for all satellite projects.
func (endpoint *Endpoint) PrepareInvoiceRecords(ctx context.Context, req *pb.PrepareInvoiceRecordsRequest) (_ *pb.PrepareInvoiceRecordsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.PrepareInvoiceProjectRecords(ctx, req.Period)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.PrepareInvoiceRecordsResponse{}, nil
}

// ApplyInvoiceRecords creates stripe line items for all unapplied invoice project records.
func (endpoint *Endpoint) ApplyInvoiceRecords(ctx context.Context, req *pb.ApplyInvoiceRecordsRequest) (_ *pb.ApplyInvoiceRecordsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.InvoiceApplyProjectRecords(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.ApplyInvoiceRecordsResponse{}, nil
}

// CreateInvoices creates invoice for all user accounts on the satellite.
func (endpoint *Endpoint) CreateInvoices(ctx context.Context, req *pb.CreateInvoicesRequest) (_ *pb.CreateInvoicesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.CreateInvoices(ctx)
	if err != nil {
		return nil, rpcstatus.Error(rpcstatus.Internal, err.Error())
	}

	return &pb.CreateInvoicesResponse{}, nil
}
