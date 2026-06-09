// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	main "storj.io/storj/cmd/tools/om-license"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

const omLicenseType = "OM"

func TestGrantOMLicenseToAllActiveUsers(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		eligibleA := insertActiveUser(ctx, t, db, "eligible-a@example.com")
		eligibleB := insertActiveUser(ctx, t, db, "eligible-b@example.com")
		hasOM := insertActiveUser(ctx, t, db, "already-has-om@example.com")
		inactive := insertInactiveUser(ctx, t, db, "inactive@example.com")

		svc := entitlements.NewService(log, db.Console().Entitlements())
		require.NoError(t, svc.Licenses().Set(ctx, hasOM.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ExpiresAt: time.Now().Add(72 * time.Hour), Key: []byte("pre-existing")},
			},
		}))

		cfg := makeCfg(t, "")
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		for _, u := range []*console.User{eligibleA, eligibleB} {
			active, err := svc.Licenses().GetActive(ctx, u.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
			require.NoError(t, err, u.Email)
			require.Len(t, active, 1, u.Email)
			require.Equal(t, uint(0), active[0].ProductID, u.Email)
			require.Equal(t, 1, active[0].Count, u.Email)
		}

		existing, err := svc.Licenses().Get(ctx, hasOM.ID)
		require.NoError(t, err)
		require.Len(t, existing.Licenses, 1)
		require.Equal(t, []byte("pre-existing"), existing.Licenses[0].Key)
		require.Equal(t, 1, existing.Licenses[0].Count, "legacy row without Count deserializes as Count=1")

		inactiveLicenses, err := svc.Licenses().Get(ctx, inactive.ID)
		require.NoError(t, err)
		require.Empty(t, inactiveLicenses.Licenses)

		// second run is a no-op: all eligible users now have an OM license at target count
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))
		for _, u := range []*console.User{eligibleA, eligibleB} {
			active, err := svc.Licenses().GetActive(ctx, u.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
			require.NoError(t, err, u.Email)
			require.Len(t, active, 1, u.Email)
			require.Equal(t, 1, active[0].Count, u.Email)
		}
	})
}

func TestGrantOMLicenseToAllActiveUsers_BumpsCountOnExisting(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		user := insertActiveUser(ctx, t, db, "bump@example.com")

		svc := entitlements.NewService(log, db.Console().Entitlements())
		originalExpires := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
		require.NoError(t, svc.Licenses().Set(ctx, user.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: originalExpires},
			},
		}))

		cfg := makeCfgWithCount(t, "", 2)
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		stored, err := svc.Licenses().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, stored.Licenses, 1, "should bump in place, not append")
		require.Equal(t, 2, stored.Licenses[0].Count)
		require.Equal(t, uint(0), stored.Licenses[0].ProductID)
		require.True(t, stored.Licenses[0].ExpiresAt.Equal(originalExpires), "ExpiresAt on bumped row must be preserved")
	})
}

func TestGrantOMLicenseToAllActiveUsers_SkipsAtOrAboveTarget(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		user := insertActiveUser(ctx, t, db, "skip-at-target@example.com")

		svc := entitlements.NewService(log, db.Console().Entitlements())
		originalExpires := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
		require.NoError(t, svc.Licenses().Set(ctx, user.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 5, ExpiresAt: originalExpires, Key: []byte("preserved")},
			},
		}))

		cfg := makeCfgWithCount(t, "", 2)
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		stored, err := svc.Licenses().Get(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, stored.Licenses, 1)
		require.Equal(t, 5, stored.Licenses[0].Count, "Count must not be reduced below existing value")
		require.Equal(t, []byte("preserved"), stored.Licenses[0].Key)
	})
}

func TestGrantOMLicenseToAllActiveUsers_LeavesExpiredOrRevokedUntouched(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		userExpired := insertActiveUser(ctx, t, db, "expired@example.com")
		userRevoked := insertActiveUser(ctx, t, db, "revoked@example.com")

		svc := entitlements.NewService(log, db.Console().Entitlements())
		expiredAt := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Second)
		revokedAt := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
		futureExpires := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)

		require.NoError(t, svc.Licenses().Set(ctx, userExpired.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 3, ExpiresAt: expiredAt, Key: []byte("old-expired")},
			},
		}))
		require.NoError(t, svc.Licenses().Set(ctx, userRevoked.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 3, ExpiresAt: futureExpires, RevokedAt: revokedAt, Key: []byte("old-revoked")},
			},
		}))

		cfg := makeCfgWithCount(t, "", 2)
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		expiredStored, err := svc.Licenses().Get(ctx, userExpired.ID)
		require.NoError(t, err)
		require.Len(t, expiredStored.Licenses, 2, "expired row should be left in place and a new active row appended")
		require.Equal(t, []byte("old-expired"), expiredStored.Licenses[0].Key)
		require.True(t, expiredStored.Licenses[0].ExpiresAt.Equal(expiredAt))
		require.Equal(t, 3, expiredStored.Licenses[0].Count)
		require.Equal(t, 2, expiredStored.Licenses[1].Count)
		require.Equal(t, uint(0), expiredStored.Licenses[1].ProductID)
		require.False(t, expiredStored.Licenses[1].ExpiresAt.IsZero())
		require.True(t, expiredStored.Licenses[1].ExpiresAt.After(time.Now()))

		revokedStored, err := svc.Licenses().Get(ctx, userRevoked.ID)
		require.NoError(t, err)
		require.Len(t, revokedStored.Licenses, 2)
		require.Equal(t, []byte("old-revoked"), revokedStored.Licenses[0].Key)
		require.True(t, revokedStored.Licenses[0].RevokedAt.Equal(revokedAt))
		require.Equal(t, 3, revokedStored.Licenses[0].Count)
		require.Equal(t, 2, revokedStored.Licenses[1].Count)
		require.True(t, revokedStored.Licenses[1].RevokedAt.IsZero())
	})
}

func TestGrantOMLicenseToAllActiveUsers_DryRunMakesNoChanges(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		fresh := insertActiveUser(ctx, t, db, "fresh@example.com")
		toBump := insertActiveUser(ctx, t, db, "to-bump@example.com")

		svc := entitlements.NewService(log, db.Console().Entitlements())
		originalExpires := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
		require.NoError(t, svc.Licenses().Set(ctx, toBump.ID, entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: originalExpires},
			},
		}))

		cfg := makeCfgWithCount(t, "", 2)
		cfg.DryRun = true
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		freshStored, err := svc.Licenses().Get(ctx, fresh.ID)
		require.NoError(t, err)
		require.Empty(t, freshStored.Licenses)

		bumpStored, err := svc.Licenses().Get(ctx, toBump.ID)
		require.NoError(t, err)
		require.Len(t, bumpStored.Licenses, 1)
		require.Equal(t, 1, bumpStored.Licenses[0].Count, "dry run must not bump Count")
	})
}

func TestGrantOMLicenseToAllActiveUsers_DryRun(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		user := insertActiveUser(ctx, t, db, "dryrun@example.com")

		cfg := makeCfg(t, "")
		cfg.DryRun = true

		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		svc := entitlements.NewService(log, db.Console().Entitlements())
		active, err := svc.Licenses().GetActive(ctx, user.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
		require.NoError(t, err)
		require.Empty(t, active)
	})
}

func TestGrantOMLicenseToAllActiveUsers_EmailFilter(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		log := zaptest.NewLogger(t)

		matched := insertActiveUser(ctx, t, db, "target@match.test")
		unmatched := insertActiveUser(ctx, t, db, "other@skip.test")

		cfg := makeCfg(t, "*@match.test")
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))

		svc := entitlements.NewService(log, db.Console().Entitlements())
		matchedLicenses, err := svc.Licenses().GetActive(ctx, matched.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
		require.NoError(t, err)
		require.Len(t, matchedLicenses, 1)

		unmatchedLicenses, err := svc.Licenses().GetActive(ctx, unmatched.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
		require.NoError(t, err)
		require.Empty(t, unmatchedLicenses)
	})
}

func insertActiveUser(ctx *testcontext.Context, t *testing.T, db satellite.DB, email string) *console.User {
	t.Helper()
	user := insertInactiveUser(ctx, t, db, email)
	active := console.Active
	require.NoError(t, db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &active}))
	return user
}

func insertInactiveUser(ctx *testcontext.Context, t *testing.T, db satellite.DB, email string) *console.User {
	t.Helper()
	user, err := db.Console().Users().Insert(ctx, &console.User{
		ID:           testrand.UUID(),
		Email:        email,
		PasswordHash: []byte("password"),
	})
	require.NoError(t, err)
	return user
}

func makeCfg(t *testing.T, emailPattern string) main.Config {
	t.Helper()
	return makeCfgWithCount(t, emailPattern, 1)
}

func makeCfgWithCount(t *testing.T, emailPattern string, count int) main.Config {
	t.Helper()
	cfg := main.Config{
		SatelliteDB:  "postgres://test",
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
		BatchSize:    100,
		Count:        count,
		SkipConfirm:  true,
		EmailPattern: emailPattern,
	}
	require.NoError(t, cfg.Verify())
	return cfg
}
