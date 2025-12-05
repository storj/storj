// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/mailservice"
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

		mike1   = makeFakeTransaction(mike, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, 1000, ts, `{"fake": "mike1"}`)
		mike2   = makeFakeTransaction(mike, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, 2000, ts.Add(time.Second*2), `{"fake": "mike2"}`)
		joe1    = makeFakeTransaction(joe, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, 500, ts.Add(time.Second), `{"fake": "joe1"}`)
		joe2    = makeFakeTransaction(joe, billing.StorjScanEthereumSource, billing.TransactionTypeDebit, -100, ts.Add(time.Second), `{"fake": "joe1"}`)
		robert1 = makeFakeTransaction(robert, otherSource, billing.TransactionTypeCredit, 3000, ts.Add(time.Second), `{"fake": "robert1"}`)

		mike1Bonus = makeBonusTransaction(mike, 100, mike1.Timestamp, mike1.Metadata)
		mike2Bonus = makeBonusTransaction(mike, 200, mike2.Timestamp, mike2.Metadata)
		joe1Bonus  = makeBonusTransaction(joe, 50, joe1.Timestamp, joe1.Metadata)
	)

	assertTXs := func(ctx *testcontext.Context, t *testing.T, db billing.TransactionsDB, userID uuid.UUID, expectedTXs []billing.Transaction) {
		t.Helper()

		actualTXs, err := db.List(ctx, userID)
		require.NoError(t, err)

		for _, actualTX := range actualTXs {
			assert.NotZero(t, actualTX.ID, "ID from the database should not be zero")
			assert.NotZero(t, actualTX.CreatedAt, "CreatedAt from the database should not be zero")
		}

		// Spanner may retry the billing transaction inserts, without changing the data values to be inserted, so the order the
		// billing transactions are retrieved by tx timestamp might differ than the order they were sent to be inserted.
		// e.g. billing transaction A and billing transaction B have the same TxTimestamp as set by the test and are called
		// by the dbx Create method first with A and then B; however, especially with the emulator, A may be retried while B is not,
		// so the insertion into the database happens for B before A. When listing billing transactions by TxTimestamp, B may then be
		// returned by the database before A. Logically, the order should not matter, so compare billing transactions by looking up
		// their unique properties rather than relying on the exact (indeterminate) insertion and retrieval order.
		for i := len(expectedTXs) - 1; i >= 0; i-- {
			for j := 0; j < len(actualTXs); j++ {
				if transactionsAreEqual(expectedTXs[i], actualTXs[j]) {
					expectedTXs[i] = expectedTXs[len(expectedTXs)-1]
					expectedTXs = expectedTXs[:len(expectedTXs)-1]
					actualTXs = append(actualTXs[:j], actualTXs[j+1:]...)
					break
				}
			}
		}

		for _, missingTX := range expectedTXs {
			assert.Fail(t, "missing expected transaction", "tx=%+v", missingTX)
		}
		unexpectedTXs := actualTXs
		for _, unexpectedTX := range unexpectedTXs {
			assert.Fail(t, "extra unexpected transaction", "tx=%+v", unexpectedTX)
		}
	}

	assertBalance := func(ctx *testcontext.Context, t *testing.T, db billing.TransactionsDB, userID uuid.UUID, expected currency.Amount) {
		t.Helper()
		actual, err := db.GetBalance(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expected, actual, "unexpected balance for user %s (%q)", userID, names[userID])
	}

	runTest := func(ctx *testcontext.Context, t *testing.T, consoleDB console.DB, db billing.TransactionsDB, bonusRate int64,
		mikeTXs, joeTXs, robertTXs []billing.Transaction,
		mikeBalance, joeBalance, robertBalance currency.Amount,
		usageLimitsConfig console.UsageLimitsConfig,
		userBalanceForUpgrade int64,
		satelliteAddress string,
		freezeService *console.AccountFreezeService,
		analyticsService *analytics.Service,
		mailService *mailservice.Service,
	) {
		paymentTypes := []billing.PaymentType{
			newFakePaymentType(billing.StorjScanEthereumSource,
				[]billing.Transaction{mike1, joe1, joe2},
				[]billing.Transaction{mike2},
			),
			newFakePaymentType(otherSource,
				[]billing.Transaction{robert1},
			),
		}

		choreObservers := billing.ChoreObservers{
			UpgradeUser: console.NewUpgradeUserObserver(consoleDB, db, usageLimitsConfig, userBalanceForUpgrade, satelliteAddress, freezeService, analyticsService, mailService),
		}

		chore := billing.NewChore(zaptest.NewLogger(t), paymentTypes, db, time.Hour, false, bonusRate, choreObservers)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		// Trigger (at least) two loops to process all batches.
		chore.TransactionCycle.TriggerWait()
		chore.TransactionCycle.TriggerWait()

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

			freezeService := console.NewAccountFreezeService(db.Console(), sat.Core.Analytics.Service, sat.Config.Console.AccountFreeze)

			runTest(ctx, t, db.Console(), db.Billing(), 0,
				[]billing.Transaction{mike2, mike1},
				[]billing.Transaction{joe1, joe2},
				[]billing.Transaction{robert1},
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(4000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				sat.Config.Console.UsageLimits,
				sat.Config.Console.UserBalanceForUpgrade,
				sat.Config.Console.ExternalAddress,
				freezeService,
				sat.API.Analytics.Service,
				sat.API.Mail.Service,
			)
		})
	})

	t.Run("with StorjScan bonus", func(t *testing.T) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			db := sat.DB

			freezeService := console.NewAccountFreezeService(db.Console(), sat.Core.Analytics.Service, sat.Config.Console.AccountFreeze)

			runTest(ctx, t, db.Console(), db.Billing(), 10,
				[]billing.Transaction{mike2, mike2Bonus, mike1, mike1Bonus},
				[]billing.Transaction{joe1, joe1Bonus, joe2},
				[]billing.Transaction{robert1},
				currency.AmountFromBaseUnits(33000000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(4500000, currency.USDollarsMicro),
				currency.AmountFromBaseUnits(30000000, currency.USDollarsMicro),
				sat.Config.Console.UsageLimits,
				sat.Config.Console.UserBalanceForUpgrade,
				sat.Config.Console.ExternalAddress,
				freezeService,
				sat.API.Analytics.Service,
				sat.API.Mail.Service,
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

		freezeService := console.NewAccountFreezeService(db.Console(), sat.Core.Analytics.Service, sat.Config.Console.AccountFreeze)

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "choreobserver@mail.test",
		}, 1)
		require.NoError(t, err)

		user2, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "choreobserver2@mail.test",
		}, 1)
		require.NoError(t, err)

		user3, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "choreobserver3@mail.test",
		}, 1)
		require.NoError(t, err)

		_, err = sat.AddProject(ctx, user.ID, "Test Project")
		require.NoError(t, err)

		choreObservers := billing.ChoreObservers{
			UpgradeUser: console.NewUpgradeUserObserver(db.Console(), db.Billing(), sat.Config.Console.UsageLimits, sat.Config.Console.UserBalanceForUpgrade, sat.Config.Console.ExternalAddress, freezeService, sat.API.Analytics.Service, sat.API.Mail.Service),
		}

		amount1 := int64(200) // $2
		amount2 := int64(800) // $8
		transaction1 := makeFakeTransaction(user.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount1, ts, `{"fake": "transaction1"}`)
		transaction2 := makeFakeTransaction(user.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount2, ts.Add(time.Second*2), `{"fake": "transaction2"}`)
		transaction3 := makeFakeTransaction(user2.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount1+amount2, ts, `{"fake": "transaction3"}`)
		transaction4 := makeFakeTransaction(user3.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount1+amount2, ts.Add(time.Second*2), `{"fake": "transaction4"}`)
		paymentTypes := []billing.PaymentType{
			newFakePaymentType(billing.StorjScanEthereumSource,
				[]billing.Transaction{transaction1},
				[]billing.Transaction{},
				[]billing.Transaction{transaction2},
				[]billing.Transaction{},
				[]billing.Transaction{transaction3},
				[]billing.Transaction{transaction4},
				[]billing.Transaction{},
			),
		}

		chore := billing.NewChore(zaptest.NewLogger(t), paymentTypes, db.Billing(), time.Hour, false, 0, choreObservers)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		chore.TransactionCycle.Pause()

		t.Run("user upgrade status", func(t *testing.T) {
			chore.TransactionCycle.TriggerWait()

			balance, err := db.Billing().GetBalance(ctx, user.ID)
			require.NoError(t, err)
			expected := currency.AmountFromBaseUnits(amount1*int64(10000), currency.USDollarsMicro)
			require.True(t, expected.Equal(balance))

			user, err = db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.FreeUser, user.Kind)

			projects, err := db.Console().Projects().GetOwn(ctx, user.ID)
			require.NoError(t, err)

			for _, p := range projects {
				require.Equal(t, usageLimitsConfig.Storage.Free, *p.StorageLimit)
				require.Equal(t, usageLimitsConfig.Bandwidth.Free, *p.BandwidthLimit)
				require.Equal(t, usageLimitsConfig.Segment.Free, *p.SegmentLimit)
			}

			now := time.Now()
			choreObservers.UpgradeUser.TestSetNow(func() time.Time {
				return now
			})

			chore.TransactionCycle.TriggerWait()

			balance, err = db.Billing().GetBalance(ctx, user.ID)
			require.NoError(t, err)
			expected = currency.AmountFromBaseUnits((amount1+amount2)*int64(10000), currency.USDollarsMicro)
			require.True(t, expected.Equal(balance))

			user, err = db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Equal(t, console.PaidUser, user.Kind)
			require.WithinDuration(t, now, *user.UpgradeTime, time.Minute)
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

		t.Run("no upgrade for legal/violation freeze", func(t *testing.T) {
			require.NoError(t, freezeService.LegalFreezeUser(ctx, user2.ID))
			require.NoError(t, freezeService.ViolationFreezeUser(ctx, user3.ID))

			chore.TransactionCycle.TriggerWait()

			expected := currency.AmountFromBaseUnits((amount1+amount2)*int64(10000), currency.USDollarsMicro)

			chore.TransactionCycle.TriggerWait()

			balance, err := db.Billing().GetBalance(ctx, user2.ID)
			require.NoError(t, err)
			require.True(t, expected.Equal(balance))

			chore.TransactionCycle.TriggerWait()

			balance, err = db.Billing().GetBalance(ctx, user3.ID)
			require.NoError(t, err)
			require.True(t, expected.Equal(balance))

			// users should not be upgraded though they have enough balance
			// since they are in legal/violation freeze.
			user, err = db.Console().Users().Get(ctx, user2.ID)
			require.NoError(t, err)
			require.Equal(t, console.FreeUser, user.Kind)

			user, err = db.Console().Users().Get(ctx, user3.ID)
			require.NoError(t, err)
			require.Equal(t, console.FreeUser, user.Kind)
		})
	})
}

func TestChore_PayInvoiceObserver(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		db := sat.DB
		consoleDB := db.Console()
		invoicesDB := sat.Core.Payments.Accounts.Invoices()
		stripeClient := sat.API.Payments.StripeClient
		customerDB := sat.Core.DB.StripeCoinPayments().Customers()
		ts := makeTimestamp()

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "choreobserver@mail.test",
		}, 1)
		require.NoError(t, err)

		cus, err := customerDB.GetCustomerID(ctx, user.ID)
		require.NoError(t, err)

		// setup storjscan wallet
		address, err := blockchain.BytesToAddress(testrand.Bytes(20))
		require.NoError(t, err)
		userID := user.ID
		err = sat.DB.Wallets().Add(ctx, userID, address)
		require.NoError(t, err)

		freezeService := console.NewAccountFreezeService(consoleDB, sat.Core.Analytics.Service, sat.Config.Console.AccountFreeze)

		choreObservers := billing.ChoreObservers{
			UpgradeUser: console.NewUpgradeUserObserver(consoleDB, db.Billing(), sat.Config.Console.UsageLimits, sat.Config.Console.UserBalanceForUpgrade, sat.Config.Console.ExternalAddress, freezeService, sat.API.Analytics.Service, sat.API.Mail.Service),
			PayInvoices: console.NewInvoiceTokenPaymentObserver(consoleDB, sat.Core.Payments.Accounts.Invoices(), freezeService),
		}

		amount := int64(2000)  // $20
		amount2 := int64(1000) // $10
		transaction := makeFakeTransaction(user.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount, ts, `{"fake": "transaction"}`)
		transaction2 := makeFakeTransaction(user.ID, billing.StorjScanEthereumSource, billing.TransactionTypeCredit, amount2, ts.Add(time.Second*2), `{"fake": "transaction2"}`)
		paymentTypes := []billing.PaymentType{
			newFakePaymentType(billing.StorjScanEthereumSource,
				[]billing.Transaction{transaction},
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

		// create invoice
		inv, err := stripeClient.Invoices().New(&stripe.InvoiceParams{
			Params:   stripe.Params{Context: ctx},
			Customer: &cus,
		})
		require.NoError(t, err)

		_, err = stripeClient.InvoiceItems().New(&stripe.InvoiceItemParams{
			Params:   stripe.Params{Context: ctx},
			Amount:   stripe.Int64(amount + amount2),
			Currency: stripe.String(string(stripe.CurrencyUSD)),
			Customer: &cus,
			Invoice:  &inv.ID,
		})
		require.NoError(t, err)

		inv, err = stripeClient.Invoices().FinalizeInvoice(inv.ID, nil)
		require.NoError(t, err)
		require.Equal(t, stripe.InvoiceStatusOpen, inv.Status)

		invoices, err := invoicesDB.List(ctx, user.ID)
		require.NoError(t, err)
		require.NotEmpty(t, invoices)
		require.Equal(t, inv.ID, invoices[0].ID)
		require.Equal(t, inv.ID, invoices[0].ID)
		require.Equal(t, string(inv.Status), invoices[0].Status)

		err = freezeService.BillingFreezeUser(ctx, userID)
		require.NoError(t, err)

		chore.TransactionCycle.TriggerWait()

		// user balance would've been the value of amount ($20) but
		// PayInvoiceObserver will use this to pay part of this user's invoice.
		balance, err := db.Billing().GetBalance(ctx, user.ID)
		require.NoError(t, err)
		require.Zero(t, balance.BaseUnits())

		invoices, err = invoicesDB.List(ctx, user.ID)
		require.NoError(t, err)
		require.NotEmpty(t, invoices)
		// invoice remains unpaid since only $20 was paid.
		require.Equal(t, string(stripe.InvoiceStatusOpen), invoices[0].Status)

		// user remains frozen since payment is not complete.
		frozen, err := freezeService.IsUserBillingFrozen(ctx, userID)
		require.NoError(t, err)
		require.True(t, frozen)

		chore.TransactionCycle.TriggerWait()

		// the second transaction of $10 reflects at this point and
		// is used to pay for the remaining invoice balance.
		invoices, err = invoicesDB.List(ctx, user.ID)
		require.NoError(t, err)
		require.NotEmpty(t, invoices)
		require.Equal(t, string(stripe.InvoiceStatusPaid), invoices[0].Status)

		// user is not frozen since payment is complete.
		frozen, err = freezeService.IsUserBillingFrozen(ctx, userID)
		require.NoError(t, err)
		require.False(t, frozen)
	})
}

func makeFakeTransaction(userID uuid.UUID, source string, typ billing.TransactionType, amountUSD int64, timestamp time.Time, metadata string) billing.Transaction {
	return billing.Transaction{
		UserID:      userID,
		Amount:      currency.AmountFromBaseUnits(amountUSD, currency.USDollars),
		Description: source + " transaction",
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

func (pt *fakePaymentType) Sources() []string             { return []string{pt.source} }
func (pt *fakePaymentType) Type() billing.TransactionType { return pt.txType }
func (pt *fakePaymentType) GetNewTransactions(_ context.Context, _ string, lastTransactionTime time.Time, metadata []byte) ([]billing.Transaction, error) {
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

func transactionsAreEqual(expected, actual billing.Transaction) bool {
	// ignore anything created by the database
	expected.ID = 0
	actual.ID = 0
	expected.CreatedAt = time.Time{}
	actual.CreatedAt = time.Time{}

	// Do a little hack to patch up the currency on the transactions since
	// the amount loaded from the database is likely in micro dollars.
	if expected.Amount.Currency() == currency.USDollars && actual.Amount.Currency() == currency.USDollarsMicro {
		expected.Amount = currency.AmountFromDecimal(
			expected.Amount.AsDecimal().Truncate(actual.Amount.Currency().DecimalPlaces()),
			actual.Amount.Currency())
	}

	return cmp.Equal(expected, actual, cmpopts.EquateApproxTime(0))
}
