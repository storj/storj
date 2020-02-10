// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/private/currency"
)

var (
	gb = decimal.NewFromInt(1e9)
	tb = decimal.NewFromInt(1e12)
)

var (
	DefaultEscrowRates = []int{75, 75, 75, 50, 50, 50, 25, 25, 25}
	DefaultRates       = Rates{
		AtRestGBHours: RequireRateFromString("0.00000205"), // $1.50/TB at rest
		GetTB:         RequireRateFromString("20.00"),      // $20.00/TB
		PutTB:         RequireRateFromString("0.00"),
		GetRepairTB:   RequireRateFromString("10.00"), // $10.00/TB
		PutRepairTB:   RequireRateFromString("0.00"),
		GetAuditTB:    RequireRateFromString("10.0"), // $10.00/TB
	}
)

type NodeInfo struct {
	ID             storj.NodeID
	CreatedAt      time.Time
	Disqualified   *time.Time
	GracefulExit   *time.Time
	UsageAtRest    float64
	UsageGet       int64
	UsagePut       int64
	UsageGetRepair int64
	UsagePutRepair int64
	UsageGetAudit  int64
	TotalHeld      currency.MicroUnit
	TotalDisposed  currency.MicroUnit
}

type Statement struct {
	NodeID       storj.NodeID
	Codes        Codes
	AtRest       currency.MicroUnit
	Get          currency.MicroUnit
	Put          currency.MicroUnit
	GetRepair    currency.MicroUnit
	PutRepair    currency.MicroUnit
	GetAudit     currency.MicroUnit
	SurgePercent int
	Owed         currency.MicroUnit
	Held         currency.MicroUnit
	Disposed     currency.MicroUnit
}

type PeriodInfo struct {
	// Period is the period.
	Period Period

	// Nodes is usage and other related information for nodes for this period.
	Nodes []NodeInfo

	// Rates is the compensation rates for different operations. If nil, the
	// default rates are used.
	Rates *Rates

	// EscrowPercents is the percent to withold from the total, after surge
	// adjustments, for each month in the node's lifetime. For example, to
	// withold 75% in the first month, 50% in the second month, 0% in the third
	// month and to leave escrow thereafter, set to [75,50,0]. If nil,
	// DefaultEscrowPercents is used.
	EscrowPercents []int

	// DisposePercent is the percent to dispose to the node after it has left
	// escrow. The remaining amount is kept until the node performs a graceful
	// exit.
	DisposePercent int

	// SurgePercent is the percent to adjust final amounts owed. For example,
	// to pay 150%, set to 150. Zero means no surge.
	SurgePercent int
}

func GenerateStatements(info PeriodInfo) ([]Statement, error) {
	endDate := info.Period.EndDateExclusive()

	rates := info.Rates
	if rates == nil {
		rates = &DefaultRates
	}
	escrowPercents := info.EscrowPercents
	if escrowPercents == nil {
		escrowPercents = DefaultEscrowRates
	}

	surgePercent := decimal.NewFromInt(int64(info.SurgePercent))
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

		escrowPercent, inEscrow := NodeEscrowPercent(escrowPercents, node.CreatedAt, endDate)
		held := PercentOf(total, decimal.NewFromInt(int64(escrowPercent)))
		owed := total.Sub(held)

		var disposed decimal.Decimal
		switch {
		case inEscrow:
			codes = append(codes, InEscrow)
		default:
			// The storage node is out of escrow. Determine how much should be
			// disposed from escrow back to the storage node.
			disposed = node.TotalHeld.Decimal()
			if gracefullyExited {
				codes = append(codes, GracefulExit)
			} else {
				disposed = PercentOf(disposed, disposePercent)
			}
			disposed = disposed.Sub(node.TotalDisposed.Decimal())
			if disposed.Sign() < 0 {
				// We've disposed more than we should have according to the
				// percent. Don't dispose any more.
				disposed = decimal.Zero
			}
			owed = owed.Add(disposed)
		}

		// If the node is disqualified nothing is owed/held/disposed.
		if node.Disqualified != nil && node.Disqualified.Before(endDate) {
			codes = append(codes, Disqualified)
			disposed = decimal.Zero
			held = decimal.Zero
			owed = decimal.Zero
		}

		var overflowErrs errs.Group
		toMicroUnit := func(v decimal.Decimal) currency.MicroUnit {
			m, err := currency.MicroUnitFromDecimal(v)
			if err != nil {
				overflowErrs.Add(err)
				return 0
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
