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

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/private/dbutil"
	"storj.io/private/dbutil/tempdb"
	migrator "storj.io/storj/cmd/tools/nullify-bad-user-agents"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

// Test no entries in table doesn't error.
func TestMigrateUsersSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateUsers, check, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateUsersLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateUsersLimited, check, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateUsersUpdateNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	userAgent := []byte("teststorj")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {

		_, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			UserAgent:    userAgent,
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, userAgent, users[0].UserAgent)
	}
	test(t, prepare, migrator.MigrateUsers, check, &migrator.Config{
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

	userID := testrand.UUID()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		_, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           userID,
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			PartnerID:    userID,
			UserAgent:    userID.Bytes(),
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		user, err := db.Console().Users().Get(ctx, userID)
		require.NoError(t, err)
		require.Nil(t, user.UserAgent)
	}
	test(t, prepare, migrator.MigrateUsers, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateUsers(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var n int
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert with user_agent = partner_id
		id := testrand.UUID()
		_, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           id,
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			PartnerID:    id,
			UserAgent:    id.Bytes(),
		})
		require.NoError(t, err)
		n++
		// insert with user_agent = partner_id
		id = testrand.UUID()
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           id,
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			PartnerID:    id,
			UserAgent:    id.Bytes(),
		})
		require.NoError(t, err)
		n++
		// insert an entry with something not matching
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			UserAgent:    []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)

		var updated int
		for _, u := range users {
			if u.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, n, updated)
		n = 0
	}

	test(t, prepare, migrator.MigrateUsers, check, &migrator.Config{
		Limit: 1,
	})
}

// Test limited number of user_agent fields are updated correctly.
func TestMigrateUsersLimited(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert an entry with valid user agent
		_, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			UserAgent:    []byte("teststorj"),
		})
		require.NoError(t, err)

		// insert matching user_agent and partner id
		id := testrand.UUID()
		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			FullName:     "Test Test",
			PasswordHash: []byte{0, 1, 2, 3},
			PartnerID:    id,
			UserAgent:    id.Bytes(),
		})
		require.NoError(t, err)

		// insert '\x00000000000000000000000000000000' user_agent
		id = testrand.UUID()
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   id,
			OwnerID:     testrand.UUID(),
			UserAgent:   id.Bytes(),
		})
		require.NoError(t, err)
	}

	maxUpdates := 1

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		_, users, err := db.Console().Users().GetByEmailWithUnverified(ctx, "test@storj.test")
		require.NoError(t, err)

		var updated int
		for _, u := range users {
			if u.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, maxUpdates, updated)
	}
	test(t, prepare, migrator.MigrateUsersLimited, check, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateProjectsSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateProjectsLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateProjectsLimited, check, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateProjectsUpdateNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var id uuid.UUID
	userAgent := []byte("teststorj")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {

		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			UserAgent:   userAgent,
		})
		require.NoError(t, err)
		id = proj.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		proj, err := db.Console().Projects().Get(ctx, id)
		require.NoError(t, err)
		require.Equal(t, userAgent, proj.UserAgent)
	}
	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
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

	var projID uuid.UUID
	id := testrand.UUID()
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		prj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   id,
			OwnerID:     testrand.UUID(),
			UserAgent:   id.Bytes(),
		})
		require.NoError(t, err)
		projID = prj.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		proj, err := db.Console().Projects().Get(ctx, projID)
		require.NoError(t, err)
		require.Nil(t, proj.UserAgent)
	}
	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateProjects(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var n int
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert matching user_agent partner_id
		id := testrand.UUID()
		_, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			PartnerID:   id,
			UserAgent:   id.Bytes(),
		})
		require.NoError(t, err)
		n++
		// insert matching user_agent
		id = testrand.UUID()
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test1",
			Description: "test1",
			OwnerID:     testrand.UUID(),
			PartnerID:   id,
			UserAgent:   id.Bytes(),
		})
		require.NoError(t, err)
		n++
		// insert an entry with something not zero
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   testrand.UUID(),
			OwnerID:     testrand.UUID(),
			UserAgent:   []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		var updated int
		for _, prj := range projects {
			if prj.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, n, updated)
		n = 0
	}

	test(t, prepare, migrator.MigrateProjects, check, &migrator.Config{
		Limit: 1,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateProjectsLimited(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert an entry with valid user agent
		_, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			UserAgent:   []byte("teststorj"),
		})
		require.NoError(t, err)

		// insert matching user_agent
		id := testrand.UUID()
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			UserAgent:   id.Bytes(),
			PartnerID:   id,
		})
		require.NoError(t, err)

		// insert matching user_agent and partner id
		id = testrand.UUID()
		_, err = db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			UserAgent:   id.Bytes(),
			PartnerID:   id,
		})
		require.NoError(t, err)
	}

	maxUpdates := 1

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		projects, err := db.Console().Projects().GetAll(ctx)
		require.NoError(t, err)

		var updated int
		for _, prj := range projects {
			if prj.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, maxUpdates, updated)
	}
	test(t, prepare, migrator.MigrateProjectsLimited, check, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateAPIKeysSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateAPIKeys, check, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateAPIKeysLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateAPIKeysLimited, check, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateAPIKeysUpdateNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var testID uuid.UUID
	id := testrand.UUID()
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		apikey, err := db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: proj.ID,
			Name:      "test0",
			Secret:    []byte("test"),
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		testID = apikey.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		apikey, err := db.Console().APIKeys().Get(ctx, testID)
		require.NoError(t, err)
		require.Nil(t, apikey.UserAgent)
	}
	test(t, prepare, migrator.MigrateAPIKeys, check, &migrator.Config{
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

	var testID uuid.UUID

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		prj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			PartnerID:   testrand.UUID(),
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		id := testrand.UUID()
		apiKey, err := db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: prj.ID,
			PartnerID: id,
			Name:      "test0",
			Secret:    []byte("test"),
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		testID = apiKey.ID
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		apiKey, err := db.Console().APIKeys().Get(ctx, testID)
		require.NoError(t, err)
		require.Nil(t, apiKey.UserAgent)
	}
	test(t, prepare, migrator.MigrateAPIKeys, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateAPIKeys(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

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

		// insert matching user_agent and partner id
		id := testrand.UUID()
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test0",
			Secret:    []byte("test"),
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert another matching user_agent and partner id
		id = testrand.UUID()
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test1",
			Secret:    []byte("test1"),
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert an entry with something not zero
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			PartnerID: testrand.UUID(),
			Name:      "test2",
			Secret:    []byte("test"),
			UserAgent: []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		keyPage, err := db.Console().APIKeys().GetPagedByProjectID(ctx, projID, console.APIKeyCursor{Page: 1, Limit: 1000})
		require.NoError(t, err)

		var updated int
		for _, key := range keyPage.APIKeys {
			if key.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, n, updated)
		n = 0
	}

	test(t, prepare, migrator.MigrateAPIKeys, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateAPIKeysLimited(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var projID uuid.UUID

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert an entry with valid user agent
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test0",
			Secret:    []byte("test"),
			UserAgent: []byte("teststorj"),
		})
		require.NoError(t, err)

		// insert matching user_agent and partner id
		id := testrand.UUID()
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			Name:      "test1",
			Secret:    []byte("test"),
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)

		// insert another matching user_agent and partner id
		id = testrand.UUID()
		_, err = db.Console().APIKeys().Create(ctx, testrand.UUID().Bytes(), console.APIKeyInfo{
			ProjectID: projID,
			PartnerID: id,
			Name:      "test2",
			Secret:    []byte("test"),
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
	}

	maxUpdates := 1

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		keyPage, err := db.Console().APIKeys().GetPagedByProjectID(ctx, projID, console.APIKeyCursor{Page: 1, Limit: 1000})
		require.NoError(t, err)

		var updated int
		for _, key := range keyPage.APIKeys {
			if key.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, maxUpdates, updated)

	}
	test(t, prepare, migrator.MigrateAPIKeysLimited, check, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateBucketMetainfosSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateBucketMetainfosLimitedSelectNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateBucketMetainfosLimited, check, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateBucketMetainfosUpdateNoRows(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

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

		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test1",
			ProjectID: projID,
			UserAgent: []byte("teststorj"),
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		b, err := db.Buckets().GetBucket(ctx, []byte(bName), projID)
		require.NoError(t, err)
		require.NotNil(t, b.UserAgent)
	}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &migrator.Config{
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

		id := testrand.UUID()
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      string(bucket),
			ProjectID: projID,
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		b, err := db.Buckets().GetBucket(ctx, bucket, projID)
		require.NoError(t, err)
		require.Nil(t, b.UserAgent)
	}
	test(t, prepare, migrator.MigrateBucketMetainfos, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateBucketMetainfos(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	var n int
	var projID uuid.UUID
	zeroedUUID := uuid.UUID{}.Bytes()
	require.NotNil(t, zeroedUUID)
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
		})
		require.NoError(t, err)
		projID = proj.ID

		// insert matching user_agent and partner id
		id := testrand.UUID()
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        id,
			Name:      "test0",
			ProjectID: projID,
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert another matching user_agent and partner id
		id = testrand.UUID()
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        id,
			Name:      "test1",
			ProjectID: projID,
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert an entry with something not zero
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test2",
			ProjectID: projID,
			PartnerID: testrand.UUID(),
			UserAgent: []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		list, err := db.Buckets().ListBuckets(ctx, projID, storj.BucketListOptions{Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
		require.NoError(t, err)

		var updated int
		for _, b := range list.Items {
			if b.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, n, updated)
		n = 0
	}

	test(t, prepare, migrator.MigrateBucketMetainfos, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateBucketMetainfosLimited(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testID := testrand.UUID()
	zeroedUUID := uuid.UUID{}.Bytes()
	require.NotNil(t, zeroedUUID)
	var projID uuid.UUID
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		proj, err := db.Console().Projects().Insert(ctx, &console.Project{
			Name:        "test",
			Description: "test",
			OwnerID:     testrand.UUID(),
			UserAgent:   []byte("teststorj"),
		})
		require.NoError(t, err)

		projID = proj.ID

		// insert matching user_agent
		id := testrand.UUID()
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testID,
			Name:      "test0",
			ProjectID: projID,
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)

		// insert another matching user_agent and partner id
		id = testrand.UUID()
		_, err = db.Buckets().CreateBucket(ctx, storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "test1",
			ProjectID: projID,
			PartnerID: id,
			UserAgent: id.Bytes(),
		})
		require.NoError(t, err)
	}

	maxUpdates := 1

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		list, err := db.Buckets().ListBuckets(ctx, projID, storj.BucketListOptions{Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
		require.NoError(t, err)

		var updated int
		for _, b := range list.Items {
			if b.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, maxUpdates, updated)
	}
	test(t, prepare, migrator.MigrateBucketMetainfosLimited, check, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

// Test no entries in table doesn't error.
func TestMigrateValueAttributionsSelectNoRows(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateValueAttributions, check, &migrator.Config{
		Limit: 8,
	})
}

// Test no entries in table doesn't error.
func TestMigrateValueAttributionsLimitedSelectNoRows(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {}
	test(t, prepare, migrator.MigrateValueAttributionsLimited, check, &migrator.Config{
		MaxUpdates: 1,
	})
}

// Test no rows to update returns no error.
func TestMigrateValueAttributionsUpdateNoRows(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	partnerID := testrand.UUID()
	ua := []byte("test")
	projID := testrand.UUID()
	bName := []byte("test")
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {

		_, err := db.Attribution().Insert(ctx, &attribution.Info{
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
	test(t, prepare, migrator.MigrateValueAttributions, check, &migrator.Config{
		Limit: 8,
	})
}

// Test select offset beyond final row.
// With only one row, selecting with an offset of 1 will return 0 rows.
// Test that this is accounted for and updates the row correctly.
func TestMigrateValueAttributionsSelectOffsetBeyondRowCount(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	projID := testrand.UUID()
	bucket := []byte("test")
	id := testrand.UUID()
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		_, err := db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  projID,
			PartnerID:  id,
			BucketName: bucket,
			UserAgent:  id.Bytes(),
		})
		require.NoError(t, err)
	}
	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		att, err := db.Attribution().Get(ctx, projID, bucket)
		require.NoError(t, err)
		require.Nil(t, att.UserAgent)
	}
	test(t, prepare, migrator.MigrateValueAttributions, check, &migrator.Config{
		Limit: 8,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateValueAttributions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	type info struct {
		bucket  []byte
		project uuid.UUID
	}

	var n int
	zeroedUUID := uuid.UUID{}.Bytes()
	require.NotNil(t, zeroedUUID)
	var infos []info
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {

		// insert matching user_agent partner id
		id := testrand.UUID()
		b := []byte("test0")
		infos = append(infos, info{b, id})
		_, err := db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert another matching user_agent partner id
		id = testrand.UUID()
		infos = append(infos, info{b, id})
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  id.Bytes(),
		})
		require.NoError(t, err)
		n++

		// insert without zeroes
		id = testrand.UUID()
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		var updated int
		for _, in := range infos {
			att, err := db.Attribution().Get(ctx, in.project, in.bucket)
			require.NoError(t, err)
			if att.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, n, updated)
		n = 0
		// clear infos for the subsequent CRDB test
		infos = []info{}
	}

	test(t, prepare, migrator.MigrateValueAttributions, check, &migrator.Config{
		Limit: 1,
	})
}

// Test user_agent field is updated correctly.
func TestMigrateValueAttributionsLimited(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	type info struct {
		bucket  []byte
		project uuid.UUID
	}

	zeroedUUID := uuid.UUID{}.Bytes()
	require.NotNil(t, zeroedUUID)
	var infos []info
	prepare := func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB) {
		// insert with matching user agent and partner id
		id := testrand.UUID()
		b := []byte("test0")
		infos = append(infos, info{b, id})
		_, err := db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  id.Bytes(),
		})
		require.NoError(t, err)

		// insert another with zeroes
		id = testrand.UUID()
		infos = append(infos, info{b, id})
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  id.Bytes(),
		})
		require.NoError(t, err)

		// insert without zeroes
		id = testrand.UUID()
		infos = append(infos, info{b, id})
		_, err = db.Attribution().Insert(ctx, &attribution.Info{
			ProjectID:  id,
			PartnerID:  id,
			BucketName: b,
			UserAgent:  []byte("teststorj"),
		})
		require.NoError(t, err)
	}

	maxUpdates := 1

	check := func(t *testing.T, ctx context.Context, db satellite.DB) {
		var updated int
		for _, in := range infos {
			att, err := db.Attribution().Get(ctx, in.project, in.bucket)
			require.NoError(t, err)
			if att.UserAgent == nil {
				updated++
			}
		}
		require.Equal(t, maxUpdates, updated)

		// clear infos for the subsequent CRDB test
		infos = []info{}
	}
	test(t, prepare, migrator.MigrateValueAttributionsLimited, check, &migrator.Config{
		MaxUpdates: maxUpdates,
	})
}

func test(t *testing.T, prepare func(t *testing.T, ctx *testcontext.Context, rawDB *dbutil.TempDatabase, db satellite.DB),
	migrate func(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config migrator.Config) (err error),
	check func(t *testing.T, ctx context.Context, db satellite.DB), config *migrator.Config) {

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

			err = migrate(ctx, log, conn, *config)
			require.NoError(t, err)

			require.NoError(t, err)

			check(t, ctx, db)
		})
	}
}
