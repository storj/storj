// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/compensation"
)

// D returns a MicroUnit representing the amount in dollars. It is in general not
// useful because it accepts a float, but makes it easier to express natual units
// in tests.
func D(v float64) currency.MicroUnit { return currency.NewMicroUnit(int64(v * 1e6)) }

func TestGenerateStatements(t *testing.T) {
	const (
		GB = 1_000_000_000
		TB = 1_000_000_000_000
	)

	rates := compensation.Rates{
		AtRestGBHours: compensation.RequireRateFromString("2"),
		GetTB:         compensation.RequireRateFromString("3"),
		PutTB:         compensation.RequireRateFromString("5"),
		GetRepairTB:   compensation.RequireRateFromString("7"),
		PutRepairTB:   compensation.RequireRateFromString("11"),
		GetAuditTB:    compensation.RequireRateFromString("13"),
	}

	// 50 percent withheld the first month
	withheldPercents := []int{50}

	// 60 percent disposed after leaving withheld and before graceful exit
	disposePercent := 60

	nodeID := testrand.NodeID()
	for _, tt := range []struct {
		name         string
		surgePercent int64
		node         compensation.NodeInfo
		statement    compensation.Statement
	}{
		{
			name:         "within withholding",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:                 nodeID,
				LastContactSuccess: time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				UsageAtRest:        1 * GB,
				UsageGet:           2 * TB,
				UsagePut:           3 * TB,
				UsageGetRepair:     4 * TB,
				UsagePutRepair:     5 * TB,
				UsageGetAudit:      6 * TB,
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				Codes:     compensation.Codes{compensation.InWithholding},
				AtRest:    D(2),
				Get:       D(6),
				Put:       D(15),
				GetRepair: D(28),
				PutRepair: D(55),
				GetAudit:  D(78),
				Owed:      D(92),
				Held:      D(92),
				Disposed:  D(0),
			},
		},
		{
			name:         "just out of withheld",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:                 nodeID,
				LastContactSuccess: time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2019, 11, 1, 0, 0, 0, 0, time.UTC),
				UsageAtRest:        1 * GB,
				UsageGet:           2 * TB,
				UsagePut:           3 * TB,
				UsageGetRepair:     4 * TB,
				UsagePutRepair:     5 * TB,
				UsageGetAudit:      6 * TB,
				TotalHeld:          D(40),
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				AtRest:    D(2),
				Get:       D(6),
				Put:       D(15),
				GetRepair: D(28),
				PutRepair: D(55),
				GetAudit:  D(78),
				Owed:      D(184 + 24), // 184 for usage, 24 disposed from withheld
				Held:      D(0),
				Disposed:  D(24),
			},
		},
		{
			name:         "out of withheld and already disposed",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:                 nodeID,
				LastContactSuccess: time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2019, 6, 12, 0, 0, 0, 0, time.UTC),
				UsageAtRest:        1 * GB,
				UsageGet:           2 * TB,
				UsagePut:           3 * TB,
				UsageGetRepair:     4 * TB,
				UsagePutRepair:     5 * TB,
				UsageGetAudit:      6 * TB,
				TotalHeld:          D(40),
				TotalDisposed:      D(24),
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				AtRest:    D(2),
				Get:       D(6),
				Put:       D(15),
				GetRepair: D(28),
				PutRepair: D(55),
				GetAudit:  D(78),
				Owed:      D(184),
				Held:      D(0),
				Disposed:  D(0),
			},
		},
		{
			name:         "graceful exit within period",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:                 nodeID,
				LastContactSuccess: time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2018, 6, 12, 0, 0, 0, 0, time.UTC),
				GracefulExit:       timePtr(time.Date(2019, 11, 30, 23, 59, 59, 0, time.UTC)),
				UsageAtRest:        1 * GB,
				UsageGet:           2 * TB,
				UsagePut:           3 * TB,
				UsageGetRepair:     4 * TB,
				UsagePutRepair:     5 * TB,
				UsageGetAudit:      6 * TB,
				TotalHeld:          D(40),
				TotalDisposed:      D(24),
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				Codes:     compensation.Codes{compensation.GracefulExit},
				AtRest:    D(2),
				Get:       D(6),
				Put:       D(15),
				GetRepair: D(28),
				PutRepair: D(55),
				GetAudit:  D(78),
				Owed:      D(184 + 16),
				Held:      D(0),
				Disposed:  D(16),
			},
		},
		{
			name:         "offline",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:                 nodeID,
				LastContactSuccess: time.Date(2019, 10, 2, 0, 0, 0, 0, time.UTC),
				CreatedAt:          time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				UsageAtRest:        1 * GB,
				UsageGet:           2 * TB,
				UsagePut:           3 * TB,
				UsageGetRepair:     4 * TB,
				UsagePutRepair:     5 * TB,
				UsageGetAudit:      6 * TB,
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				Codes:     compensation.Codes{compensation.Offline, compensation.InWithholding},
				AtRest:    D(2),
				Get:       D(6),
				Put:       D(15),
				GetRepair: D(28),
				PutRepair: D(55),
				GetAudit:  D(78),
				Owed:      D(0),
				Held:      D(0),
				Disposed:  D(0),
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			statements, err := compensation.GenerateStatements(compensation.PeriodInfo{
				Period:           compensation.Period{Year: 2019, Month: 11},
				Nodes:            []compensation.NodeInfo{tt.node},
				SurgePercent:     tt.surgePercent,
				Rates:            &rates,
				WithheldPercents: withheldPercents,
				DisposePercent:   disposePercent,
			})
			require.NoError(t, err)
			assert.Equal(t, []compensation.Statement{tt.statement}, statements)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
