// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
)

var (
	gb = decimal.NewFromInt(1e9)
	tb = decimal.NewFromInt(1e12)
)

var (
	// DefaultWithheldPercents contains the standard withholding schedule.
	DefaultWithheldPercents = []int{75, 75, 75, 50, 50, 50, 25, 25, 25}

	// DefaultRates contains the standard operation rates.
	DefaultRates = Rates{
		AtRestGBHours: RequireRateFromString("0.00000205"), // $1.50/TB at rest
		GetTB:         RequireRateFromString("20.00"),      // $20.00/TB
		PutTB:         RequireRateFromString("0.00"),
		GetRepairTB:   RequireRateFromString("10.00"), // $10.00/TB
		PutRepairTB:   RequireRateFromString("0.00"),
		GetAuditTB:    RequireRateFromString("10.0"), // $10.00/TB
	}
)

// NodeInfo contains all of the information about a node and the operations
// it performed in some period.
type NodeInfo struct {
	ID                 storj.NodeID
	CreatedAt          time.Time
	LastContactSuccess time.Time
	Disqualified       *time.Time
	GracefulExit       *time.Time
	UsageAtRest        float64
	UsageGet           int64
	UsagePut           int64
	UsageGetRepair     int64
	UsagePutRepair     int64
	UsageGetAudit      int64
	TotalHeld          currency.MicroUnit
	TotalDisposed      currency.MicroUnit
	TotalPaid          currency.MicroUnit
	TotalDistributed   currency.MicroUnit
}

// Statement is the computed amounts and codes from a node.
type Statement struct {
	NodeID       storj.NodeID
	Codes        Codes
	AtRest       currency.MicroUnit
	Get          currency.MicroUnit
	Put          currency.MicroUnit
	GetRepair    currency.MicroUnit
	PutRepair    currency.MicroUnit
	GetAudit     currency.MicroUnit
	SurgePercent int64
	Owed         currency.MicroUnit
	Held         currency.MicroUnit
	Disposed     currency.MicroUnit
}

// PeriodInfo contains configuration about the payment info to generate
// the statements.
type PeriodInfo struct {
	// Period is the period.
	Period Period

	// Nodes is usage and other related information for nodes for this period.
	Nodes []NodeInfo

	// Rates is the compensation rates for different operations. If nil, the
	// default rates are used.
	Rates *Rates

	// WithheldPercents is the percent to withhold from the total, after surge
	// adjustments, for each month in the node's lifetime. For example, to
	// withhold 75% in the first month, 50% in the second month, 0% in the third
	// month and to leave withheld thereafter, set to [75,50,0]. If nil,
	// DefaultWithheldPercents is used.
	WithheldPercents []int

	// DisposePercent is the percent to dispose to the node after it has left
	// withholding. The remaining amount is kept until the node performs a graceful
	// exit.
	DisposePercent int

	// SurgePercent is the percent to adjust final amounts owed. For example,
	// to pay 150%, set to 150. Zero means no surge.
	SurgePercent int64
}

// GenerateStatements generates all of the Statements for the given PeriodInfo.
func GenerateStatements(info PeriodInfo) ([]Statement, error) {
	startDate := info.Period.StartDate()
	endDate := info.Period.EndDateExclusive()

	rates := info.Rates
	if rates == nil {
		rates = &DefaultRates
	}
	withheldPercents := info.WithheldPercents
	if withheldPercents == nil {
		withheldPercents = DefaultWithheldPercents
	}

	surgePercent := decimal.NewFromInt(info.SurgePercent)
	disposePercent := decimal.NewFromInt(int64(info.DisposePercent))

	// Intermediate calculations (especially at-rest related) can overflow an
	// int64 so we need to use arbitrary precision fixed point math. The final
	// calculations should fit comfortably into an int64.  If not, it means
	// we're trying to pay somebody more than 9,223,372,036,854,775,807
	// micro-units (e.g.  $9,223,372,036,854 dollars).
	statements := make([]Statement, 0, len(info.Nodes))
	for _, node := range info.Nodes {
		var codes []Code

		atRest := decimal.NewFromFloat(node.UsageAtRest).
			Mul(decimal.Decimal(rates.AtRestGBHours)).
			Div(gb)
		get := decimal.NewFromInt(node.UsageGet).
			Mul(decimal.Decimal(rates.GetTB)).
			Div(tb)
		put := decimal.NewFromInt(node.UsagePut).
			Mul(decimal.Decimal(rates.PutTB)).
			Div(tb)
		getRepair := decimal.NewFromInt(node.UsageGetRepair).
			Mul(decimal.Decimal(rates.GetRepairTB)).
			Div(tb)
		putRepair := decimal.NewFromInt(node.UsagePutRepair).
			Mul(decimal.Decimal(rates.PutRepairTB)).
			Div(tb)
		getAudit := decimal.NewFromInt(node.UsageGetAudit).
			Mul(decimal.Decimal(rates.GetAuditTB)).
			Div(tb)

		total := decimal.Sum(atRest, get, put, getRepair, putRepair, getAudit)
		if info.SurgePercent > 0 {
			total = PercentOf(total, surgePercent)
		}

		gracefullyExited := node.GracefulExit != nil && node.GracefulExit.Before(endDate)
		if gracefullyExited {
			codes = append(codes, GracefulExit)
		}

		offline := node.LastContactSuccess.Before(startDate)
		if offline {
			codes = append(codes, Offline)
		}

		withheldPercent, inWithholding := NodeWithheldPercent(withheldPercents, node.CreatedAt, endDate)
		held := PercentOf(total, decimal.NewFromInt(int64(withheldPercent)))
		owed := total.Sub(held)
		if inWithholding {
			codes = append(codes, InWithholding)
		}

		var disposed decimal.Decimal
		if !inWithholding || gracefullyExited {
			// The storage node is out of withholding. Determine how much should be
			// disposed from withheld back to the storage node.
			disposed = node.TotalHeld.Decimal()
			if !gracefullyExited {
				disposed = PercentOf(disposed, disposePercent)
			} else { // if it's a graceful exit, don't withhold anything
				owed = owed.Add(held)
				held = decimal.Zero
			}
			disposed = disposed.Sub(node.TotalDisposed.Decimal())
			if disposed.Sign() < 0 {
				// We've disposed more than we should have according to the
				// percent. Don't dispose any more.
				disposed = decimal.Zero
			}
			owed = owed.Add(disposed)
		}

		// If the node is disqualified but not gracefully exited, nothing is owed/held/disposed.
		if node.Disqualified != nil && node.Disqualified.Before(endDate) && !gracefullyExited {
			codes = append(codes, Disqualified)
			disposed = decimal.Zero
			held = decimal.Zero
			owed = decimal.Zero
		}

		// If the node is offline, nothing is owed/held/disposed.
		if offline {
			disposed = decimal.Zero
			held = decimal.Zero
			owed = decimal.Zero
		}

		var overflowErrs errs.Group
		toMicroUnit := func(v decimal.Decimal) currency.MicroUnit {
			m, err := currency.MicroUnitFromDecimal(v)
			if err != nil {
				overflowErrs.Add(err)
				return currency.MicroUnit{}
			}
			return m
		}
		statement := Statement{
			NodeID:       node.ID,
			Codes:        codes,
			AtRest:       toMicroUnit(atRest),
			Get:          toMicroUnit(get),
			Put:          toMicroUnit(put),
			GetRepair:    toMicroUnit(getRepair),
			PutRepair:    toMicroUnit(putRepair),
			GetAudit:     toMicroUnit(getAudit),
			SurgePercent: info.SurgePercent,
			Owed:         toMicroUnit(owed),
			Held:         toMicroUnit(held),
			Disposed:     toMicroUnit(disposed),
		}

		if err := overflowErrs.Err(); err != nil {
			return nil, Error.New("currency overflows encountered while calculating payment for node %s", statement.NodeID.String())
		}

		statements = append(statements, statement)
	}

	return statements, nil
}
