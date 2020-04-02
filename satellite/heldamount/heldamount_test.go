// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/heldamount"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestHeldAmountDB(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		heldAmount := db.HeldAmount()
		NodeID := storj.NodeID{}
		period := "2020-01"
		paystub := heldamount.PayStub{
			Period:         "2020-01",
			NodeID:         NodeID,
			Created:        time.Now().UTC(),
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
		}

		paystub2 := heldamount.PayStub{
			Period:         "2020-02",
			NodeID:         NodeID,
			Created:        time.Now().UTC(),
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
		}

		paystub3 := heldamount.PayStub{
			Period:         "2020-03",
			NodeID:         NodeID,
			Created:        time.Now().UTC(),
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
		}

		t.Run("Test StorePayStub", func(t *testing.T) {
			err := heldAmount.CreatePaystub(ctx, paystub)
			assert.NoError(t, err)
			err = heldAmount.CreatePaystub(ctx, paystub2)
			assert.NoError(t, err)
			err = heldAmount.CreatePaystub(ctx, paystub3)
			assert.NoError(t, err)
		})

		t.Run("Test GetPayStub", func(t *testing.T) {
			stub, err := heldAmount.GetPaystub(ctx, NodeID, period)
			assert.NoError(t, err)
			assert.Equal(t, stub.Period, paystub.Period)
			assert.Equal(t, stub.Codes, paystub.Codes)
			assert.Equal(t, stub.CompAtRest, paystub.CompAtRest)
			assert.Equal(t, stub.CompGet, paystub.CompGet)
			assert.Equal(t, stub.CompGetAudit, paystub.CompGetAudit)
			assert.Equal(t, stub.CompGetRepair, paystub.CompGetRepair)
			assert.Equal(t, stub.CompPut, paystub.CompPut)
			assert.Equal(t, stub.CompPutRepair, paystub.CompPutRepair)
			assert.Equal(t, stub.Disposed, paystub.Disposed)
			assert.Equal(t, stub.Held, paystub.Held)
			assert.Equal(t, stub.Owed, paystub.Owed)
			assert.Equal(t, stub.Paid, paystub.Paid)
			assert.Equal(t, stub.NodeID, paystub.NodeID)
			assert.Equal(t, stub.SurgePercent, paystub.SurgePercent)
			assert.Equal(t, stub.UsageAtRest, paystub.UsageAtRest)
			assert.Equal(t, stub.UsageGet, paystub.UsageGet)
			assert.Equal(t, stub.UsageGetAudit, paystub.UsageGetAudit)
			assert.Equal(t, stub.UsageGetRepair, paystub.UsageGetRepair)
			assert.Equal(t, stub.UsagePut, paystub.UsagePut)
			assert.Equal(t, stub.UsagePutRepair, paystub.UsagePutRepair)

			stub, err = heldAmount.GetPaystub(ctx, NodeID, "")
			assert.Error(t, err)

			stub, err = heldAmount.GetPaystub(ctx, storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, period)
			assert.Error(t, err)
		})

		t.Run("Test GetAllPaystubs", func(t *testing.T) {
			stubs, err := heldAmount.GetAllPaystubs(ctx, NodeID)
			assert.NoError(t, err)
			for i := 0; i < len(stubs); i++ {
				if stubs[i].Period == "2020-01" {
					assert.Equal(t, stubs[i].Period, paystub.Period)
					assert.Equal(t, stubs[i].Codes, paystub.Codes)
					assert.Equal(t, stubs[i].CompAtRest, paystub.CompAtRest)
					assert.Equal(t, stubs[i].CompGet, paystub.CompGet)
					assert.Equal(t, stubs[i].CompGetAudit, paystub.CompGetAudit)
					assert.Equal(t, stubs[i].CompGetRepair, paystub.CompGetRepair)
					assert.Equal(t, stubs[i].CompPut, paystub.CompPut)
					assert.Equal(t, stubs[i].CompPutRepair, paystub.CompPutRepair)
					assert.Equal(t, stubs[i].Disposed, paystub.Disposed)
					assert.Equal(t, stubs[i].Held, paystub.Held)
					assert.Equal(t, stubs[i].Owed, paystub.Owed)
					assert.Equal(t, stubs[i].Paid, paystub.Paid)
					assert.Equal(t, stubs[i].NodeID, paystub.NodeID)
					assert.Equal(t, stubs[i].SurgePercent, paystub.SurgePercent)
					assert.Equal(t, stubs[i].UsageAtRest, paystub.UsageAtRest)
					assert.Equal(t, stubs[i].UsageGet, paystub.UsageGet)
					assert.Equal(t, stubs[i].UsageGetAudit, paystub.UsageGetAudit)
					assert.Equal(t, stubs[i].UsageGetRepair, paystub.UsageGetRepair)
					assert.Equal(t, stubs[i].UsagePut, paystub.UsagePut)
					assert.Equal(t, stubs[i].UsagePutRepair, paystub.UsagePutRepair)
				}
				if stubs[i].Period == "2020-02" {
					assert.Equal(t, stubs[i].Period, paystub2.Period)
					assert.Equal(t, stubs[i].Codes, paystub2.Codes)
					assert.Equal(t, stubs[i].CompAtRest, paystub2.CompAtRest)
					assert.Equal(t, stubs[i].CompGet, paystub2.CompGet)
					assert.Equal(t, stubs[i].CompGetAudit, paystub2.CompGetAudit)
					assert.Equal(t, stubs[i].CompGetRepair, paystub2.CompGetRepair)
					assert.Equal(t, stubs[i].CompPut, paystub2.CompPut)
					assert.Equal(t, stubs[i].CompPutRepair, paystub2.CompPutRepair)
					assert.Equal(t, stubs[i].Disposed, paystub2.Disposed)
					assert.Equal(t, stubs[i].Held, paystub2.Held)
					assert.Equal(t, stubs[i].Owed, paystub2.Owed)
					assert.Equal(t, stubs[i].Paid, paystub2.Paid)
					assert.Equal(t, stubs[i].NodeID, paystub2.NodeID)
					assert.Equal(t, stubs[i].SurgePercent, paystub2.SurgePercent)
					assert.Equal(t, stubs[i].UsageAtRest, paystub2.UsageAtRest)
					assert.Equal(t, stubs[i].UsageGet, paystub2.UsageGet)
					assert.Equal(t, stubs[i].UsageGetAudit, paystub2.UsageGetAudit)
					assert.Equal(t, stubs[i].UsageGetRepair, paystub2.UsageGetRepair)
					assert.Equal(t, stubs[i].UsagePut, paystub2.UsagePut)
					assert.Equal(t, stubs[i].UsagePutRepair, paystub2.UsagePutRepair)
				}
				if stubs[i].Period == "2020-03" {
					assert.Equal(t, stubs[i].Period, paystub3.Period)
					assert.Equal(t, stubs[i].Codes, paystub3.Codes)
					assert.Equal(t, stubs[i].CompAtRest, paystub3.CompAtRest)
					assert.Equal(t, stubs[i].CompGet, paystub3.CompGet)
					assert.Equal(t, stubs[i].CompGetAudit, paystub3.CompGetAudit)
					assert.Equal(t, stubs[i].CompGetRepair, paystub3.CompGetRepair)
					assert.Equal(t, stubs[i].CompPut, paystub3.CompPut)
					assert.Equal(t, stubs[i].CompPutRepair, paystub3.CompPutRepair)
					assert.Equal(t, stubs[i].Disposed, paystub3.Disposed)
					assert.Equal(t, stubs[i].Held, paystub3.Held)
					assert.Equal(t, stubs[i].Owed, paystub3.Owed)
					assert.Equal(t, stubs[i].Paid, paystub3.Paid)
					assert.Equal(t, stubs[i].NodeID, paystub3.NodeID)
					assert.Equal(t, stubs[i].SurgePercent, paystub3.SurgePercent)
					assert.Equal(t, stubs[i].UsageAtRest, paystub3.UsageAtRest)
					assert.Equal(t, stubs[i].UsageGet, paystub3.UsageGet)
					assert.Equal(t, stubs[i].UsageGetAudit, paystub3.UsageGetAudit)
					assert.Equal(t, stubs[i].UsageGetRepair, paystub3.UsageGetRepair)
					assert.Equal(t, stubs[i].UsagePut, paystub3.UsagePut)
					assert.Equal(t, stubs[i].UsagePutRepair, paystub3.UsagePutRepair)
				}
			}
		})
	})
}
