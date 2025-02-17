// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package snopayouts_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/satellite/snopayouts"
)

func TestPayoutDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		snoPayoutDB := db.SNOPayouts()
		NodeID := storj.NodeID{}

		paystub := snopayouts.Paystub{
			Period:         "2020-01",
			NodeID:         NodeID,
			Codes:          "1",
			UsageAtRest:    1,
			UsageGet:       2,
			UsagePut:       3,
			UsageGetRepair: 4,
			UsagePutRepair: 5,
			UsageGetAudit:  6,
			CompAtRest:     7,
			CompGet:        8,
			CompPut:        9,
			CompGetRepair:  10,
			CompPutRepair:  11,
			CompGetAudit:   12,
			SurgePercent:   13,
			Held:           14,
			Owed:           15,
			Disposed:       16,
			Paid:           17,
			Distributed:    18,
		}

		paystub2 := snopayouts.Paystub{
			Period:         "2020-02",
			NodeID:         NodeID,
			Codes:          "2",
			UsageAtRest:    4,
			UsageGet:       5,
			UsagePut:       6,
			UsageGetRepair: 7,
			UsagePutRepair: 8,
			UsageGetAudit:  9,
			CompAtRest:     10,
			CompGet:        11,
			CompPut:        12,
			CompGetRepair:  13,
			CompPutRepair:  14,
			CompGetAudit:   15,
			SurgePercent:   16,
			Held:           17,
			Owed:           18,
			Disposed:       19,
			Paid:           20,
			Distributed:    21,
		}

		paystub3 := snopayouts.Paystub{
			Period:         "2020-03",
			NodeID:         NodeID,
			Codes:          "33",
			UsageAtRest:    10,
			UsageGet:       11,
			UsagePut:       12,
			UsageGetRepair: 13,
			UsagePutRepair: 14,
			UsageGetAudit:  15,
			CompAtRest:     16,
			CompGet:        17,
			CompPut:        18,
			CompGetRepair:  19,
			CompPutRepair:  20,
			CompGetAudit:   21,
			SurgePercent:   22,
			Held:           23,
			Owed:           24,
			Disposed:       25,
			Paid:           26,
			Distributed:    27,
		}

		{
			err := snoPayoutDB.TestCreatePaystub(ctx, paystub)
			require.NoError(t, err)

			err = snoPayoutDB.TestCreatePaystub(ctx, paystub2)
			require.NoError(t, err)

			err = snoPayoutDB.TestCreatePaystub(ctx, paystub3)
			require.NoError(t, err)
		}

		{
			actual, err := snoPayoutDB.GetPaystub(ctx, NodeID, "2020-01")
			require.NoError(t, err)
			actual.Created = time.Time{} // created is chosen by the database layer
			require.Equal(t, paystub, actual)

			_, err = snoPayoutDB.GetPaystub(ctx, NodeID, "")
			require.Error(t, err)

			_, err = snoPayoutDB.GetPaystub(ctx, testrand.NodeID(), "2020-01")
			require.Error(t, err)
		}

		{
			stubs, err := snoPayoutDB.GetAllPaystubs(ctx, NodeID)
			require.NoError(t, err)
			for _, actual := range stubs {
				actual.Created = time.Time{} // created is chosen by the database layer
				require.Equal(t, actual, map[string]snopayouts.Paystub{
					"2020-01": paystub,
					"2020-02": paystub2,
					"2020-03": paystub3,
				}[actual.Period])
			}
		}

		payment := snopayouts.Payment{
			NodeID:  NodeID,
			Period:  "2020-01",
			Amount:  123,
			Receipt: "receipt",
			Notes:   "notes",
		}

		{
			err := snoPayoutDB.TestCreatePayment(ctx, payment)
			require.NoError(t, err)
		}

		{
			actual, err := snoPayoutDB.GetPayment(ctx, NodeID, "2020-01")
			require.NoError(t, err)
			actual.Created = time.Time{} // created is chosen by the database layer
			actual.ID = 0                // id is chosen by the database layer
			require.Equal(t, payment, actual)

			_, err = snoPayoutDB.GetPayment(ctx, NodeID, "")
			require.Error(t, err)

			_, err = snoPayoutDB.GetPayment(ctx, testrand.NodeID(), "2020-01")
			require.Error(t, err)
		}
	})
}
