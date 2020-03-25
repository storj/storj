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

		t.Run("Test StorePayStub", func(t *testing.T) {
			err := heldAmount.CreatePaystub(ctx, paystub)
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

		payment := heldamount.StoragenodePayment{
			ID:      1,
			Created: time.Now().UTC(),
			NodeID:  NodeID,
			Period:  "2020-01",
			Amount:  228,
			Receipt: "receipt",
			Notes:   "notes",
		}

		t.Run("Test StorePayment", func(t *testing.T) {
			err := heldAmount.CreatePayment(ctx, payment)
			assert.NoError(t, err)
		})

		t.Run("Test GetPayment", func(t *testing.T) {
			paym, err := heldAmount.GetPayment(ctx, NodeID, period)
			assert.NoError(t, err)
			assert.Equal(t, paym.NodeID, payment.NodeID)
			assert.Equal(t, paym.Period, payment.Period)
			assert.Equal(t, paym.Amount, payment.Amount)
			assert.Equal(t, paym.Notes, payment.Notes)
			assert.Equal(t, paym.Receipt, payment.Receipt)

			paym, err = heldAmount.GetPayment(ctx, NodeID, "")
			assert.Error(t, err)

			paym, err = heldAmount.GetPayment(ctx, storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, period)
			assert.Error(t, err)
		})
	})
}
