// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"context"
	"database/sql"
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
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/trust"
)

var (
	// ErrPayoutService defines payout service error.
	ErrPayoutService = errs.Class("payouts service")

	// ErrBadPeriod defines that period has wrong format.
	ErrBadPeriod = errs.Class("wrong period format")

	mon = monkit.Package()
)

// Service retrieves info from satellites using an rpc client.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	stefanSatellite storj.NodeID

	db           DB
	reputationDB reputation.DB
	satellitesDB satellites.DB
	trust        *trust.Pool
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, db DB, reputationDB reputation.DB, satelliteDB satellites.DB, trust *trust.Pool) (_ *Service, err error) {
	id, err := storj.NodeIDFromString("118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW")
	if err != nil {
		return &Service{}, err
	}

	return &Service{
		log:             log,
		stefanSatellite: id,
		db:              db,
		reputationDB:    reputationDB,
		satellitesDB:    satelliteDB,
		trust:           trust,
	}, nil
}

// SatellitePayStubMonthly retrieves held amount for particular satellite for selected month from storagenode database.
func (service *Service) SatellitePayStubMonthly(ctx context.Context, satelliteID storj.NodeID, period string) (payStub *PayStub, err error) {
	defer mon.Task()(&ctx, &satelliteID, &period)(&err)

	payStub, err = service.db.GetPayStub(ctx, satelliteID, period)
	if err != nil {
		if ErrNoPayStubForPeriod.Has(err) {
			return nil, nil
		}

		return nil, ErrPayoutService.Wrap(err)
	}

	payStub.UsageAtRestTbM()

	return payStub, nil
}

// AllPayStubsMonthly retrieves held amount for all satellites per selected period from storagenode database.
func (service *Service) AllPayStubsMonthly(ctx context.Context, period string) (payStubs []PayStub, err error) {
	defer mon.Task()(&ctx, &period)(&err)

	payStubs, err = service.db.AllPayStubs(ctx, period)
	if err != nil {
		return payStubs, ErrPayoutService.Wrap(err)
	}

	for i := 0; i < len(payStubs); i++ {
		payStubs[i].UsageAtRestTbM()
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

			return []PayStub{}, ErrPayoutService.Wrap(err)
		}

		payStubs = append(payStubs, *payStub)
	}

	for i := 0; i < len(payStubs); i++ {
		payStubs[i].UsageAtRestTbM()
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

			return []PayStub{}, ErrPayoutService.Wrap(err)
		}

		payStubs = append(payStubs, payStub...)
	}

	for i := 0; i < len(payStubs); i++ {
		payStubs[i].UsageAtRestTbM()
	}

	return payStubs, nil
}

// SatellitePeriods retrieves all periods for concrete satellite in which we have some payouts data.
func (service *Service) SatellitePeriods(ctx context.Context, satelliteID storj.NodeID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.SatellitePeriods(ctx, satelliteID)
}

// AllPeriods retrieves all periods in which we have some payouts data.
func (service *Service) AllPeriods(ctx context.Context) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.db.AllPeriods(ctx)
}

// AllHeldbackHistory retrieves heldback history for all satellites from storagenode database.
func (service *Service) AllHeldbackHistory(ctx context.Context) (result []SatelliteHeldHistory, err error) {
	defer mon.Task()(&ctx)(&err)
	satellitesIDs := service.trust.GetSatellites(ctx)

	satellitesIDs = append(satellitesIDs, service.stefanSatellite)
	for i := 0; i < len(satellitesIDs); i++ {
		var history SatelliteHeldHistory

		helds, err := service.db.SatellitesHeldbackHistory(ctx, satellitesIDs[i])
		if err != nil {
			return nil, ErrPayoutService.Wrap(err)
		}

		if helds == nil {
			continue
		}

		disposed, err := service.db.SatellitesDisposedHistory(ctx, satellitesIDs[i])
		if err != nil {
			return nil, ErrPayoutService.Wrap(err)
		}

		for i, amountPeriod := range helds {
			switch i {
			case 0, 1, 2:
				history.HoldForFirstPeriod += amountPeriod.Amount
				history.TotalHeld += amountPeriod.Amount
			case 3, 4, 5:
				history.HoldForSecondPeriod += amountPeriod.Amount
				history.TotalHeld += amountPeriod.Amount
			case 6, 7, 8:
				history.HoldForThirdPeriod += amountPeriod.Amount
				history.TotalHeld += amountPeriod.Amount
			default:
			}
		}

		history.TotalDisposed = disposed
		history.SatelliteID = satellitesIDs[i]
		history.SatelliteName = "stefan-benten"

		if satellitesIDs[i] != service.stefanSatellite {
			url, err := service.trust.GetNodeURL(ctx, satellitesIDs[i])
			if err != nil {
				return nil, ErrPayoutService.Wrap(err)
			}
			history.SatelliteName = url.Address
		}

		stats, err := service.reputationDB.Get(ctx, satellitesIDs[i])
		if err != nil {
			return nil, ErrPayoutService.Wrap(err)
		}

		history.JoinedAt = stats.JoinedAt.Round(time.Minute)
		result = append(result, history)
	}

	return result, nil
}

// AllSatellitesPayoutPeriod retrieves paystub and payment receipt for specific month from all satellites.
func (service *Service) AllSatellitesPayoutPeriod(ctx context.Context, period string) (result []SatellitePayoutForPeriod, err error) {
	defer mon.Task()(&ctx)(&err)

	paystubs, err := service.AllPayStubsMonthly(ctx, period)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	for _, paystub := range paystubs {
		receipt, err := service.db.GetReceipt(ctx, paystub.SatelliteID, period)
		if err != nil {
			if !ErrNoPayStubForPeriod.Has(err) {
				return nil, ErrPayoutService.Wrap(err)
			}
		}

		stats, err := service.reputationDB.Get(ctx, paystub.SatelliteID)
		if err != nil {
			return nil, ErrPayoutService.Wrap(err)
		}

		satellite, err := service.satellitesDB.GetSatellite(ctx, paystub.SatelliteID)
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			return nil, ErrPayoutService.Wrap(err)
		}

		isExitComplete := false
		if satellite.Status == satellites.ExitSucceeded {
			isExitComplete = true
		}

		satelliteURL := satellite.Address

		if satelliteURL == "" {
			url, err := service.trust.GetNodeURL(ctx, paystub.SatelliteID)
			if err != nil && !errs.Is(err, trust.ErrUntrusted) {
				return nil, ErrPayoutService.Wrap(err)
			}

			satelliteURL = url.Address
			// if satellite is not in trusted list, and we don't know the address,
			// we display the satelliteID as the URL.
			if satelliteURL == "" {
				satelliteURL = paystub.SatelliteID.String()
			}
		}

		payoutForPeriod, err := PaystubToSatellitePayoutForPeriod(paystub, stats.JoinedAt, receipt, satelliteURL, isExitComplete)
		if err != nil {
			return nil, ErrPayoutService.Wrap(err)
		}
		result = append(result, payoutForPeriod)
	}

	return result, nil
}

// HeldAmountHistory retrieves held amount history for all satellites.
func (service *Service) HeldAmountHistory(ctx context.Context) (_ []HeldAmountHistory, err error) {
	defer mon.Task()(&ctx)(&err)

	heldHistory, err := service.db.HeldAmountHistory(ctx)
	if err != nil {
		return nil, ErrPayoutService.Wrap(err)
	}

	trustedSatellites := service.trust.GetSatellites(ctx)

	for _, trustedSatellite := range trustedSatellites {
		var found bool

		for _, satelliteHeldHistory := range heldHistory {
			if trustedSatellite.Compare(satelliteHeldHistory.SatelliteID) == 0 {
				found = true
				break
			}
		}
		if !found {
			heldHistory = append(heldHistory, HeldAmountHistory{
				SatelliteID: trustedSatellite,
			})
		}
	}

	return heldHistory, nil
}

// PaystubToSatellitePayoutForPeriod converts PayStub to SatellitePayoutForPeriod.
func PaystubToSatellitePayoutForPeriod(paystub PayStub, joinedAt time.Time, receipt string, satelliteURL string, isExitComplete bool) (payoutForPeriod SatellitePayoutForPeriod, err error) {
	if paystub.SurgePercent == 0 {
		paystub.SurgePercent = 100
	}

	earned, surge := paystub.GetEarnedWithSurge()

	heldPeriod, err := date.PeriodToTime(paystub.Period)
	if err != nil {
		return SatellitePayoutForPeriod{}, err
	}

	if joinedAt.IsZero() {
		joinedAt = time.Now()
		payoutForPeriod.HeldPercent = (float64(paystub.Held) / float64(earned)) * 100
	} else {
		payoutForPeriod.HeldPercent = GetHeldRate(joinedAt, heldPeriod)
	}

	payoutForPeriod.SatelliteID = paystub.SatelliteID.String()
	payoutForPeriod.SatelliteURL = satelliteURL
	payoutForPeriod.Age = int64(date.MonthsCountSince(joinedAt))
	payoutForPeriod.Earned = earned
	payoutForPeriod.Surge = surge
	payoutForPeriod.SurgePercent = paystub.SurgePercent
	payoutForPeriod.Held = paystub.Held
	payoutForPeriod.AfterHeld = surge - paystub.Held
	payoutForPeriod.Disposed = paystub.Disposed
	payoutForPeriod.Paid = paystub.Paid
	payoutForPeriod.Receipt = receipt
	payoutForPeriod.IsExitComplete = isExitComplete
	payoutForPeriod.Distributed = paystub.Distributed

	return payoutForPeriod, nil
}

// parsePeriodRange creates period range form start and end periods.
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

// GetHeldRate returns held rate for specific period from join date of node.
func GetHeldRate(joinTime time.Time, requestTime time.Time) (heldRate float64) {
	monthsSinceJoin := date.MonthsBetweenDates(joinTime, requestTime)
	switch monthsSinceJoin {
	case 0, 1, 2:
		heldRate = 75
	case 3, 4, 5:
		heldRate = 50
	case 6, 7, 8:
		heldRate = 25
	default:
		heldRate = 0
	}

	return heldRate
}
