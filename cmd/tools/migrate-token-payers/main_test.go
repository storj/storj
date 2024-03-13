// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/currency"
	"storj.io/common/dbutil/tempdb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	migrator "storj.io/storj/cmd/tools/migrate-token-payers"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test no entries in table doesn't error.
func TestMigrateUsersSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepareEmpty := func(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User) {
		return wouldBeUpdated, wouldNotBeUpdated
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
	}
	test(t, prepareEmpty, check, &migrator.Config{
		Limit:   8,
		Verbose: true,
	})
}

func insertUser(ctx context.Context, db satellite.DB, status console.UserStatus) (*console.User, error) {
	u, err := db.Console().Users().Insert(ctx, &console.User{
		ID:           testrand.UUID(),
		Email:        "test@storj.test",
		FullName:     "abcdefg hijklmnop",
		PasswordHash: []byte{0, 1, 2, 3, 4},
	})
	if err != nil {
		return nil, err
	}
	if status != console.Inactive { // Inactive is default
		err = db.Console().Users().Update(ctx, u.ID, console.UpdateUserRequest{
			Status: &status,
		})
		if err != nil {
			return nil, err
		}
		u, err = db.Console().Users().Get(ctx, u.ID)
		if err != nil {
			return nil, err
		}
	}
	return u, err
}

func insertPayments(ctx context.Context, db satellite.DB, userID uuid.UUID, billingBalance, coinpayment bool) error {
	if billingBalance {
		_, err := db.Billing().Insert(ctx, billing.Transaction{
			UserID:      userID,
			Amount:      currency.AmountFromBaseUnits(1000, currency.USDollars),
			Description: "STORJ deposit",
			Source:      billing.StorjScanEthereumSource,
			Status:      billing.TransactionStatusCompleted,
			Type:        billing.TransactionTypeCredit,
			Metadata:    []byte(`{"fake": "test"}`),
			Timestamp:   time.Now(),
		})
		if err != nil {
			return err
		}
	}
	if coinpayment {
		_, err := db.StripeCoinPayments().Transactions().TestInsert(ctx, stripe.Transaction{
			ID:        coinpayments.TransactionID(userID.String()),
			AccountID: userID,
			Address:   "0123456789",
			Amount:    currency.AmountFromBaseUnits(1000, currency.USDollars),
			Received:  currency.AmountFromBaseUnits(1000, currency.USDollars),
			Status:    coinpayments.StatusCompleted,
			Key:       "01238902345867234",
			Timeout:   time.Hour,
			CreatedAt: time.Now(),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func prepare(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User) {
	// 3 users expected for update
	for i := 0; i < 4; i++ {
		u, err := insertUser(ctx, db, console.UserStatus(1))
		require.NoError(t, err)
		require.NotNil(t, u)
		wouldBeUpdated = append(wouldBeUpdated, *u)
	}

	// test all user statuses. 1 is expected for update above.
	for i := 0; i < 6; i++ {
		if i == 1 {
			continue
		}
		u, err := insertUser(ctx, db, console.UserStatus(i))
		require.NoError(t, err)
		require.NotNil(t, u)
		wouldNotBeUpdated = append(wouldNotBeUpdated, *u)
	}

	// insert one active user who will not be migrated
	u, err := insertUser(ctx, db, console.UserStatus(1))
	require.NoError(t, err)
	require.NotNil(t, u)
	wouldNotBeUpdated = append(wouldNotBeUpdated, *u)

	// insert payments for users expected for update
	for i, user := range wouldBeUpdated {
		switch i {
		case 0:
			require.NoError(t, insertPayments(ctx, db, user.ID, true, false))
		case 1:
			require.NoError(t, insertPayments(ctx, db, user.ID, false, true))
		default:
			require.NoError(t, insertPayments(ctx, db, user.ID, true, true))
		}
	}

	// insert payments for users not expected for update due to user status (except one, see comment below)
	var insertedBalance, insertedCoinpayments bool
	for _, user := range wouldNotBeUpdated {
		if user.Status == 1 { // make sure active free tier user without payments is not updated
			continue
		}
		if !insertedBalance {
			require.NoError(t, insertPayments(ctx, db, user.ID, true, false))
			insertedBalance = true
		} else if !insertedCoinpayments {
			require.NoError(t, insertPayments(ctx, db, user.ID, false, true))
			insertedCoinpayments = true
		} else {
			require.NoError(t, insertPayments(ctx, db, user.ID, true, true))
		}
	}
	return wouldBeUpdated, wouldNotBeUpdated
}

// Test paid_tier field is updated correctly.
func TestMigrateUsers(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.True(t, u.PaidTier)
		}
		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, u.PaidTier)
		}
	}

	test(t, prepare, check, &migrator.Config{
		Limit:   1000,
		Verbose: true,
	})
}

// Test Limit limits number of users updated.
func TestMigrateUsersLimit(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	limit := 1
	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		var updated int
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			if u.PaidTier {
				updated++
			}
		}

		require.Equal(t, limit, updated)

		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, u.PaidTier)
		}
	}

	test(t, prepare, check, &migrator.Config{
		Limit:   limit,
		Verbose: true,
	})
}

// Test dry run results in no updates.
func TestMigrateUsersDryRun(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		// Would have been updated, had we not set the dry run option. We are asserting paid tier was not set.
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, u.PaidTier)
		}
		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.False(t, u.PaidTier)
		}
	}

	test(t, prepare, check, &migrator.Config{
		Limit:   1000,
		DryRun:  true,
		Verbose: true,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User),
	check func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User), config *migrator.Config) {

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	for _, satelliteDB := range satellitedbtest.Databases() {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			tempDB, err := tempdb.OpenUnique(ctx, satelliteDB.MasterDB.URL, schema)
			require.NoError(t, err)

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB, "migrate-token-payers")
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.Testing().TestMigrateToLatest(ctx)
			require.NoError(t, err)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			wouldBeUpdated, wouldNotBeUpdated := prepare(t, ctx, db)

			err = migrator.Migrate(ctx, log, conn, *config)
			require.NoError(t, err)

			require.NoError(t, err)

			check(t, ctx, db, wouldBeUpdated, wouldNotBeUpdated)
		})
	}
}
