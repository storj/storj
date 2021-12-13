// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"strings"
	"testing"

	pgx "github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/partnerid-to-useragent-migration"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test no entries in table doesn't error.
func TestMigrateUsersSelectNoRows(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	partnerDB := rewards.DefaultPartnersDB
	partnerInfo, err := partnerDB.All(ctx)
	require.NoError(t, err)

	var p migrator.Partners
	for _, info := range partnerInfo {
		p.UUIDs = append(p.UUIDs, info.UUID)
		p.Names = append(p.Names, []byte(info.Name))
	}
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, 7, prepare, migrator.MigrateUsers, check, &p)
}

// Test no rows to update returns no error.
func TestMigrateUsersUpdateNoRows(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	partnerDB := rewards.DefaultPartnersDB
	partnerInfo, err := partnerDB.All(ctx)
	require.NoError(t, err)

	var p migrator.Partners
	for _, info := range partnerInfo {
		p.UUIDs = append(p.UUIDs, info.UUID)
		p.Names = append(p.Names, []byte(info.Name))
	}
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert an entry with no partner ID
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, 7, prepare, migrator.MigrateUsers, check, &p)
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateUsersSelectOffsetBeyondRowCount(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	partnerDB := rewards.DefaultPartnersDB
	partnerInfo, err := partnerDB.All(ctx)
	require.NoError(t, err)

	var p migrator.Partners
	for _, info := range partnerInfo {
		p.UUIDs = append(p.UUIDs, info.UUID)
		p.Names = append(p.Names, []byte(info.Name))
	}
	userID := testrand.UUID()
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           userID,
			PartnerID:    p.UUIDs[0],
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		user, err := db.Console().Users().Get(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, p.Names[0], user.UserAgent)
	}
	test(t, 7, prepare, migrator.MigrateUsers, check, &p)
}

// Test user_agent field is updated correctly.
func TestMigrateUsers(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	partnerDB := rewards.DefaultPartnersDB
	partnerInfo, err := partnerDB.All(ctx)
	require.NoError(t, err)

	var p migrator.Partners
	for _, info := range partnerInfo {
		p.UUIDs = append(p.UUIDs, info.UUID)
		p.Names = append(p.Names, []byte(info.Name))
	}

	var n int

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert an entry with no partner ID
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			PartnerID:    testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		for _, p := range partnerInfo {
			id := testrand.UUID()

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Console.Users API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}
			_, err = db.Console().Users().Insert(ctx, &console.User{
				ID:           id,
				Email:        "test@storj.test",
				FullName:     "Test Test",
				PasswordHash: []byte{0, 1, 2, 3},
				PartnerID:    p.UUID,
			})
			require.NoError(t, err)
			n++
		}
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)

		require.Len(t, users, n)
		for _, u := range users {
			var expectedUA []byte
			if u.PartnerID.IsZero() {
				require.Nil(t, u.UserAgent)
				continue
			}
			for _, p := range partnerInfo {
				if u.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = u.PartnerID.Bytes()
			}
			require.Equal(t, expectedUA, u.UserAgent)
		}
		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, 7, prepare, migrator.MigrateUsers, check, &p)
}

func test(t *testing.T, offset int, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB),
	migrate func(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *migrator.Partners, limit int) (err error),
	check func(t *testing.T, ctx context.Context, db satellite.DB), p *migrator.Partners) {

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

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB)
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.MigrateToLatest(ctx)
			require.NoError(t, err)

			prepare(t, ctx, tempDB, db)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			err = migrate(ctx, log, conn, p, offset)
			require.NoError(t, err)

			require.NoError(t, err)

			check(t, ctx, db)
		})
	}
}
