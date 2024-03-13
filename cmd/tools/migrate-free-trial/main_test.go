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

	"storj.io/common/dbutil/tempdb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	migrator "storj.io/storj/cmd/tools/migrate-free-trial"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test no entries in table doesn't error.
func TestMigrateUsersSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trialExp := time.Now()
	notifyCount := 3
	prepareEmpty := func(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User) {
		return wouldBeUpdated, wouldNotBeUpdated
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
	}
	test(t, prepareEmpty, check, trialExp, notifyCount, &migrator.Config{
		Limit:   8,
		Verbose: true,
	})
}

func insertUser(ctx context.Context, db satellite.DB, paidTier, setTrialExp bool) (*console.User, error) {
	user := &console.User{
		ID:           testrand.UUID(),
		Email:        "test@storj.test",
		FullName:     "abcdefg hijklmnop",
		PasswordHash: []byte{0, 1, 2, 3, 4},
	}
	if setTrialExp {
		now := time.Now().Add(-24 * time.Hour)
		user.TrialExpiration = &now
	}
	u, err := db.Console().Users().Insert(ctx, user)
	if err != nil {
		return nil, err
	}
	if paidTier {
		// Insert doesn't set paid_tier
		err = db.Console().Users().Update(ctx, u.ID, console.UpdateUserRequest{PaidTier: &paidTier})
		if err != nil {
			return nil, err
		}
		u.PaidTier = true
	}
	return u, err
}

func prepare(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User) {
	// insert one user where paid tier is true
	u, err := insertUser(ctx, db, true, false)
	require.NoError(t, err)
	require.NotNil(t, u)
	wouldNotBeUpdated = append(wouldNotBeUpdated, *u)

	// insert one user where paid tier is false and trial expiration is not null
	u, err = insertUser(ctx, db, false, true)
	require.NoError(t, err)
	require.NotNil(t, u)
	wouldNotBeUpdated = append(wouldNotBeUpdated, *u)

	// insert two users where paid tier is false and trial expiration is null
	for i := 0; i < 2; i++ {
		u, err = insertUser(ctx, db, false, false)
		require.NoError(t, err)
		require.NotNil(t, u)
		wouldBeUpdated = append(wouldBeUpdated, *u)
	}

	return wouldBeUpdated, wouldNotBeUpdated
}

// Test trial_expiration field is updated correctly.
func TestMigrateUsers(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trialExp := time.Now()
	notifyCount := 3

	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.NotNil(t, u.TrialExpiration)
			require.Equal(t, trialExp.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond))
			require.Equal(t, notifyCount, u.TrialNotifications)
		}
		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			if !u.PaidTier {
				require.NotNil(t, u.TrialExpiration)
				require.NotEqual(t, trialExp.Truncate(time.Millisecond), u.TrialExpiration.Truncate(time.Millisecond))
			}
		}
	}

	test(t, prepare, check, trialExp, notifyCount, &migrator.Config{
		Limit:   1,
		Verbose: true,
	})
}

// Test Limit limits number of users updated.
func TestMigrateUsersLimit(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trialExp := time.Now()
	notifyCount := 3
	maxUpdates := 1
	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		var updated int
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			if u.TrialExpiration != nil {
				updated++
			}
		}

		require.Equal(t, maxUpdates, updated)

		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			if !u.PaidTier {
				require.NotNil(t, u.TrialExpiration)
			}
		}
	}

	test(t, prepare, check, trialExp, notifyCount, &migrator.Config{
		Limit:      1000,
		Verbose:    true,
		MaxUpdates: 1,
	})
}

// Test dry run results in no updates.
func TestMigrateUsersDryRun(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	trialExp := time.Now()
	notifyCount := 3

	check := func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User) {
		require.True(t, len(wouldBeUpdated) > 0)
		require.True(t, len(wouldNotBeUpdated) > 0)
		// Would have been updated, had we not set the dry run option. We are asserting that the fields that would have
		// been updated were not.
		for _, user := range wouldBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			require.Nil(t, u.TrialExpiration)
			require.Zero(t, u.TrialNotifications)
		}
		for _, user := range wouldNotBeUpdated {
			u, err := db.Console().Users().Get(ctx, user.ID)
			require.NoError(t, err)
			if !u.PaidTier {
				require.NotNil(t, u.TrialExpiration)
			}
		}
	}

	test(t, prepare, check, trialExp, notifyCount, &migrator.Config{
		Limit:   1000,
		DryRun:  true,
		Verbose: true,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, db satellite.DB) (wouldBeUpdated, wouldNotBeUpdated []console.User),
	check func(t *testing.T, ctx context.Context, db satellite.DB, wouldBeUpdated, wouldNotBeUpdated []console.User), trialExp time.Time, notifyCount int, config *migrator.Config) {

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

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB, "migrate-free-trial")
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.Testing().TestMigrateToLatest(ctx)
			require.NoError(t, err)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			wouldBeUpdated, wouldNotBeUpdated := prepare(t, ctx, db)

			if config.MaxUpdates > 0 {
				err = migrator.MigrateLimited(ctx, log, conn, trialExp, notifyCount, *config)
				require.NoError(t, err)
			} else {
				err = migrator.Migrate(ctx, log, conn, trialExp, notifyCount, *config)
				require.NoError(t, err)
			}

			require.NoError(t, err)

			check(t, ctx, db, wouldBeUpdated, wouldNotBeUpdated)
		})
	}
}
