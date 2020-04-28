// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package heldamount_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/heldamount"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestHeldAmountDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmount := db.HeldAmount()
		satelliteID := storj.NodeID{}
		period := "2020-01"
		paystub := heldamount.PayStub{
			SatelliteID:    satelliteID,
			Period:         "2020-01",
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
			err := heldAmount.StorePayStub(ctx, paystub)
			assert.NoError(t, err)
		})

		t.Run("Test GetPayStub", func(t *testing.T) {
			stub, err := heldAmount.GetPayStub(ctx, satelliteID, period)
			assert.NoError(t, err)
			assert.Equal(t, stub.Period, paystub.Period)
			assert.Equal(t, stub.Created, paystub.Created)
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
			assert.Equal(t, stub.SatelliteID, paystub.SatelliteID)
			assert.Equal(t, stub.SurgePercent, paystub.SurgePercent)
			assert.Equal(t, stub.UsageAtRest, paystub.UsageAtRest)
			assert.Equal(t, stub.UsageGet, paystub.UsageGet)
			assert.Equal(t, stub.UsageGetAudit, paystub.UsageGetAudit)
			assert.Equal(t, stub.UsageGetRepair, paystub.UsageGetRepair)
			assert.Equal(t, stub.UsagePut, paystub.UsagePut)
			assert.Equal(t, stub.UsagePutRepair, paystub.UsagePutRepair)

			stub, err = heldAmount.GetPayStub(ctx, satelliteID, "")
			assert.Error(t, err)
			assert.Equal(t, true, heldamount.ErrNoPayStubForPeriod.Has(err))
			assert.Nil(t, stub)

			stub, err = heldAmount.GetPayStub(ctx, storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, period)
			assert.Error(t, err)
			assert.Equal(t, true, heldamount.ErrNoPayStubForPeriod.Has(err))
			assert.Nil(t, stub)
		})

		t.Run("Test AllPayStubs", func(t *testing.T) {
			stubs, err := heldAmount.AllPayStubs(ctx, period)
			assert.NoError(t, err)
			assert.NotNil(t, stubs)
			assert.Equal(t, 1, len(stubs))
			assert.Equal(t, stubs[0].Period, paystub.Period)
			assert.Equal(t, stubs[0].Created, paystub.Created)
			assert.Equal(t, stubs[0].Codes, paystub.Codes)
			assert.Equal(t, stubs[0].CompAtRest, paystub.CompAtRest)
			assert.Equal(t, stubs[0].CompGet, paystub.CompGet)
			assert.Equal(t, stubs[0].CompGetAudit, paystub.CompGetAudit)
			assert.Equal(t, stubs[0].CompGetRepair, paystub.CompGetRepair)
			assert.Equal(t, stubs[0].CompPut, paystub.CompPut)
			assert.Equal(t, stubs[0].CompPutRepair, paystub.CompPutRepair)
			assert.Equal(t, stubs[0].Disposed, paystub.Disposed)
			assert.Equal(t, stubs[0].Held, paystub.Held)
			assert.Equal(t, stubs[0].Owed, paystub.Owed)
			assert.Equal(t, stubs[0].Paid, paystub.Paid)
			assert.Equal(t, stubs[0].SatelliteID, paystub.SatelliteID)
			assert.Equal(t, stubs[0].SurgePercent, paystub.SurgePercent)
			assert.Equal(t, stubs[0].UsageAtRest, paystub.UsageAtRest)
			assert.Equal(t, stubs[0].UsageGet, paystub.UsageGet)
			assert.Equal(t, stubs[0].UsageGetAudit, paystub.UsageGetAudit)
			assert.Equal(t, stubs[0].UsageGetRepair, paystub.UsageGetRepair)
			assert.Equal(t, stubs[0].UsagePut, paystub.UsagePut)
			assert.Equal(t, stubs[0].UsagePutRepair, paystub.UsagePutRepair)

			stubs, err = heldAmount.AllPayStubs(ctx, "")
			assert.Equal(t, len(stubs), 0)
			assert.NoError(t, err)
		})

		t.Run("Test SatellitesHeldbackHistory", func(t *testing.T) {
			heldback, err := heldAmount.SatellitesHeldbackHistory(ctx, satelliteID)
			assert.NoError(t, err)
			assert.Equal(t, heldback[0].Held, paystub.Held)
			assert.Equal(t, heldback[0].Period, paystub.Period)
		})
	})
}

func TestSatellitePayStubPeriodCached(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmountDB := db.HeldAmount()
		service := heldamount.NewService(nil, heldAmountDB, rpc.Dialer{}, nil)

		payStub := heldamount.PayStub{
			SatelliteID:    storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			Created:        time.Now().UTC(),
			Codes:          "code",
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

		for i := 1; i < 4; i++ {
			payStub.Period = fmt.Sprintf("2020-0%d", i)
			err := heldAmountDB.StorePayStub(ctx, payStub)
			require.NoError(t, err)
		}

		payStubs, err := service.SatellitePayStubPeriodCached(ctx, payStub.SatelliteID, "2020-01", "2020-03")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.SatellitePayStubPeriodCached(ctx, payStub.SatelliteID, "2019-01", "2021-03")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.SatellitePayStubPeriodCached(ctx, payStub.SatelliteID, "2019-01", "2020-01")
		require.NoError(t, err)
		require.Equal(t, 1, len(payStubs))
	})
}

func TestAllPayStubPeriodCached(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmountDB := db.HeldAmount()
		service := heldamount.NewService(nil, heldAmountDB, rpc.Dialer{}, nil)

		payStub := heldamount.PayStub{
			SatelliteID:    storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			Created:        time.Now().UTC(),
			Codes:          "code",
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

		for i := 1; i < 4; i++ {
			payStub.SatelliteID[0] += byte(i)
			for j := 1; j < 4; j++ {
				payStub.Period = fmt.Sprintf("2020-0%d", j)
				err := heldAmountDB.StorePayStub(ctx, payStub)
				require.NoError(t, err)
			}
		}

		payStubs, err := service.AllPayStubsPeriodCached(ctx, "2020-01", "2020-03")
		require.NoError(t, err)
		require.Equal(t, 9, len(payStubs))

		payStubs, err = service.AllPayStubsPeriodCached(ctx, "2019-01", "2021-03")
		require.NoError(t, err)
		require.Equal(t, 9, len(payStubs))

		payStubs, err = service.AllPayStubsPeriodCached(ctx, "2019-01", "2020-01")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.AllPayStubsPeriodCached(ctx, "2019-01", "2019-01")
		require.NoError(t, err)
		require.Equal(t, 0, len(payStubs))
	})
}

func TestAllHeldbackHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmountDB := db.HeldAmount()
		service := heldamount.NewService(nil, heldAmountDB, rpc.Dialer{}, nil)

		id := storj.NodeID{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}

		payStub := heldamount.PayStub{
			SatelliteID:    id,
			Created:        time.Now().UTC(),
			Codes:          "code",
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

		for j := 1; j < 5; j++ {
			payStub.Period = fmt.Sprintf("2020-0%d", j)
			err := heldAmountDB.StorePayStub(ctx, payStub)
			require.NoError(t, err)
		}

		heldback, err := service.AllHeldbackHistory(ctx, id)
		require.NoError(t, err)

		var expectedResult []heldamount.HeldbackPeriod

		period75 := heldamount.HeldbackPeriod{
			PercentageRate: 75,
			Held:           payStub.Held * 3,
		}
		period50 := heldamount.HeldbackPeriod{
			PercentageRate: 50,
			Held:           payStub.Held,
		}

		expectedResult = append(expectedResult, period75, period50)
		require.Equal(t, expectedResult, heldback)
	})
}
