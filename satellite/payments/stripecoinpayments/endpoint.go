// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// Endpoint
type Endpoint struct {
	service *Service
}

func NewEndpoint(service *Service) *Endpoint {
	return &Endpoint{service:service}
}

func (endpoint *Endpoint) PrepareInvoiceRecords(ctx context.Context, req *pb.PrepareInvoiceRecordsRequest) (_ *pb.PrepareInvoiceRecordsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.PrepareInvoiceProjectRecords(ctx, req.Period)
	if err != nil {
		return nil, err
	}

	return &pb.PrepareInvoiceRecordsResponse{}, nil
}

func (endpoint *Endpoint) ApplyInvoiceRecords(ctx context.Context, req *pb.ApplyInvoiceRecordsRequest) (_ *pb.ApplyInvoiceRecordsResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.InvoiceApplyProjectRecords(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.ApplyInvoiceRecordsResponse{}, nil
}

func (endpoint *Endpoint) CreateInvoices(ctx context.Context, req *pb.CreateInvoicesRequest) (_ *pb.CreateInvoicesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	err = endpoint.service.CreateInvoices(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.CreateInvoicesResponse{}, nil
}
