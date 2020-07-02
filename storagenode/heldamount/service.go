// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/trust"
)

var (
	// ErrHeldAmountService defines held amount service error.
	ErrHeldAmountService = errs.Class("heldamount service error")

	// ErrBadPeriod defines that period has wrong format.
	ErrBadPeriod = errs.Class("wrong period format")

	mon = monkit.Package()
)

// Service retrieves info from satellites using an rpc client
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db           DB
	reputationDB reputation.DB
	trust        *trust.Pool
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, db DB, reputationDB reputation.DB, trust *trust.Pool) *Service {
	return &Service{
		log:          log,
		db:           db,
		reputationDB: reputationDB,
		trust:        trust,
	}
}

// SatellitePayStubMonthly retrieves held amount for particular satellite for selected month from storagenode database.
func (service *Service) SatellitePayStubMonthly(ctx context.Context, satelliteID storj.NodeID, period string) (payStub *PayStub, err error) {
	defer mon.Task()(&ctx, &satelliteID, &period)(&err)

	payStub, err = service.db.GetPayStub(ctx, satelliteID, period)
	if err != nil {
		return nil, ErrHeldAmountService.Wrap(err)
	}

	return payStub, nil
}

// AllPayStubsMonthly retrieves held amount for all satellites per selected period from storagenode database.
func (service *Service) AllPayStubsMonthly(ctx context.Context, period string) (payStubs []PayStub, err error) {
	defer mon.Task()(&ctx, &period)(&err)

	payStubs, err = service.db.AllPayStubs(ctx, period)
	if err != nil {
		return payStubs, ErrHeldAmountService.Wrap(err)
	}

	return payStubs, nil
}

// SatellitePayStubPeriod retrieves held amount for all satellites for selected months from storagenode database.
func (service *Service) SatellitePayStubPeriod(ctx context.Context, satelliteID storj.NodeID, periodStart, periodEnd string) (payStubs []PayStub, err error) {
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

// AllPayStubsPeriod retrieves held amount for all satellites for selected range of months from storagenode database.
func (service *Service) AllPayStubsPeriod(ctx context.Context, periodStart, periodEnd string) (payStubs []PayStub, err error) {
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

// SatellitePeriods retrieves all periods for concrete satellite in which we have some heldamount data.
func (service *Service) SatellitePeriods(ctx context.Context, satelliteID storj.NodeID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.SatellitePeriods(ctx, satelliteID)
}

// AllPeriods retrieves all periods in which we have some heldamount data.
func (service *Service) AllPeriods(ctx context.Context) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.AllPeriods(ctx)
}

// HeldHistory amount of held for specific percent rate period.
type HeldHistory struct {
	SatelliteID   storj.NodeID `json:"satelliteID"`
	SatelliteName string       `json:"satelliteName"`
	Age           int64        `json:"age"`
	FirstPeriod   int64        `json:"firstPeriod"`
	SecondPeriod  int64        `json:"secondPeriod"`
	ThirdPeriod   int64        `json:"thirdPeriod"`
	TotalHeld     int64        `json:"totalHeld"`
	TotalDisposed int64        `json:"totalDisposed"`
	JoinedAt      time.Time    `json:"joinedAt"`
}

// AllHeldbackHistory retrieves heldback history for all satellites from storagenode database.
func (service *Service) AllHeldbackHistory(ctx context.Context) (result []HeldHistory, err error) {
	defer mon.Task()(&ctx)(&err)

	satellites := service.trust.GetSatellites(ctx)
	for i := 0; i < len(satellites); i++ {
		var history HeldHistory

		heldback, err := service.db.SatellitesHeldbackHistory(ctx, satellites[i])
		if err != nil {
			return nil, ErrHeldAmountService.Wrap(err)
		}

		disposed, err := service.db.SatellitesDisposedHistory(ctx, satellites[i])
		if err != nil {
			return nil, ErrHeldAmountService.Wrap(err)
		}

		for i, t := range heldback {
			switch i {
			case 0, 1, 2:
				history.FirstPeriod += t.Held
				history.TotalHeld += t.Held
			case 3, 4, 5:
				history.SecondPeriod += t.Held
				history.TotalHeld += t.Held
			case 6, 7, 8:
				history.ThirdPeriod += t.Held
				history.TotalHeld += t.Held
			default:
			}
		}

		history.TotalDisposed = disposed
		history.SatelliteID = satellites[i]
		url, err := service.trust.GetNodeURL(ctx, satellites[i])
		if err != nil {
			return nil, ErrHeldAmountService.Wrap(err)
		}

		stats, err := service.reputationDB.Get(ctx, satellites[i])
		if err != nil {
			return nil, ErrHeldAmountService.Wrap(err)
		}

		history.Age = int64(date.MonthsCountSince(stats.JoinedAt))
		history.SatelliteName = url.Address
		history.JoinedAt = stats.JoinedAt

		result = append(result, history)
	}

	return result, nil
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
