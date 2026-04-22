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
		}

		existing, err := svc.Licenses().Get(ctx, hasOM.ID)
		require.NoError(t, err)
		require.Len(t, existing.Licenses, 1)
		require.Equal(t, []byte("pre-existing"), existing.Licenses[0].Key)

		inactiveLicenses, err := svc.Licenses().Get(ctx, inactive.ID)
		require.NoError(t, err)
		require.Empty(t, inactiveLicenses.Licenses)

		// second run is a no-op: all eligible users now have an OM license
		require.NoError(t, main.GrantOMLicenseToAllActiveUsers(ctx, log, db, cfg))
		for _, u := range []*console.User{eligibleA, eligibleB} {
			active, err := svc.Licenses().GetActive(ctx, u.ID, entitlements.GetActiveOptions{LicenseType: omLicenseType})
			require.NoError(t, err, u.Email)
			require.Len(t, active, 1, u.Email)
		}
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
	cfg := main.Config{
		SatelliteDB:  "postgres://test",
		ExpiresAt:    time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
		BatchSize:    100,
		SkipConfirm:  true,
		EmailPattern: emailPattern,
	}
	require.NoError(t, cfg.Verify())
	return cfg
}
