// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pgx "github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/partnerid-to-useragent-migration"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/rewards"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test no entries in table doesn't error.
func TestMigrateUsersSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateUsers, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateUsersLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateUsersLimited, check, &p, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateUsersUpdateNoRows(t *testing.T) {
	t.Parallel()
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
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Nil(t, users[0].UserAgent)
	}
	test(t, prepare, migrator.MigrateUsers, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateUsersSelectOffsetBeyondRowCount(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateUsers, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateUsers(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateUsers, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test limited number of user_agent fields are updated correctly.
func TestMigrateUsersLimited(t *testing.T) {
	t.Parallel()
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

	maxUpdates := 10

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)
		require.Len(t, users, n)

		var updated int
		for _, u := range users {
			var expectedUA []byte
			if u.PartnerID.IsZero() {
				require.Nil(t, u.UserAgent)
				continue
			}

			// only a limited number should be updated.
			if u.UserAgent == nil {
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

			updated++
			require.Equal(t, expectedUA, u.UserAgent)
		}
		require.Equal(t, maxUpdates, updated)

		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateUsersLimited, check, &p, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateProjectsSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateProjects, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateProjectsLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateProjectsLimited, check, &p, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateProjectsUpdateNoRows(t *testing.T) {
	t.Parallel()
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
	var id uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert an entry with no partner ID
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		id = proj.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		proj, err := db.Console().Projects().Get(ctx, id)
		require.NoError(t, err)
		require.Nil(t, proj.UserAgent)
	}
	test(t, prepare, migrator.MigrateProjects, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateProjectsSelectOffsetBeyondRowCount(t *testing.T) {
	t.Parallel()
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
	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		prj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   p.UUIDs[0],
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		projID = prj.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		proj, err := db.Console().Projects().Get(ctx, projID)
		require.NoError(t, err)
		require.Equal(t, p.Names[0], proj.UserAgent)
	}
	test(t, prepare, migrator.MigrateProjects, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateProjects(t *testing.T) {
	t.Parallel()
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
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   testrand.UUID(),
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		for _, p := range partnerInfo {
			id := testrand.UUID()

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Console.Projects API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}
			_, err = db.Console().Projects().Insert(ctx, &console.Project{
				Name:        "test",
				Description: "test",
				PartnerID:   p.UUID,
				OwnerID:     id,
			})
			require.NoError(t, err)
			n++
		}
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		require.Len(t, projects, n)
		for _, prj := range projects {
			if prj.PartnerID.IsZero() {
				require.Nil(t, prj.UserAgent)
				continue
			}
			var expectedUA []byte
			for _, p := range partnerInfo {
				if prj.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = prj.PartnerID.Bytes()
			}
			require.Equal(t, expectedUA, prj.UserAgent)
		}
		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateProjects, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateProjectsLimited(t *testing.T) {
	t.Parallel()
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
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   testrand.UUID(),
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		for _, p := range partnerInfo {
			id := testrand.UUID()

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Console.Projects API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}
			_, err = db.Console().Projects().Insert(ctx, &console.Project{
				Name:        "test",
				Description: "test",
				PartnerID:   p.UUID,
				OwnerID:     id,
			})
			require.NoError(t, err)
			n++
		}
	}

	maxUpdates := 10

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)
		require.Len(t, projects, n)

		var updated int
		for _, prj := range projects {
			if prj.PartnerID.IsZero() {
				require.Nil(t, prj.UserAgent)
				continue
			}

			// only a limited number should be updated.
			if prj.UserAgent == nil {
				continue
			}

			var expectedUA []byte
			for _, p := range partnerInfo {
				if prj.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = prj.PartnerID.Bytes()
			}

			updated++
			require.Equal(t, expectedUA, prj.UserAgent)
		}
		require.Equal(t, maxUpdates, updated)

		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateProjectsLimited, check, &p, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateAPIKeysSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateAPIKeys, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateAPIKeysLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateAPIKeysLimited, check, &p, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateAPIKeysUpdateNoRows(t *testing.T) {
	t.Parallel()
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

	var id uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		// insert an entry with no partner ID
		apikey, err := db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: proj.ID,
			Name:      "test0",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		id = apikey.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		apikey, err := db.Console().APIKeys().Get(ctx, id)
		require.NoError(t, err)
		require.Nil(t, apikey.UserAgent)
	}
	test(t, prepare, migrator.MigrateAPIKeys, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateAPIKeysSelectOffsetBeyondRowCount(t *testing.T) {
	t.Parallel()
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
	var apiKeyID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		prj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   p.UUIDs[0],
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		apiKey, err := db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: prj.ID,
			PartnerID: prj.PartnerID,
			Name:      "test0",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		apiKeyID = apiKey.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		apiKey, err := db.Console().APIKeys().Get(ctx, apiKeyID)
		require.NoError(t, err)
		require.Equal(t, p.Names[0], apiKey.UserAgent)
	}
	test(t, prepare, migrator.MigrateAPIKeys, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateAPIKeys(t *testing.T) {
	t.Parallel()
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

	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with no partner ID
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test0",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			PartnerID: testrand.UUID(),
			Name:      "test1",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		n++

		for i, p := range partnerInfo {

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Console.APIKeys API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
				ProjectID: projID,
				PartnerID: p.UUID,
				Name:      fmt.Sprint(i),
				Secret:    []byte("test"),
			})
			require.NoError(t, err)
			n++
		}
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		keyPage, err := db.Console().APIKeys().GetPagedByProjectID(ctx, projID, console.APIKeyCursor{Page: 1, Limit: 1000})
		require.NoError(t, err)

		require.Len(t, keyPage.APIKeys, n)
		for _, key := range keyPage.APIKeys {
			if key.PartnerID.IsZero() {
				require.Nil(t, key.UserAgent)
				continue
			}
			var expectedUA []byte
			for _, p := range partnerInfo {
				if key.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = key.PartnerID.Bytes()
			}
			require.Equal(t, expectedUA, key.UserAgent)
		}
		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateAPIKeys, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateAPIKeysLimited(t *testing.T) {
	t.Parallel()
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

	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with no partner ID
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test0",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			PartnerID: testrand.UUID(),
			Name:      "test1",
			Secret:    []byte("test"),
		})
		require.NoError(t, err)
		n++

		for i, p := range partnerInfo {

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Console.APIKeys API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
				ProjectID: projID,
				PartnerID: p.UUID,
				Name:      fmt.Sprint(i),
				Secret:    []byte("test"),
			})
			require.NoError(t, err)
			n++
		}
	}

	maxUpdates := 10

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		keyPage, err := db.Console().APIKeys().GetPagedByProjectID(ctx, projID, console.APIKeyCursor{Page: 1, Limit: 1000})
		require.NoError(t, err)
		require.Len(t, keyPage.APIKeys, n)

		var updated int
		for _, key := range keyPage.APIKeys {
			if key.PartnerID.IsZero() {
				require.Nil(t, key.UserAgent)
				continue
			}

			// only a limited number should be updated.
			if key.UserAgent == nil {
				continue
			}

			var expectedUA []byte
			for _, p := range partnerInfo {
				if key.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = key.PartnerID.Bytes()
			}

			updated++
			require.Equal(t, expectedUA, key.UserAgent)
		}
		require.Equal(t, maxUpdates, updated)

		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateAPIKeysLimited, check, &p, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateBucketMetainfosSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateBucketMetainfosLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
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
	test(t, prepare, migrator.MigrateBucketMetainfosLimited, check, &p, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateBucketMetainfosUpdateNoRows(t *testing.T) {
	t.Parallel()
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

	bName := "test1"
	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with no partner ID
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test1",
			ProjectID: projID,
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		b, err := db.Buckets().GetBucket(ctx, []byte(bName), projID)
		require.NoError(t, err)
		require.Nil(t, b.UserAgent)
	}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateBucketMetainfosSelectOffsetBeyondRowCount(t *testing.T) {
	t.Parallel()
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
	var projID uuid.UUID
	bucket := []byte("test")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		prj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		projID = prj.ID

		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      string(bucket),
			ProjectID: projID,
			PartnerID: p.UUIDs[0],
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		b, err := db.Buckets().GetBucket(ctx, bucket, projID)
		require.NoError(t, err)
		require.Equal(t, p.Names[0], b.UserAgent)
	}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateBucketMetainfos(t *testing.T) {
	t.Parallel()
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

	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with no partner ID
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test0",
			ProjectID: projID,
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test1",
			ProjectID: projID,
			PartnerID: testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		for i, p := range partnerInfo {
			id, err := uuid.New()
			require.NoError(t, err)

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Buckets API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
				ID:        id,
				Name:      fmt.Sprint(i),
				ProjectID: projID,
				PartnerID: p.UUID,
			})
			require.NoError(t, err)
			n++
		}
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		list, err := db.Buckets().ListBuckets(ctx, projID, storj.BucketListOptions{Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
		require.NoError(t, err)

		require.Len(t, list.Items, n)
		for _, b := range list.Items {
			if b.PartnerID.IsZero() {
				require.Nil(t, b.UserAgent)
				continue
			}
			var expectedUA []byte
			for _, p := range partnerInfo {
				if b.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = b.PartnerID.Bytes()
			}
			require.Equal(t, expectedUA, b.UserAgent)
		}
		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateBucketMetainfosLimited(t *testing.T) {
	t.Parallel()
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

	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with no partner ID
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test0",
			ProjectID: projID,
		})
		require.NoError(t, err)
		n++

		// insert an entry with a partner ID which does not exist in the partnersDB
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test1",
			ProjectID: projID,
			PartnerID: testrand.UUID(),
		})
		require.NoError(t, err)
		n++

		for i, p := range partnerInfo {
			id, err := uuid.New()
			require.NoError(t, err)

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Buckets API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
				ID:        id,
				Name:      fmt.Sprint(i),
				ProjectID: projID,
				PartnerID: p.UUID,
			})
			require.NoError(t, err)
			n++
		}
	}

	maxUpdates := 10

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		list, err := db.Buckets().ListBuckets(ctx, projID, storj.BucketListOptions{Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
		require.NoError(t, err)
		require.Len(t, list.Items, n)

		var updated int
		for _, b := range list.Items {
			if b.PartnerID.IsZero() {
				require.Nil(t, b.UserAgent)
				continue
			}

			// only a limited number should be updated.
			if b.UserAgent == nil {
				continue
			}

			var expectedUA []byte
			for _, p := range partnerInfo {
				if b.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = b.PartnerID.Bytes()
			}

			updated++
			require.Equal(t, expectedUA, b.UserAgent)
		}
		require.Equal(t, maxUpdates, updated)

		// reset n for the subsequent CRDB test
		n = 0
	}
	test(t, prepare, migrator.MigrateBucketMetainfosLimited, check, &p, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateValueAttributionsSelectNoRows(t *testing.T) {
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
	test(t, prepare, migrator.MigrateValueAttributions, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateValueAttributionsLimitedSelectNoRows(t *testing.T) {
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
	test(t, prepare, migrator.MigrateValueAttributionsLimited, check, &p, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateValueAttributionsUpdateNoRows(t *testing.T) {
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

	// For value_attributions, partner_id is not nullable. The attributions API will insert a uuid
	// full of zeros if one is not specified. Thus, to test that a row is not updated, we can't do so
	// by leaving partner_id empty. We must do so by setting both partner_id and user_agent, since the migration
	// updates where partner_id is not null and user_agent is null.
	partnerID := testrand.UUID()
	ua := []byte("test")
	projID := testrand.UUID()
	bName := []byte("test")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {

		// insert an entry with user_agent column set
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projID,
			PartnerID:  partnerID,
			BucketName: bName,
			UserAgent:  ua,
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		att, err := db.Attribution().Get(ctx, projID, bName)
		require.NoError(t, err)
		require.Equal(t, partnerID, att.PartnerID)
		require.Equal(t, ua, att.UserAgent)
	}
	test(t, prepare, migrator.MigrateValueAttributions, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateValueAttributionsSelectOffsetBeyondRowCount(t *testing.T) {
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
	projID := testrand.UUID()
	bucket := []byte("test")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projID,
			PartnerID:  p.UUIDs[0],
			BucketName: bucket,
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		att, err := db.Attribution().Get(ctx, projID, bucket)
		require.NoError(t, err)
		require.Equal(t, p.Names[0], att.UserAgent)
	}
	test(t, prepare, migrator.MigrateValueAttributions, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateValueAttributions(t *testing.T) {
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

	type info struct {
		bucket  []byte
		project uuid.UUID
	}

	var infos []info
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// The partner_id field of value_attributions is not nullable.
		// However, if no partner ID is passed to the Insert method it does not return an error.
		// It will insert the empty UUID byte array into partner_id.
		// This is the same as the empty UUID byte array of the Kafka partner in the partnerDB, as the Kafka entry UUID
		// field is not populated.
		// Thus, if I insert an entry with no partner_id for the test, the migration will find that the partner_id matches
		// Kafka's UUID and insert 'Kafka' into the user_agent column.
		// This is not good.
		// However this may not be a practical issue, as there seem to be checks before Insert is called to make
		// sure either UserAgent or PartnerID are populated. In the migration, if user_agent is not null, the
		// row will not be updated.

		for i, p := range partnerInfo {

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Attribution API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			projID := testrand.UUID()

			bucket := []byte(fmt.Sprint(i))

			in := info{bucket, projID}
			infos = append(infos, in)

			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  in.project,
				BucketName: in.bucket,
				PartnerID:  p.UUID,
			})
			require.NoError(t, err)
		}

		// insert an entry with a partner ID which does not exist in the partnersDB
		id := testrand.UUID()
		b := []byte("test0")
		infos = append(infos, info{b, id})
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		for _, in := range infos {
			att, err := db.Attribution().Get(ctx, in.project, in.bucket)
			require.NoError(t, err)
			if att.PartnerID.IsZero() {
				require.Nil(t, att.UserAgent)
				continue
			}
			var expectedUA []byte
			for _, p := range partnerInfo {
				if att.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = att.PartnerID.Bytes()
			}
			require.Equal(t, expectedUA, att.UserAgent)
		}
		// clear infos for the subsequent CRDB test
		infos = []info{}
	}
	test(t, prepare, migrator.MigrateValueAttributions, check, &p, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateValueAttributionsLimited(t *testing.T) {
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

	type info struct {
		bucket  []byte
		project uuid.UUID
	}

	var infos []info
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// The partner_id field of value_attributions is not nullable.
		// However, if no partner ID is passed to the Insert method it does not return an error.
		// It will insert the empty UUID byte array into partner_id.
		// This is the same as the empty UUID byte array of the Kafka partner in the partnerDB, as the Kafka entry UUID
		// field is not populated.
		// Thus, if I insert an entry with no partner_id for the test, the migration will find that the partner_id matches
		// Kafka's UUID and insert 'Kafka' into the user_agent column.
		// This is not good.
		// However this may not be a practical issue, as there seem to be checks before Insert is called to make
		// sure either UserAgent or PartnerID are populated. In the migration, if user_agent is not null, the
		// row will not be updated.

		for i, p := range partnerInfo {

			// The partner Kafka has no UUID and its ID is too short to convert to a UUID.
			// The Attribution API expects a UUID for inserting and getting.
			// Even if we insert its ID, OSPP005, directly into the DB, attempting to
			// retrieve the entry from the DB would result in an error when it tries to
			// convert the PartnerID bytes to a UUID.
			if p.UUID.IsZero() {
				continue
			}

			projID := testrand.UUID()

			bucket := []byte(fmt.Sprint(i))

			in := info{bucket, projID}
			infos = append(infos, in)

			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  in.project,
				BucketName: in.bucket,
				PartnerID:  p.UUID,
			})
			require.NoError(t, err)
		}

		// insert an entry with a partner ID which does not exist in the partnersDB
		id := testrand.UUID()
		b := []byte("test0")
		infos = append(infos, info{b, id})
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
		})
		require.NoError(t, err)
	}

	maxUpdates := 10

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		var updated int
		for _, in := range infos {
			att, err := db.Attribution().Get(ctx, in.project, in.bucket)
			require.NoError(t, err)
			if att.PartnerID.IsZero() {
				require.Nil(t, att.UserAgent)
				continue
			}

			// only a limited number should be updated.
			if att.UserAgent == nil {
				continue
			}

			var expectedUA []byte
			for _, p := range partnerInfo {
				if att.PartnerID == p.UUID {
					expectedUA = []byte(p.Name)
					break
				}
			}
			if expectedUA == nil {
				expectedUA = att.PartnerID.Bytes()
			}

			updated++
			require.Equal(t, expectedUA, att.UserAgent)
		}
		require.Equal(t, maxUpdates, updated)

		// clear infos for the subsequent CRDB test
		infos = []info{}
	}
	test(t, prepare, migrator.MigrateValueAttributionsLimited, check, &p, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB),
	migrate func(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *migrator.Partners, config migrator.Config) (err error),
	check func(t *testing.T, ctx context.Context, db satellite.DB), p *migrator.Partners, config *migrator.Config) {

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

			err = db.TestingMigrateToLatest(ctx)
			require.NoError(t, err)

			prepare(t, ctx, tempDB, db)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			err = migrate(ctx, log, conn, p, *config)
			require.NoError(t, err)

			require.NoError(t, err)

			check(t, ctx, db)
		})
	}
}
