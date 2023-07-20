// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/billing"
)

func TestChore(t *testing.T) {
	ts := makeTimestamp()

	const otherSource = "NOT-STORJCAN"

	var (
		mike   = testrand.UUID()
		joe    = testrand.UUID()
		robert = testrand.UUID()

		names = map[uuid.UUID]string{
			mike:   "mike",
			joe:    "joe",
			robert: "robert",
		}

		mike1   = makeFakeTransaction(mike, billing.StorjScanSource, billing.TransactionTypeCredit, 1000, ts, `{"fake": "mike1"}`)
		mike2   = makeFakeTransaction(mike, billing.StorjScanSource, billing.TransactionTypeCredit, 2000, ts.Add(time.Second*2), `{"fake": "mike2"}`)
		joe1    = makeFakeTransaction(joe, billing.StorjScanSource, billing.TransactionTypeCredit, 500, ts.Add(time.Second), `{"fake": "joe1"}`)
		joe2    = makeFakeTransaction(joe, billing.StorjScanSource, billing.TransactionTypeDebit, -100, ts.Add(time.Second), `{"fake": "joe1"}`)
		robert1 = makeFakeTransaction(robert, otherSource, billing.TransactionTypeCredit, 3000, ts.Add(time.Second), `{"fake": "robert1"}`)

		mike1Bonus = makeBonusTransaction(mike, 100, mike1.Timestamp, mike1.Metadata)
		mike2Bonus = makeBonusTransaction(mike, 200, mike2.Timestamp, mike2.Metadata)
		joe1Bonus  = makeBonusTransaction(joe, 50, joe1.Timestamp, joe1.Metadata)
	)

	assertTXs := func(ctx *testcontext.Context, t *testing.T, db billing.TransactionsDB, userID uuid.UUID, expectedTXs []billing.Transaction) {
		t.Helper()

		actualTXs, err := db.List(ctx, userID)
		require.NoError(t, err)
		for i := 0; i < len(expectedTXs) && i < len(actualTXs); i++ {
			assertTxEqual(t, expectedTXs[i], actualTXs[i], "unexpected transaction at index %d", i)
		}
		for i := len(expectedTXs); i < len(actualTXs); i++ {
			assert.Fail(t, "extra unexpected transaction", "index=%d tx=%+v", i, actualTXs[i])
		}
		for i := len(actualTXs); i < len(expectedTXs); i++ {
			assert.Fail(t, "missing expected transaction", "index=%d tx=%+v", i, expectedTXs[i])
		}
	}

	assertBalance := func(ctx *testcontext.Context, t *testing.T, db billing.TransactionsDB, userID uuid.UUID, expected currency.Amount) {
		t.Helper()
		actual, err := db.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual, "unexpected balance for user %s (%q)", userID, names[userID])
	}

	runTest := func(ctx *testcontext.Context, t *testing.T, consoleDB console.DB, db billing.TransactionsDB, bonusRate int64, mikeTXs, joeTXs, robertTXs []billing.Transaction, mikeBalance, joeBalance, robertBalance currency.Amount, usageLimitsConfig console.UsageLimitsConfig, userBalanceForUpgrade int64) {
		paymentTypes := []billing.PaymentType{
			newFakePaymentType(billing.StorjScanSource,
				[]billing.Transaction{mike1, joe1, joe2},
				[]billing.Transaction{mike2},
			),
			newFakePaymentType(otherSource,
				[]billing.Transaction{robert1},
			),
		}

		choreObservers := billing.ChoreObservers{
			UpgradeUser: console.NewUpgradeUserObserver(consoleDB, db, usageLimitsConfig, userBalanceForUpgrade),
		}

		chore := billing.NewChore(zaptest.NewLogger(t), paymentTypes, db, time.Hour, false, bonusRate, choreObservers)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		// Trigger (at least) two loops to process all batches.
		chore.TransactionCycle.Pause()
		chore.TransactionCycle.TriggerWait()
		chore.TransactionCycle.TriggerWait()
		chore.TransactionCycle.Pause()

		assertTXs(ctx, t, db, mike, mikeTXs)
		assertTXs(ctx, t, db, joe, joeTXs)
		assertTXs(ctx, t, db, robert, robertTXs)
		assertBalance(ctx, t, db, mike, mikeBalance)
		assertBalance(ctx, t, db, joe, joeBalance)
		assertBalance(ctx, t, db, robert, robertBalance)
	}

	t.Run("without StorjScan bonus", func(t *testing.T) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			db := sat.DB

			runTest(ctx, t, db.Console(), db.Billing(), 0,
				[]billing.Transaction{mike2, mike1},
				[]billing.Transaction{joe1, joe2},
				[]billing.Transaction{robert1},
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(4000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				sat.Config.Console.UsageLimits,
				sat.Config.Console.UserBalanceForUpgrade,
			)
		})
	})

	t.Run("with StorjScan bonus", func(t *testing.T) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			db := sat.DB

			runTest(ctx, t, db.Console(), db.Billing(), 10,
				[]billing.Transaction{mike2, mike2Bonus, mike1, mike1Bonus},
				[]billing.Transaction{joe1, joe1Bonus, joe2},
				[]billing.Transaction{robert1},
				currency.AmountFromBaseUnits(33000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(4500000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				sat.Config.Console.UsageLimits,
				sat.Config.Console.UserBalanceForUpgrade,
			)
		})
	})
}

func TestChore_UpgradeUserObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		usageLimitsConfig := sat.Config.Console.UsageLimits
		ts := makeTimestamp()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "choreobserver@mail.test",
		}, 1)
		require.NoError(t, err)

		_, err = sat.AddProject(ctx, user.ID, "Test Project")
		require.NoError(t, err)

		choreObservers := billing.ChoreObservers{
			UpgradeUser: console.NewUpgradeUserObserver(db.Console(), db.Billing(), sat.Config.Console.UsageLimits, sat.Config.Console.UserBalanceForUpgrade),
		}

		amount1 := int64(200) // $2
		amount2 := int64(800) // $8
		transaction1 := makeFakeTransaction(user.ID, billing.StorjScanSource, billing.TransactionTypeCredit, amount1, ts, `{"fake": "transaction1"}`)
		transaction2 := makeFakeTransaction(user.ID, billing.StorjScanSource, billing.TransactionTypeCredit, amount2, ts.Add(time.Second*2), `{"fake": "transaction2"}`)
		paymentTypes := []billing.PaymentType{
			newFakePaymentType(billing.StorjScanSource,
				[]billing.Transaction{transaction1},
				[]billing.Transaction{},
				[]billing.Transaction{transaction2},
				[]billing.Transaction{},
			),
		}

		chore := billing.NewChore(zaptest.NewLogger(t), paymentTypes, db.Billing(), time.Hour, false, 0, choreObservers)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		t.Run("user upgrade status", func(t *testing.T) {
			chore.TransactionCycle.Pause()
			chore.TransactionCycle.TriggerWait()
			chore.TransactionCycle.Pause()

			balance, err := db.Billing().GetBalance(ctx, user.ID)
			require.NoError(t, err)
			expected := currency.AmountFromBaseUnits(amount1*int64(10000), currency.USDollarsMicro)
			require.True(t, expected.Equal(balance))

			user, err = db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, user.PaidTier)

			projects, err := db.Console().Projects().GetOwn(ctx, user.ID)
			require.NoError(t, err)

			for _, p := range projects {
				require.Equal(t, usageLimitsConfig.Storage.Free, *p.StorageLimit)
				require.Equal(t, usageLimitsConfig.Bandwidth.Free, *p.BandwidthLimit)
				require.Equal(t, usageLimitsConfig.Segment.Free, *p.SegmentLimit)
			}

			chore.TransactionCycle.TriggerWait()
			chore.TransactionCycle.Pause()

			balance, err = db.Billing().GetBalance(ctx, user.ID)
			require.NoError(t, err)
			expected = currency.AmountFromBaseUnits((amount1+amount2)*int64(10000), currency.USDollarsMicro)
			require.True(t, expected.Equal(balance))

			user, err = db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.True(t, user.PaidTier)
			require.Equal(t, usageLimitsConfig.Storage.Paid.Int64(), user.ProjectStorageLimit)
			require.Equal(t, usageLimitsConfig.Bandwidth.Paid.Int64(), user.ProjectBandwidthLimit)
			require.Equal(t, usageLimitsConfig.Segment.Paid, user.ProjectSegmentLimit)
			require.Equal(t, usageLimitsConfig.Project.Paid, user.ProjectLimit)

			projects, err = db.Console().Projects().GetOwn(ctx, user.ID)
			require.NoError(t, err)

			for _, p := range projects {
				require.Equal(t, usageLimitsConfig.Storage.Paid, *p.StorageLimit)
				require.Equal(t, usageLimitsConfig.Bandwidth.Paid, *p.BandwidthLimit)
				require.Equal(t, usageLimitsConfig.Segment.Paid, *p.SegmentLimit)
			}
		})
	})
}

func makeFakeTransaction(userID uuid.UUID, source string, typ billing.TransactionType, amountUSD int64, timestamp time.Time, metadata string) billing.Transaction {
	return billing.Transaction{
		UserID:      userID,
		Amount:      currency.AmountFromBaseUnits(amountUSD, currency.USDollars),
		Description: fmt.Sprintf("%s transaction", source),
		Source:      source,
		Status:      billing.TransactionStatusCompleted,
		Type:        typ,
		Metadata:    []byte(metadata),
		Timestamp:   timestamp,
	}
}

func makeBonusTransaction(userID uuid.UUID, amountUSD int64, timestamp time.Time, metadata []byte) billing.Transaction {
	return billing.Transaction{
		UserID:      userID,
		Amount:      currency.AmountFromBaseUnits(amountUSD, currency.USDollars),
		Description: "STORJ Token Bonus (10%)",
		Source:      billing.StorjScanBonusSource,
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   timestamp,
	}
}

type fakePaymentType struct {
	source              string
	txType              billing.TransactionType
	txBatches           [][]billing.Transaction
	lastTransactionTime time.Time
	lastMetadata        []byte
}

func newFakePaymentType(source string, txBatches ...[]billing.Transaction) *fakePaymentType {
	return &fakePaymentType{
		source:    source,
		txType:    billing.TransactionTypeCredit,
		txBatches: txBatches,
	}
}

func (pt *fakePaymentType) Source() string                { return pt.source }
func (pt *fakePaymentType) Type() billing.TransactionType { return pt.txType }
func (pt *fakePaymentType) GetNewTransactions(_ context.Context, lastTransactionTime time.Time, metadata []byte) ([]billing.Transaction, error) {
	// Ensure that the chore is passing up the expected fields
	switch {
	case !pt.lastTransactionTime.Equal(lastTransactionTime):
		return nil, errs.New("expected last timestamp %q but got %q", pt.lastTransactionTime, lastTransactionTime)
	case !bytes.Equal(pt.lastMetadata, metadata):
		return nil, errs.New("expected metadata %q but got %q", string(pt.lastMetadata), string(metadata))
	}

	var txs []billing.Transaction
	if len(pt.txBatches) > 0 {
		txs = pt.txBatches[0]
		pt.txBatches = pt.txBatches[1:]
		if len(txs) > 0 {
			// Set up the next expected fields
			pt.lastTransactionTime = txs[len(txs)-1].Timestamp
			pt.lastMetadata = txs[len(txs)-1].Metadata
		}
	}
	return txs, nil
}

func assertTxEqual(t *testing.T, exp, act billing.Transaction, msgAndArgs ...interface{}) {
	// Assert that the actual transaction has a database id and created at date
	assert.NotZero(t, act.ID)
	assert.NotEqual(t, time.Time{}, act.CreatedAt)

	act.ID = 0
	exp.ID = 0
	act.CreatedAt = time.Time{}
	exp.CreatedAt = time.Time{}

	// Do a little hack to patch up the currency on the transactions since
	// the amount loaded from the database is likely in micro dollars.
	if exp.Amount.Currency() == currency.USDollars && act.Amount.Currency() == currency.USDollarsMicro {
		exp.Amount = currency.AmountFromDecimal(
			exp.Amount.AsDecimal().Truncate(act.Amount.Currency().DecimalPlaces()),
			act.Amount.Currency())
	}
	assert.Equal(t, exp, act, msgAndArgs...)
}
