// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/trust"
)

// Client encapsulates HeldAmountClient with underlying connection.
//
// architecture: Client
type Client struct {
	conn *rpc.Conn
	pb.DRPCHeldAmountClient
}

// Close closes underlying client connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Endpoint retrieves info from satellites using an rpc client.
//
// architecture: Endpoint
type Endpoint struct {
	log *zap.Logger

	dialer rpc.Dialer
	trust  *trust.Pool
}

// NewEndpoint creates new instance of endpoint.
func NewEndpoint(log *zap.Logger, dialer rpc.Dialer, trust *trust.Pool) *Endpoint {
	return &Endpoint{
		log:    log,
		dialer: dialer,
		trust:  trust,
	}
}

// GetPaystub retrieves held amount for particular satellite from satellite using RPC.
func (endpoint *Endpoint) GetPaystub(ctx context.Context, satelliteID storj.NodeID, period string) (_ *PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := endpoint.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	requestedPeriod, err := date.PeriodToTime(period)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	resp, err := client.GetPayStub(ctx, &pb.GetHeldAmountRequest{Period: requestedPeriod})
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.OutOfRange {
			return nil, ErrNoPayStubForPeriod.Wrap(err)
		}

		return nil, ErrPayoutService.Wrap(err)
	}

	return &PayStub{
		Period:         period[0:7],
		SatelliteID:    satelliteID,
		Created:        resp.CreatedAt,
		Codes:          resp.Codes,
		UsageAtRest:    resp.UsageAtRest,
		UsageGet:       resp.UsageGet,
		UsagePut:       resp.UsagePut,
		UsageGetRepair: resp.UsageGetRepair,
		UsagePutRepair: resp.UsagePutRepair,
		UsageGetAudit:  resp.UsageGetAudit,
		CompAtRest:     resp.CompAtRest,
		CompGet:        resp.CompGet,
		CompPut:        resp.CompPut,
		CompGetRepair:  resp.CompGetRepair,
		CompPutRepair:  resp.CompPutRepair,
		CompGetAudit:   resp.CompGetAudit,
		SurgePercent:   resp.SurgePercent,
		Held:           resp.Held,
		Owed:           resp.Owed,
		Disposed:       resp.Disposed,
		Paid:           resp.Paid,
		Distributed:    resp.Distributed,
	}, nil
}

// GetAllPaystubs retrieves all paystubs for particular satellite.
func (endpoint *Endpoint) GetAllPaystubs(ctx context.Context, satelliteID storj.NodeID) (_ []PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := endpoint.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	resp, err := client.GetAllPaystubs(ctx, &pb.GetAllPaystubsRequest{})
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	var payStubs []PayStub

	for i := 0; i < len(resp.Paystub); i++ {
		paystub := PayStub{
			Period:         resp.Paystub[i].Period.String()[0:7],
			SatelliteID:    satelliteID,
			Created:        resp.Paystub[i].CreatedAt,
			Codes:          resp.Paystub[i].Codes,
			UsageAtRest:    resp.Paystub[i].UsageAtRest,
			UsageGet:       resp.Paystub[i].UsageGet,
			UsagePut:       resp.Paystub[i].UsagePut,
			UsageGetRepair: resp.Paystub[i].UsageGetRepair,
			UsagePutRepair: resp.Paystub[i].UsagePutRepair,
			UsageGetAudit:  resp.Paystub[i].UsageGetAudit,
			CompAtRest:     resp.Paystub[i].CompAtRest,
			CompGet:        resp.Paystub[i].CompGet,
			CompPut:        resp.Paystub[i].CompPut,
			CompGetRepair:  resp.Paystub[i].CompGetRepair,
			CompPutRepair:  resp.Paystub[i].CompPutRepair,
			CompGetAudit:   resp.Paystub[i].CompGetAudit,
			SurgePercent:   resp.Paystub[i].SurgePercent,
			Held:           resp.Paystub[i].Held,
			Owed:           resp.Paystub[i].Owed,
			Disposed:       resp.Paystub[i].Disposed,
			Paid:           resp.Paystub[i].Paid,
			Distributed:    resp.Paystub[i].Distributed,
		}

		payStubs = append(payStubs, paystub)
	}

	return payStubs, nil
}

// GetPayment retrieves payment data from particular satellite using grpc.
func (endpoint *Endpoint) GetPayment(ctx context.Context, satelliteID storj.NodeID, period string) (_ *Payment, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := endpoint.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	requestedPeriod, err := date.PeriodToTime(period)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	resp, err := client.GetPayment(ctx, &pb.GetPaymentRequest{Period: requestedPeriod})
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.OutOfRange {
			return nil, nil
		}

		return nil, ErrPayoutService.Wrap(err)
	}

	return &Payment{
		ID:          resp.Id,
		Created:     resp.CreatedAt,
		SatelliteID: satelliteID,
		Period:      period[0:7],
		Amount:      resp.Amount,
		Receipt:     resp.Receipt,
		Notes:       resp.Notes,
	}, nil
}

// GetAllPayments retrieves all payments for particular satellite.
func (endpoint *Endpoint) GetAllPayments(ctx context.Context, satelliteID storj.NodeID) (_ []Payment, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := endpoint.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	resp, err := client.GetAllPayments(ctx, &pb.GetAllPaymentsRequest{})
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	var payments []Payment

	for i := 0; i < len(resp.Payment); i++ {
		payment := Payment{
			ID:          resp.Payment[i].Id,
			Created:     resp.Payment[i].CreatedAt,
			SatelliteID: satelliteID,
			Period:      resp.Payment[i].Period.String()[0:7],
			Amount:      resp.Payment[i].Amount,
			Receipt:     resp.Payment[i].Receipt,
			Notes:       resp.Payment[i].Notes,
		}

		payments = append(payments, payment)
	}

	return payments, nil
}

// dial dials the SnoPayout client for the satellite by id.
func (endpoint *Endpoint) dial(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeurl, err := endpoint.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %w", satelliteID, err)
	}

	conn, err := endpoint.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %w", satelliteID, err)
	}

	return &Client{
		conn:                 conn,
		DRPCHeldAmountClient: pb.NewDRPCHeldAmountClient(conn),
	}, nil
}
