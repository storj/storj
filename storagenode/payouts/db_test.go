// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package payouts_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
)

func TestHeldAmountDB(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payout := db.Payout()
		satelliteID := testrand.NodeID()
		period := "2020-01"
		paystub := payouts.PayStub{
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
		}
		paystub2 := paystub
		paystub2.Period = "2020-02"
		paystub2.Created = paystub.Created.Add(time.Hour * 24 * 30)

		t.Run("StorePayStub", func(t *testing.T) {
			err := payout.StorePayStub(ctx, paystub)
			assert.NoError(t, err)
		})

		payment := payouts.Payment{
			SatelliteID: satelliteID,
			Period:      period,
			Receipt:     "test",
		}

		t.Run("GetPayStub", func(t *testing.T) {
			err := payout.StorePayment(ctx, payment)
			assert.NoError(t, err)

			stub, err := payout.GetPayStub(ctx, satelliteID, period)
			assert.NoError(t, err)
			receipt, err := payout.GetReceipt(ctx, satelliteID, period)
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
			assert.Equal(t, receipt, payment.Receipt)

			stub, err = payout.GetPayStub(ctx, satelliteID, "")
			assert.Error(t, err)
			assert.Equal(t, true, payouts.ErrNoPayStubForPeriod.Has(err))
			assert.Nil(t, stub)
			assert.NotNil(t, receipt)
			receipt, err = payout.GetReceipt(ctx, satelliteID, "")
			assert.Error(t, err)
			assert.Equal(t, true, payouts.ErrNoPayStubForPeriod.Has(err))

			stub, err = payout.GetPayStub(ctx, testrand.NodeID(), period)
			assert.Error(t, err)
			assert.Equal(t, true, payouts.ErrNoPayStubForPeriod.Has(err))
			assert.Nil(t, stub)
			assert.NotNil(t, receipt)
		})

		t.Run("AllPayStubs", func(t *testing.T) {
			stubs, err := payout.AllPayStubs(ctx, period)
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

			stubs, err = payout.AllPayStubs(ctx, "")
			assert.Equal(t, len(stubs), 0)
			assert.NoError(t, err)
		})

		payment = payouts.Payment{
			ID:          1,
			Created:     time.Now().UTC(),
			SatelliteID: satelliteID,
			Period:      period,
			Amount:      228,
			Receipt:     "receipt",
			Notes:       "notes",
		}

		t.Run("StorePayment", func(t *testing.T) {
			err := payout.StorePayment(ctx, payment)
			assert.NoError(t, err)
		})

		t.Run("SatellitesHeldbackHistory", func(t *testing.T) {
			heldback, err := payout.SatellitesHeldbackHistory(ctx, satelliteID)
			assert.NoError(t, err)
			assert.Equal(t, heldback[0].Amount, paystub.Held)
			assert.Equal(t, heldback[0].Period, paystub.Period)
		})

		t.Run("SatellitePeriods", func(t *testing.T) {
			periods, err := payout.SatellitePeriods(ctx, paystub.SatelliteID)
			assert.NoError(t, err)
			assert.NotNil(t, periods)
			assert.Equal(t, 1, len(periods))
			assert.Equal(t, paystub.Period, periods[0])

			err = payout.StorePayStub(ctx, paystub2)
			require.NoError(t, err)

			periods, err = payout.SatellitePeriods(ctx, paystub.SatelliteID)
			assert.NoError(t, err)
			assert.NotNil(t, periods)
			assert.Equal(t, 2, len(periods))
			assert.Equal(t, paystub.Period, periods[0])
			assert.Equal(t, paystub2.Period, periods[1])
		})

		t.Run("AllPeriods", func(t *testing.T) {
			periods, err := payout.AllPeriods(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, periods)
			assert.Equal(t, 2, len(periods))
			assert.Equal(t, paystub.Period, periods[0])
			assert.Equal(t, paystub2.Period, periods[1])

			paystub3 := paystub2
			paystub3.SatelliteID = testrand.NodeID()
			paystub3.Period = "2020-03"
			paystub3.Created = paystub2.Created.Add(time.Hour * 24 * 30)

			err = payout.StorePayStub(ctx, paystub3)
			require.NoError(t, err)

			periods, err = payout.AllPeriods(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, periods)
			assert.Equal(t, 3, len(periods))
			assert.Equal(t, paystub.Period, periods[0])
			assert.Equal(t, paystub2.Period, periods[1])
			assert.Equal(t, paystub3.Period, periods[2])
		})
	})
}

func TestSatellitePayStubPeriodCached(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmountDB := db.Payout()
		reputationDB := db.Reputation()
		satellitesDB := db.Satellites()
		service, err := payouts.NewService(nil, heldAmountDB, reputationDB, satellitesDB, nil)
		require.NoError(t, err)

		payStub := payouts.PayStub{
			SatelliteID:    testrand.NodeID(),
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

		payStubs, err := service.SatellitePayStubPeriod(ctx, payStub.SatelliteID, "2020-01", "2020-03")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.SatellitePayStubPeriod(ctx, payStub.SatelliteID, "2019-01", "2021-03")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.SatellitePayStubPeriod(ctx, payStub.SatelliteID, "2019-01", "2020-01")
		require.NoError(t, err)
		require.Equal(t, 1, len(payStubs))
	})
}

func TestAllPayStubPeriodCached(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		heldAmountDB := db.Payout()
		reputationDB := db.Reputation()
		satellitesDB := db.Satellites()
		service, err := payouts.NewService(nil, heldAmountDB, reputationDB, satellitesDB, nil)
		require.NoError(t, err)

		payStub := payouts.PayStub{
			SatelliteID:    testrand.NodeID(),
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

		payStubs, err := service.AllPayStubsPeriod(ctx, "2020-01", "2020-03")
		require.NoError(t, err)
		require.Equal(t, 9, len(payStubs))

		payStubs, err = service.AllPayStubsPeriod(ctx, "2019-01", "2021-03")
		require.NoError(t, err)
		require.Equal(t, 9, len(payStubs))

		payStubs, err = service.AllPayStubsPeriod(ctx, "2019-01", "2020-01")
		require.NoError(t, err)
		require.Equal(t, 3, len(payStubs))

		payStubs, err = service.AllPayStubsPeriod(ctx, "2019-01", "2019-01")
		require.NoError(t, err)
		require.Equal(t, 0, len(payStubs))
	})
}

func TestPayouts(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payout := db.Payout()
		t.Run("SatelliteIDs", func(t *testing.T) {
			id1 := testrand.NodeID()
			id2 := testrand.NodeID()
			id3 := testrand.NodeID()
			err := payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id1,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id1,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id2,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id3,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id3,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: id2,
			})
			require.NoError(t, err)
			listIDs, err := payout.GetPayingSatellitesIDs(ctx)
			require.Equal(t, len(listIDs), 3)
			require.NoError(t, err)
		})
		t.Run("GetSatelliteEarned", func(t *testing.T) {
			id1 := testrand.NodeID()
			id2 := testrand.NodeID()
			id3 := testrand.NodeID()
			err := payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-11",
				SatelliteID: id1,
				CompGet:     11,
				CompAtRest:  11,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-12",
				SatelliteID: id1,
				CompGet:     22,
				CompAtRest:  22,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-11",
				SatelliteID: id2,
				CompGet:     33,
				CompAtRest:  33,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-10",
				SatelliteID: id3,
				CompGet:     44,
				CompAtRest:  44,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-11",
				SatelliteID: id3,
				CompGet:     55,
				CompAtRest:  55,
			})
			require.NoError(t, err)
			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-10",
				SatelliteID: id2,
				CompGet:     66,
				CompAtRest:  66,
			})
			require.NoError(t, err)
			satellite1Earned, err := payout.GetEarnedAtSatellite(ctx, id1)
			require.Equal(t, int(satellite1Earned), 66)
			require.NoError(t, err)
			satellite2Earned, err := payout.GetEarnedAtSatellite(ctx, id2)
			require.Equal(t, int(satellite2Earned), 198)
			require.NoError(t, err)
			satellite3Earned, err := payout.GetEarnedAtSatellite(ctx, id3)
			require.Equal(t, int(satellite3Earned), 198)
			require.NoError(t, err)
		})

		t.Run("GetSatelliteSummary", func(t *testing.T) {
			id1 := testrand.NodeID()
			id2 := testrand.NodeID()

			err := payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-11",
				SatelliteID: id1,
				Paid:        11,
				Held:        11,
			})
			require.NoError(t, err)

			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-10",
				SatelliteID: id1,
				Paid:        22,
				Held:        22,
			})
			require.NoError(t, err)

			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-11",
				SatelliteID: id2,
				Paid:        33,
				Held:        33,
			})
			require.NoError(t, err)

			err = payout.StorePayStub(ctx, payouts.PayStub{
				Period:      "2020-10",
				SatelliteID: id2,
				Paid:        66,
				Held:        66,
			})
			require.NoError(t, err)

			paid, held, err := payout.GetSatellitePeriodSummary(ctx, id1, "2020-10")
			require.NoError(t, err)
			require.Equal(t, paid, int64(22))
			require.Equal(t, held, int64(22))

			paid2, held2, err := payout.GetSatelliteSummary(ctx, id2)
			require.NoError(t, err)
			require.Equal(t, paid2, int64(99))
			require.Equal(t, held2, int64(99))
		})
	})
}

func TestUndistributed(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payoutdb := db.Payout()
		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()

		t.Run("empty db no error", func(t *testing.T) {
			undistributed, err := payoutdb.GetUndistributed(ctx)
			require.NoError(t, err)
			require.EqualValues(t, undistributed, 0)
		})

		t.Run("few paystubs with different satellites", func(t *testing.T) {
			err := payoutdb.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: satelliteID2,
				Period:      "2020-01",
				Distributed: 150,
				Paid:        250,
			})
			require.NoError(t, err)

			err = payoutdb.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: satelliteID2,
				Period:      "2020-02",
				Distributed: 250,
				Paid:        350,
			})
			require.NoError(t, err)

			err = payoutdb.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: satelliteID1,
				Period:      "2020-01",
				Distributed: 100,
				Paid:        300,
			})
			require.NoError(t, err)

			err = payoutdb.StorePayStub(ctx, payouts.PayStub{
				SatelliteID: satelliteID1,
				Period:      "2020-02",
				Distributed: 400,
				Paid:        500,
			})
			require.NoError(t, err)

			undistributed, err := payoutdb.GetUndistributed(ctx)
			require.NoError(t, err)
			require.EqualValues(t, undistributed, 500)
		})
	})
}

func TestSummedPaystubs(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payoutdb := db.Payout()
		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()

		err := payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID2,
			Period:      "2020-01",
			Distributed: 150,
			Paid:        250,
			Disposed:    100,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID2,
			Period:      "2020-02",
			Distributed: 250,
			Paid:        350,
			Disposed:    200,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID1,
			Period:      "2020-01",
			Distributed: 100,
			Paid:        300,
			Disposed:    300,
		})
		require.NoError(t, err)

		err = payoutdb.StorePayStub(ctx, payouts.PayStub{
			SatelliteID: satelliteID1,
			Period:      "2020-02",
			Distributed: 400,
			Paid:        500,
			Disposed:    400,
		})
		require.NoError(t, err)

		t.Run("all satellites period", func(t *testing.T) {
			paystub, err := payoutdb.GetSatellitePaystubs(ctx, satelliteID1)
			require.NoError(t, err)
			require.EqualValues(t, paystub.Distributed, 500)
			require.EqualValues(t, paystub.Paid, 800)
			require.EqualValues(t, paystub.Disposed, 700)
		})

		t.Run("satellites period", func(t *testing.T) {
			paystub, err := payoutdb.GetPaystubs(ctx)
			require.NoError(t, err)
			require.EqualValues(t, paystub.Distributed, 900)
			require.EqualValues(t, paystub.Paid, 1400)
			require.EqualValues(t, paystub.Disposed, 1000)
		})

		t.Run("all satellites period", func(t *testing.T) {
			paystub, err := payoutdb.GetPeriodPaystubs(ctx, "2020-01")
			require.NoError(t, err)
			require.EqualValues(t, paystub.Distributed, 250)
			require.EqualValues(t, paystub.Paid, 550)
			require.EqualValues(t, paystub.Disposed, 400)
		})

		t.Run("satellites period", func(t *testing.T) {
			paystub, err := payoutdb.GetSatellitePeriodPaystubs(ctx, "2020-02", satelliteID2)
			require.NoError(t, err)
			require.EqualValues(t, paystub.Distributed, 250)
			require.EqualValues(t, paystub.Paid, 350)
			require.EqualValues(t, paystub.Disposed, 200)
		})
	})
}

func TestDBHeldAmountHistory(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {
		payoutsDB := db.Payout()
		satelliteID1 := testrand.NodeID()
		satelliteID2 := testrand.NodeID()

		paystubs := []payouts.PayStub{
			{
				SatelliteID: satelliteID1,
				Period:      "2021-01",
				Held:        10,
			},
			{
				SatelliteID: satelliteID1,
				Period:      "2021-02",
				Held:        10,
			},
			{
				SatelliteID: satelliteID1,
				Period:      "2021-03",
				Held:        10,
			},
			{
				SatelliteID: satelliteID1,
				Period:      "2021-04",
				Held:        10,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-01",
				Held:        0,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-02",
				Held:        0,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-03",
				Held:        0,
			},
			{
				SatelliteID: satelliteID2,
				Period:      "2021-04",
				Held:        0,
			},
		}

		expected := []payouts.HeldAmountHistory{
			{
				SatelliteID: satelliteID1,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 10,
					},
					{
						Period: "2021-02",
						Amount: 10,
					},
					{
						Period: "2021-03",
						Amount: 10,
					},
					{
						Period: "2021-04",
						Amount: 10,
					},
				},
			},
			{
				SatelliteID: satelliteID2,
				HeldAmounts: []payouts.HeldForPeriod{
					{
						Period: "2021-01",
						Amount: 0,
					},
					{
						Period: "2021-02",
						Amount: 0,
					},
					{
						Period: "2021-03",
						Amount: 0,
					},
					{
						Period: "2021-04",
						Amount: 0,
					},
				},
			},
		}

		for _, paystub := range paystubs {
			err := payoutsDB.StorePayStub(ctx, paystub)
			require.NoError(t, err)
		}

		history, err := payoutsDB.HeldAmountHistory(ctx)
		require.NoError(t, err)
		require.ElementsMatch(t, expected, history)
	})
}
