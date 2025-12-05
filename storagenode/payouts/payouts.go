// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// DB works with payouts database.
//
// architecture: Database
type DB interface {
	// StorePayStub inserts or updates paystub into the DB.
	StorePayStub(ctx context.Context, paystub PayStub) error
	// GetPayStub retrieves paystub for specific satellite and period.
	GetPayStub(ctx context.Context, satelliteID storj.NodeID, period string) (*PayStub, error)
	// AllPayStubs retrieves paystubs from all satellites in specific period from DB.
	AllPayStubs(ctx context.Context, period string) ([]PayStub, error)
	// SatellitesHeldbackHistory retrieves heldback history for specific satellite from DB.
	SatellitesHeldbackHistory(ctx context.Context, satelliteID storj.NodeID) ([]HeldForPeriod, error)
	// SatellitesDisposedHistory returns all disposed amount for specific satellite from DB.
	SatellitesDisposedHistory(ctx context.Context, satelliteID storj.NodeID) (int64, error)
	// SatellitePeriods retrieves all periods for concrete satellite in which we have some payouts data.
	SatellitePeriods(ctx context.Context, satelliteID storj.NodeID) ([]string, error)
	// AllPeriods retrieves all periods in which we have some payouts data.
	AllPeriods(ctx context.Context) ([]string, error)
	// StorePayment inserts or updates payment into the DB
	StorePayment(ctx context.Context, payment Payment) error
	// GetReceipt retrieves receipt for specific satellite and period.
	GetReceipt(ctx context.Context, satelliteID storj.NodeID, period string) (string, error)
	// GetTotalEarned returns total earned amount of node from all paystubs.
	GetTotalEarned(ctx context.Context) (_ int64, err error)
	// GetEarnedAtSatellite returns total earned value for node from specific satellite.
	GetEarnedAtSatellite(ctx context.Context, id storj.NodeID) (int64, error)
	// GetPayingSatellitesIDs returns list of satellite ID's that ever paid to storagenode.
	GetPayingSatellitesIDs(ctx context.Context) ([]storj.NodeID, error)
	// GetSatelliteSummary returns satellite all time paid and held amounts.
	GetSatelliteSummary(ctx context.Context, satelliteID storj.NodeID) (paid, held int64, err error)
	// GetSatellitePeriodSummary returns satellite paid and held amounts for specific period.
	GetSatellitePeriodSummary(ctx context.Context, satelliteID storj.NodeID, period string) (paid, held int64, err error)
	// GetUndistributed returns total undistributed amount.
	GetUndistributed(ctx context.Context) (int64, error)
	// GetSatellitePaystubs returns summed paystubs for specific satellite.
	GetSatellitePaystubs(ctx context.Context, satelliteID storj.NodeID) (*PayStub, error)
	// GetPaystubs returns summed all paystubs.
	GetPaystubs(ctx context.Context) (*PayStub, error)
	// GetSatellitesPeriodPaystubs returns summed all satellites paystubs for specific period.
	GetPeriodPaystubs(ctx context.Context, period string) (*PayStub, error)
	// GetSatellitePeriodPaystubs returns summed satellite paystubs for specific period.
	GetSatellitePeriodPaystubs(ctx context.Context, period string, satelliteID storj.NodeID) (*PayStub, error)
	// HeldAmountHistory retrieves held amount history for all satellites.
	HeldAmountHistory(ctx context.Context) ([]HeldAmountHistory, error)
}

// ErrNoPayStubForPeriod represents errors from the payouts database.
var ErrNoPayStubForPeriod = errs.Class("no payStub for period")

// PayStub is node payouts data for satellite by specific period.
type PayStub struct {
	SatelliteID    storj.NodeID `json:"satelliteId"`
	Period         string       `json:"period"`
	Created        time.Time    `json:"created"`
	Codes          string       `json:"codes"`
	UsageAtRest    float64      `json:"usageAtRest"`
	UsageGet       int64        `json:"usageGet"`
	UsagePut       int64        `json:"usagePut"`
	UsageGetRepair int64        `json:"usageGetRepair"`
	UsagePutRepair int64        `json:"usagePutRepair"`
	UsageGetAudit  int64        `json:"usageGetAudit"`
	CompAtRest     int64        `json:"compAtRest"`
	CompGet        int64        `json:"compGet"`
	CompPut        int64        `json:"compPut"`
	CompGetRepair  int64        `json:"compGetRepair"`
	CompPutRepair  int64        `json:"compPutRepair"`
	CompGetAudit   int64        `json:"compGetAudit"`
	SurgePercent   int64        `json:"surgePercent"`
	Held           int64        `json:"held"`
	Owed           int64        `json:"owed"`
	Disposed       int64        `json:"disposed"`
	Paid           int64        `json:"paid"`
	Distributed    int64        `json:"distributed"`
}

// GetEarnedWithSurge returns paystub's total earned and surge.
func (paystub *PayStub) GetEarnedWithSurge() (earned int64, surge int64) {
	earned = paystub.CompGetAudit + paystub.CompGet + paystub.CompGetRepair + paystub.CompAtRest
	surge = earned * paystub.SurgePercent / 100

	return earned, surge
}

// UsageAtRestTbM converts paystub's usage_at_rest from tbh to tbm.
func (paystub *PayStub) UsageAtRestTbM() {
	paystub.UsageAtRest /= 720
}

// Payment is node payment data for specific period.
type Payment struct {
	ID          int64        `json:"id"`
	Created     time.Time    `json:"created"`
	SatelliteID storj.NodeID `json:"satelliteId"`
	Period      string       `json:"period"`
	Amount      int64        `json:"amount"`
	Receipt     string       `json:"receipt"`
	Notes       string       `json:"notes"`
}

// SatelliteHeldHistory amount of held for specific satellite for all time since join.
type SatelliteHeldHistory struct {
	SatelliteID         storj.NodeID `json:"satelliteID"`
	SatelliteName       string       `json:"satelliteName"`
	HoldForFirstPeriod  int64        `json:"holdForFirstPeriod"`
	HoldForSecondPeriod int64        `json:"holdForSecondPeriod"`
	HoldForThirdPeriod  int64        `json:"holdForThirdPeriod"`
	TotalHeld           int64        `json:"totalHeld"`
	TotalDisposed       int64        `json:"totalDisposed"`
	JoinedAt            time.Time    `json:"joinedAt"`
}

// SatellitePayoutForPeriod contains payouts information for specific period for specific satellite.
type SatellitePayoutForPeriod struct {
	SatelliteID    string  `json:"satelliteID"`
	SatelliteURL   string  `json:"satelliteURL"`
	Age            int64   `json:"age"`
	Earned         int64   `json:"earned"`
	Surge          int64   `json:"surge"`
	SurgePercent   int64   `json:"surgePercent"`
	Held           int64   `json:"held"`
	HeldPercent    float64 `json:"heldPercent"`
	AfterHeld      int64   `json:"afterHeld"`
	Disposed       int64   `json:"disposed"`
	Paid           int64   `json:"paid"`
	Receipt        string  `json:"receipt"`
	IsExitComplete bool    `json:"isExitComplete"`
	Distributed    int64   `json:"distributed"`
}

// HeldAmountHistory contains held amount history for satellite.
type HeldAmountHistory struct {
	SatelliteID storj.NodeID    `json:"satelliteId"`
	HeldAmounts []HeldForPeriod `json:"heldAmounts"`
}

// HeldForPeriod is node's held amount for period.
type HeldForPeriod struct {
	Period string `json:"period"`
	Amount int64  `json:"amount"`
}
