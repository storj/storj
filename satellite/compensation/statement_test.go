// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"

	"storj.io/storj/satellite/compensation"
)

func TestGenerateStatements(t *testing.T) {
	rates := compensation.Rates{
		AtRestGBHours: compensation.RequireRateFromString("1"),
		GetTB:         compensation.RequireRateFromString("2"),
		PutTB:         compensation.RequireRateFromString("3"),
		GetRepairTB:   compensation.RequireRateFromString("4"),
		PutRepairTB:   compensation.RequireRateFromString("5"),
		GetAuditTB:    compensation.RequireRateFromString("6"),
	}

	// 50 percent withheld the first month
	escrowPercents := []int{50}

	// 60 percent disposed after leaving escrow and before graceful exit
	disposePercent := 60

	nodeID := testrand.NodeID()
	for _, tt := range []struct {
		name         string
		surgePercent int
		node         compensation.NodeInfo
		statement    compensation.Statement
	}{
		{
			name:         "within escrow",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:             nodeID,
				CreatedAt:      time.Date(2019, 11, 2, 0, 0, 0, 0, time.UTC),
				UsageAtRest:    1_000_000_000,     // 1GB/hours
				UsageGet:       2_000_000_000_000, // 2TB
				UsagePut:       3_000_000_000_000, // 3TB
				UsageGetRepair: 4_000_000_000_000, // 4TB
				UsagePutRepair: 5_000_000_000_000, // 5TB
				UsageGetAudit:  6_000_000_000_000, // 6TB
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				Codes:     compensation.Codes{compensation.InEscrow},
				AtRest:    1_000_000,
				Get:       4_000_000,
				Put:       9_000_000,
				GetRepair: 16_000_000,
				PutRepair: 25_000_000,
				GetAudit:  36_000_000,
				Owed:      45_500_000,
				Held:      45_500_000,
				Disposed:  0_000_000,
			},
		},
		{
			name:         "just out of escrow",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:             nodeID,
				CreatedAt:      time.Date(2019, 11, 1, 0, 0, 0, 0, time.UTC),
				UsageAtRest:    1_000_000_000,     // 1GB/hours
				UsageGet:       2_000_000_000_000, // 2TB
				UsagePut:       3_000_000_000_000, // 3TB
				UsageGetRepair: 4_000_000_000_000, // 4TB
				UsagePutRepair: 5_000_000_000_000, // 5TB
				UsageGetAudit:  6_000_000_000_000, // 6TB
				TotalHeld:      40_000_000,
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				AtRest:    1_000_000,
				Get:       4_000_000,
				Put:       9_000_000,
				GetRepair: 16_000_000,
				PutRepair: 25_000_000,
				GetAudit:  36_000_000,
				Owed:      115_000_000, // 91 for usage, 24 disposed from escrow
				Held:      0_000_000,
				Disposed:  24_000_000,
			},
		},
		{
			name:         "out of escrow and already disposed",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:             nodeID,
				CreatedAt:      time.Date(2019, 6, 12, 0, 0, 0, 0, time.UTC),
				UsageAtRest:    1_000_000_000,     // 1GB/hours
				UsageGet:       2_000_000_000_000, // 2TB
				UsagePut:       3_000_000_000_000, // 3TB
				UsageGetRepair: 4_000_000_000_000, // 4TB
				UsagePutRepair: 5_000_000_000_000, // 5TB
				UsageGetAudit:  6_000_000_000_000, // 6TB
				TotalHeld:      40_000_000,
				TotalDisposed:  24_000_000,
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				AtRest:    1_000_000,
				Get:       4_000_000,
				Put:       9_000_000,
				GetRepair: 16_000_000,
				PutRepair: 25_000_000,
				GetAudit:  36_000_000,
				Owed:      91_000_000,
				Held:      0_000_000,
				Disposed:  0_000_000,
			},
		},
		{
			name:         "graceful exit within period",
			surgePercent: 0,
			node: compensation.NodeInfo{
				ID:             nodeID,
				CreatedAt:      time.Date(2018, 6, 12, 0, 0, 0, 0, time.UTC),
				GracefulExit:   timePtr(time.Date(2019, 11, 30, 23, 59, 59, 0, time.UTC)),
				UsageAtRest:    1_000_000_000,     // 1GB/hours
				UsageGet:       2_000_000_000_000, // 2TB
				UsagePut:       3_000_000_000_000, // 3TB
				UsageGetRepair: 4_000_000_000_000, // 4TB
				UsagePutRepair: 5_000_000_000_000, // 5TB
				UsageGetAudit:  6_000_000_000_000, // 6TB
				TotalHeld:      40_000_000,
				TotalDisposed:  24_000_000,
			},
			statement: compensation.Statement{
				NodeID:    nodeID,
				Codes:     compensation.Codes{compensation.GracefulExit},
				AtRest:    1_000_000,
				Get:       4_000_000,
				Put:       9_000_000,
				GetRepair: 16_000_000,
				PutRepair: 25_000_000,
				GetAudit:  36_000_000,
				Owed:      91_000_000 + 16_000_000,
				Held:      0_000_000,
				Disposed:  16_000_000,
			},
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			statements, err := compensation.GenerateStatements(compensation.PeriodInfo{
				Period:         compensation.Period{Year: 2019, Month: 11},
				Nodes:          []compensation.NodeInfo{tt.node},
				SurgePercent:   tt.surgePercent,
				Rates:          &rates,
				EscrowPercents: escrowPercents,
				DisposePercent: disposePercent,
			})
			require.NoError(t, err)
			assert.Equal(t, []compensation.Statement{tt.statement}, statements)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
