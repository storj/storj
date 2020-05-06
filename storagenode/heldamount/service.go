// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/trust"
)

var (
	// ErrHeldAmountService defines held amount service error.
	ErrHeldAmountService = errs.Class("heldamount service error")

	// ErrBadPeriod defines that period has wrong format.
	ErrBadPeriod = errs.Class("wrong period format")

	mon = monkit.Package()
)

// Client encapsulates HeldAmountClient with underlying connection
//
// architecture: Client
type Client struct {
	conn *rpc.Conn
	pb.DRPCHeldAmountClient
}

// Close closes underlying client connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// TODO: separate service on service and endpoint.

// Service retrieves info from satellites using an rpc client
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db DB

	dialer rpc.Dialer
	trust  *trust.Pool
}

// NewService creates new instance of service
func NewService(log *zap.Logger, db DB, dialer rpc.Dialer, trust *trust.Pool) *Service {
	return &Service{
		log:    log,
		db:     db,
		dialer: dialer,
		trust:  trust,
	}
}

// GetPaystubStats retrieves held amount for particular satellite from satellite using grpc.
func (service *Service) GetPaystubStats(ctx context.Context, satelliteID storj.NodeID, period string) (_ *PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := service.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	requestedPeriod, err := date.PeriodToTime(period)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}

	resp, err := client.GetPayStub(ctx, &pb.GetHeldAmountRequest{Period: requestedPeriod})
	if err != nil {
		if rpcstatus.Code(err) == rpcstatus.OutOfRange {
			return nil, ErrNoPayStubForPeriod.Wrap(err)
		}

		return nil, ErrHeldAmountService.Wrap(err)
	}

	return &PayStub{
		Period:         period[0:7],
		SatelliteID:    satelliteID,
		Created:        resp.CreatedAt,
		Codes:          resp.Codes,
		UsageAtRest:    float64(resp.UsageAtRest),
		UsageGet:       resp.UsageGet,
		UsagePut:       resp.UsagePut,
		UsageGetRepair: resp.CompGetRepair,
		UsagePutRepair: resp.CompPutRepair,
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
	}, nil
}

// GetAllPaystubs retrieves all paystubs for particular satellite.
func (service *Service) GetAllPaystubs(ctx context.Context, satelliteID storj.NodeID) (_ []PayStub, err error) {
	defer mon.Task()(&ctx)(&err)

	client, err := service.dial(ctx, satelliteID)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}
	defer func() { err = errs.Combine(err, client.Close()) }()

	resp, err := client.GetAllPaystubs(ctx, &pb.GetAllPaystubsRequest{})
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}

	var payStubs []PayStub

	for i := 0; i < len(resp.Paystub); i++ {
		paystub := PayStub{
			Period:         resp.Paystub[i].Period.String()[0:7],
			SatelliteID:    satelliteID,
			Created:        resp.Paystub[i].CreatedAt,
			Codes:          resp.Paystub[i].Codes,
			UsageAtRest:    float64(resp.Paystub[i].UsageAtRest),
			UsageGet:       resp.Paystub[i].UsageGet,
			UsagePut:       resp.Paystub[i].UsagePut,
			UsageGetRepair: resp.Paystub[i].CompGetRepair,
			UsagePutRepair: resp.Paystub[i].CompPutRepair,
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
		}

		payStubs = append(payStubs, paystub)
	}

	return payStubs, nil
}

// SatellitePayStubMonthlyCached retrieves held amount for particular satellite for selected month from storagenode database.
func (service *Service) SatellitePayStubMonthlyCached(ctx context.Context, satelliteID storj.NodeID, period string) (payStub *PayStub, err error) {
	defer mon.Task()(&ctx, &satelliteID, &period)(&err)

	payStub, err = service.db.GetPayStub(ctx, satelliteID, period)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}

	return payStub, nil
}

// AllPayStubsMonthlyCached retrieves held amount for all satellites per selected period from storagenode database.
func (service *Service) AllPayStubsMonthlyCached(ctx context.Context, period string) (payStubs []PayStub, err error) {
	defer mon.Task()(&ctx, &period)(&err)

	payStubs, err = service.db.AllPayStubs(ctx, period)
	if err != nil {
		return payStubs, ErrHeldAmountService.Wrap(err)
	}

	return payStubs, nil
}

// SatellitePayStubPeriodCached retrieves held amount for all satellites for selected months from storagenode database.
func (service *Service) SatellitePayStubPeriodCached(ctx context.Context, satelliteID storj.NodeID, periodStart, periodEnd string) (payStubs []PayStub, err error) {
	defer mon.Task()(&ctx, &satelliteID, &periodStart, &periodEnd)(&err)

	periods, err := parsePeriodRange(periodStart, periodEnd)
	if err != nil {
		return []PayStub{}, err
	}

	for _, period := range periods {
		payStub, err := service.db.GetPayStub(ctx, satelliteID, period)
		if err != nil {
			if ErrNoPayStubForPeriod.Has(err) {
				continue
			}

			return []PayStub{}, ErrHeldAmountService.Wrap(err)
		}

		payStubs = append(payStubs, *payStub)
	}

	return payStubs, nil
}

// AllPayStubsPeriodCached retrieves held amount for all satellites for selected range of months from storagenode database.
func (service *Service) AllPayStubsPeriodCached(ctx context.Context, periodStart, periodEnd string) (payStubs []PayStub, err error) {
	defer mon.Task()(&ctx, &periodStart, &periodEnd)(&err)

	periods, err := parsePeriodRange(periodStart, periodEnd)
	if err != nil {
		return []PayStub{}, err
	}

	for _, period := range periods {
		payStub, err := service.db.AllPayStubs(ctx, period)
		if err != nil {
			if ErrNoPayStubForPeriod.Has(err) {
				continue
			}

			return []PayStub{}, ErrHeldAmountService.Wrap(err)
		}

		payStubs = append(payStubs, payStub...)
	}

	return payStubs, nil
}

// HeldbackPeriod amount of held for specific percent rate period.
type HeldbackPeriod struct {
	PercentageRate int
	Held           int64
}

// AllHeldbackHistory retrieves heldback history for all specific satellite from storagenode database.
func (service *Service) AllHeldbackHistory(ctx context.Context, id storj.NodeID) (result []HeldbackPeriod, err error) {
	defer mon.Task()(&ctx, &id)(&err)

	heldback, err := service.db.SatellitesHeldbackHistory(ctx, id)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}

	var total75, total50, total25, total0 int64

	for i, t := range heldback {
		switch i {
		case 0, 1, 2:
			total75 += t.Held
		case 3, 4, 5:
			total50 += t.Held
		case 6, 7, 8:
			total25 += t.Held
		default:
			total0 += t.Held
		}
	}

	period75percent := HeldbackPeriod{
		PercentageRate: 75,
		Held:           total75,
	}
	period50percent := HeldbackPeriod{
		PercentageRate: 50,
		Held:           total50,
	}
	period25percent := HeldbackPeriod{
		PercentageRate: 25,
		Held:           total25,
	}
	period0percent := HeldbackPeriod{
		PercentageRate: 0,
		Held:           total0,
	}

	result = append(result, period75percent)

	switch {
	case len(heldback) > 3:
		result = append(result, period50percent)
	case len(heldback) > 6:
		result = append(result, period25percent)
	case len(heldback) > 9:
		result = append(result, period0percent)
	}

	return result, nil
}

// dial dials the HeldAmount client for the satellite by id
func (service *Service) dial(ctx context.Context, satelliteID storj.NodeID) (_ *Client, err error) {
	defer mon.Task()(&ctx)(&err)

	address, err := service.trust.GetAddress(ctx, satelliteID)
	if err != nil {
		return nil, errs.New("unable to find satellite %s: %w", satelliteID, err)
	}

	conn, err := service.dialer.DialAddressID(ctx, address, satelliteID)
	if err != nil {
		return nil, errs.New("unable to connect to the satellite %s: %w", satelliteID, err)
	}

	return &Client{
		conn:                 conn,
		DRPCHeldAmountClient: pb.NewDRPCHeldAmountClient(conn),
	}, nil
}

// TODO: move to separate struct.
func parsePeriodRange(periodStart, periodEnd string) (periods []string, err error) {
	var yearStart, yearEnd, monthStart, monthEnd int

	start := strings.Split(periodStart, "-")
	if len(start) != 2 {
		return nil, ErrBadPeriod.New("period start has wrong format")
	}
	end := strings.Split(periodEnd, "-")
	if len(start) != 2 {
		return nil, ErrBadPeriod.New("period end has wrong format")
	}

	yearStart, err = strconv.Atoi(start[0])
	if err != nil {
		return nil, ErrBadPeriod.New("period start has wrong format")
	}
	monthStart, err = strconv.Atoi(start[1])
	if err != nil || monthStart > 12 || monthStart < 1 {
		return nil, ErrBadPeriod.New("period start has wrong format")
	}
	yearEnd, err = strconv.Atoi(end[0])
	if err != nil {
		return nil, ErrBadPeriod.New("period end has wrong format")
	}
	monthEnd, err = strconv.Atoi(end[1])
	if err != nil || monthEnd > 12 || monthEnd < 1 {
		return nil, ErrBadPeriod.New("period end has wrong format")
	}
	if yearEnd < yearStart {
		return nil, ErrBadPeriod.New("period has wrong format")
	}
	if yearEnd == yearStart && monthEnd < monthStart {
		return nil, ErrBadPeriod.New("period has wrong format")
	}

	for ; yearStart <= yearEnd; yearStart++ {
		lastMonth := 12
		if yearStart == yearEnd {
			lastMonth = monthEnd
		}
		for ; monthStart <= lastMonth; monthStart++ {
			format := "%d-%d"
			if monthStart < 10 {
				format = "%d-0%d"
			}
			periods = append(periods, fmt.Sprintf(format, yearStart, monthStart))
		}

		monthStart = 1
	}

	return periods, nil
}
