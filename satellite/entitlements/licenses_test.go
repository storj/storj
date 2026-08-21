// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestLicenseEntitlements(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		entSvc := entitlements.NewService(zaptest.NewLogger(t), db.Console().Entitlements())
		licenses := entSvc.Licenses()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)
		require.NotNil(t, user)

		userID := user.ID

		// Getting licenses for a user with no entitlements should return an error.
		got, err := licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, got)

		now := time.Now().UTC().Truncate(time.Second)
		expectedPublicID := testrand.UUID().String()
		expectedKey := testrand.Bytes(32)
		expectedExpiresAt := now.Add(30 * 24 * time.Hour)

		licensesToSet := entitlements.AccountLicenses{
			Licenses: []entitlements.AccountLicense{
				{
					Type:       "pro",
					ProductID:  42,
					Count:      5,
					PublicID:   expectedPublicID,
					BucketName: "my-bucket",
					ExpiresAt:  expectedExpiresAt,
					Key:        expectedKey,
				},
			},
		}

		err = licenses.Set(ctx, userID, licensesToSet)
		require.NoError(t, err)

		// Get licenses should return what we set.
		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Len(t, got.Licenses, 1)
		require.Equal(t, entitlements.AccountLicense{
			Type:       "pro",
			ProductID:  42,
			Count:      5,
			PublicID:   expectedPublicID,
			BucketName: "my-bucket",
			ExpiresAt:  expectedExpiresAt,
			Key:        expectedKey,
		}, got.Licenses[0])

		// Update licenses with additional entries.
		licensesToSet.Licenses = append(licensesToSet.Licenses, entitlements.AccountLicense{
			Type:       "enterprise",
			Count:      1,
			BucketName: "another-bucket",
		})

		err = licenses.Set(ctx, userID, licensesToSet)
		require.NoError(t, err)

		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Len(t, got.Licenses, 2)
		require.Equal(t, "enterprise", got.Licenses[1].Type)
		require.Equal(t, "another-bucket", got.Licenses[1].BucketName)

		// Test with empty licenses list.
		err = licenses.Set(ctx, userID, entitlements.AccountLicenses{})
		require.NoError(t, err)

		got, err = licenses.Get(ctx, userID)
		require.NoError(t, err)
		require.Empty(t, got.Licenses)
	})
}

func TestAccountLicense_CountBackwardCompat(t *testing.T) {
	// Simulate JSON stored before the Count field was introduced (no "count" key).
	cases := []struct {
		name      string
		data      string
		wantCount int
	}{
		{"no count field", `{"type":"legacy"}`, 1},
		{"explicit count zero", `{"type":"legacy","count":0}`, 1},
		{"explicit count set", `{"type":"legacy","count":3}`, 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var license entitlements.AccountLicense
			require.NoError(t, json.Unmarshal([]byte(tc.data), &license))
			require.Equal(t, tc.wantCount, license.Count)
		})
	}
}

func TestLicenses_GetActive(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		entSvc := entitlements.NewService(zaptest.NewLogger(t), db.Console().Entitlements())
		licenses := entSvc.Licenses()

		user, err := db.Console().Users().Insert(ctx, &console.User{
			ID:           testrand.UUID(),
			Email:        "test@storj.test",
			PasswordHash: []byte("password"),
		})
		require.NoError(t, err)
		require.NotNil(t, user)

		userID := user.ID
		publicID := testrand.UUID()
		now := time.Now().UTC().Truncate(time.Second)

		t.Run("no licenses", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Empty(t, active)
		})

		// Set up various licenses for testing.
		expectedLicenses := []entitlements.AccountLicense{
			{
				Type:      "pro",
				Count:     1,
				ExpiresAt: now.Add(30 * 24 * time.Hour),
			},
			{
				Type:      "enterprise",
				Count:     1,
				PublicID:  publicID.String(),
				ExpiresAt: now.Add(60 * 24 * time.Hour),
			},
			{
				Type:       "basic",
				Count:      1,
				PublicID:   publicID.String(),
				BucketName: "specific-bucket",
				ExpiresAt:  now.Add(90 * 24 * time.Hour),
				Key:        testrand.Bytes(32),
			},
			{
				Type:      "expired",
				Count:     1,
				ExpiresAt: now.Add(-24 * time.Hour),
			},
			{
				Type:      "revoked",
				Count:     1,
				ExpiresAt: now.Add(30 * 24 * time.Hour),
				RevokedAt: now.Add(-24 * time.Hour),
			},
			{
				Type:      "future-revoked",
				Count:     1,
				ExpiresAt: now.Add(30 * 24 * time.Hour),
				RevokedAt: now.Add(24 * time.Hour),
			},
		}

		require.NoError(t, licenses.Set(ctx, userID, entitlements.AccountLicenses{
			Licenses: expectedLicenses,
		}))

		t.Run("get all active without filters", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Len(t, active, len(expectedLicenses))
			require.ElementsMatch(t, expectedLicenses, active)
		})

		t.Run("filter by time - exclude expired", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				Now: &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("filter by license type", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "pro",
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
			}, active)
		})

		t.Run("filter by license type and time", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "expired",
				Now:         &now,
			})
			require.NoError(t, err)
			require.Empty(t, active)
		})

		t.Run("global license matches all projects", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID: publicID,
				Now:      &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("project-specific license", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				LicenseType: "enterprise",
				PublicID:    publicID,
				Now:         &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[1],
			}, active)
		})

		t.Run("bucket-specific license", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID:   publicID,
				BucketName: "specific-bucket",
				Now:        &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
				expectedLicenses[5],
			}, active)
		})

		t.Run("non-matching project", func(t *testing.T) {
			otherPublicID := testrand.UUID()
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID: otherPublicID,
				Now:      &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[5],
			}, active)
		})

		t.Run("non-matching bucket", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				PublicID:   publicID,
				BucketName: "other-bucket",
				Now:        &now,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[5],
			}, active)
		})

		t.Run("future time excludes future revocations", func(t *testing.T) {
			futureTime := now.Add(48 * time.Hour)
			active, err := licenses.GetActive(ctx, userID, entitlements.GetActiveOptions{
				Now: &futureTime,
			})
			require.NoError(t, err)

			require.ElementsMatch(t, []entitlements.AccountLicense{
				expectedLicenses[0],
				expectedLicenses[1],
				expectedLicenses[2],
			}, active)
		})

		t.Run("non-existent user", func(t *testing.T) {
			active, err := licenses.GetActive(ctx, testrand.UUID(), entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Empty(t, active)
		})

		// A license that has not started yet is not in force, so it must not be
		// reported as active. BillableSeatDays already treats it that way, and the two
		// have to agree on when a license applies. Its own account, so the shared
		// fixture above is untouched.
		t.Run("filter by time - exclude not yet started", func(t *testing.T) {
			other, err := db.Console().Users().Insert(ctx, &console.User{
				ID:           testrand.UUID(),
				Email:        "startsat@storj.test",
				PasswordHash: []byte("password"),
			})
			require.NoError(t, err)

			// Count is set explicitly because UnmarshalJSON defaults an absent one to
			// 1, which would not match a zero value on the way back out.
			future := entitlements.AccountLicense{
				Type:      entitlements.OMLicenseType,
				Count:     1,
				StartsAt:  now.Add(time.Hour),
				ExpiresAt: now.AddDate(1, 0, 0),
			}
			started := entitlements.AccountLicense{
				Type:      "started-license",
				Count:     1,
				StartsAt:  now.Add(-time.Hour),
				ExpiresAt: now.AddDate(1, 0, 0),
			}
			// A license from before the field existed reads as always in force.
			legacy := entitlements.AccountLicense{
				Type:      "legacy-license",
				Count:     1,
				ExpiresAt: now.AddDate(1, 0, 0),
			}
			require.NoError(t, licenses.Set(ctx, other.ID, entitlements.AccountLicenses{
				Licenses: []entitlements.AccountLicense{future, started, legacy},
			}))

			active, err := licenses.GetActive(ctx, other.ID, entitlements.GetActiveOptions{Now: &now})
			require.NoError(t, err)
			require.ElementsMatch(t, []entitlements.AccountLicense{started, legacy}, active)

			// Once its start time has passed it is in force.
			later := now.Add(2 * time.Hour)
			active, err = licenses.GetActive(ctx, other.ID, entitlements.GetActiveOptions{Now: &later})
			require.NoError(t, err)
			require.ElementsMatch(t, []entitlements.AccountLicense{future, started, legacy}, active)

			// Without a Now the filter does not apply at all.
			active, err = licenses.GetActive(ctx, other.ID, entitlements.GetActiveOptions{})
			require.NoError(t, err)
			require.Len(t, active, 3)
		})
	})
}

func TestAccountLicense_BillableSeatDays(t *testing.T) {
	// July 2026: 31 days.
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	day := func(d int) time.Time {
		return time.Date(2026, 7, d, 0, 0, 0, 0, time.UTC)
	}

	for _, tt := range []struct {
		name       string
		license    entitlements.AccountLicense
		billedDays int
		ok         bool
	}{
		{
			name:       "active for whole period",
			license:    entitlements.AccountLicense{StartsAt: periodStart, ExpiresAt: day(31).AddDate(1, 0, 0)},
			billedDays: 31,
			ok:         true,
		},
		{
			name:       "started before the period",
			license:    entitlements.AccountLicense{StartsAt: day(1).AddDate(0, -3, 0), ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			billedDays: 31,
			ok:         true,
		},
		{
			name:       "zero StartsAt is treated as the whole period",
			license:    entitlements.AccountLicense{ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			billedDays: 31,
			ok:         true,
		},
		{
			name:       "started mid-period is prorated",
			license:    entitlements.AccountLicense{StartsAt: day(21), ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			billedDays: 11,
			ok:         true,
		},
		{
			name:       "started on the last day bills one day",
			license:    entitlements.AccountLicense{StartsAt: day(31), ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			billedDays: 1,
			ok:         true,
		},
		{
			name:       "partial day is rounded up",
			license:    entitlements.AccountLicense{StartsAt: day(21).Add(6 * time.Hour), ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			billedDays: 11,
			ok:         true,
		},
		{
			// Expiry is a scheduled end, so the charge stops there.
			name: "expiring mid-period is capped at the expiry date",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: day(10),
			},
			billedDays: 9,
			ok:         true,
		},
		{
			name: "starting and expiring inside the period bills only those days",
			license: entitlements.AccountLicense{
				StartsAt:  day(11),
				ExpiresAt: day(21),
			},
			billedDays: 10,
			ok:         true,
		},
		{
			name: "expiring exactly at the period end bills the whole period",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: periodEnd,
			},
			billedDays: 31,
			ok:         true,
		},
		{
			name: "expiring after the period bills the whole period",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: periodEnd.AddDate(0, 1, 0),
			},
			billedDays: 31,
			ok:         true,
		},
		{
			name:       "zero ExpiresAt never caps the charge",
			license:    entitlements.AccountLicense{StartsAt: day(21)},
			billedDays: 11,
			ok:         true,
		},
		{
			// The day it ends is not charged to it, so a replacement can be charged
			// for that day instead.
			name: "the day of a mid-day expiry is not charged",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: day(10).Add(6 * time.Hour),
			},
			billedDays: 9,
			ok:         true,
		},
		{
			// Revocation ends access, so like expiry it caps the charge.
			name: "revoked mid-period is capped at the revocation date",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: periodEnd.AddDate(1, 0, 0),
				RevokedAt: day(10),
			},
			billedDays: 9,
			ok:         true,
		},
		{
			name: "revocation before expiry caps the charge",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: day(20),
				RevokedAt: day(10),
			},
			billedDays: 9,
			ok:         true,
		},
		{
			name: "expiry before revocation caps the charge",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart,
				ExpiresAt: day(10),
				RevokedAt: day(20),
			},
			billedDays: 9,
			ok:         true,
		},
		{
			name: "prorated at both ends",
			license: entitlements.AccountLicense{
				StartsAt:  day(11),
				ExpiresAt: periodEnd.AddDate(1, 0, 0),
				RevokedAt: day(21),
			},
			billedDays: 10,
			ok:         true,
		},
		{
			name: "revoked before the period is not billable",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart.AddDate(0, -2, 0),
				ExpiresAt: periodEnd.AddDate(1, 0, 0),
				RevokedAt: periodStart.AddDate(0, -1, 0),
			},
			ok: false,
		},
		{
			name: "revoked exactly at the period start is not billable",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart.AddDate(0, -2, 0),
				ExpiresAt: periodEnd.AddDate(1, 0, 0),
				RevokedAt: periodStart,
			},
			ok: false,
		},
		{
			name: "expired before the period is not billable",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart.AddDate(0, -2, 0),
				ExpiresAt: periodStart.AddDate(0, -1, 0),
			},
			ok: false,
		},
		{
			name: "expired exactly at the period start is not billable",
			license: entitlements.AccountLicense{
				StartsAt:  periodStart.AddDate(0, -2, 0),
				ExpiresAt: periodStart,
			},
			ok: false,
		},
		{
			name: "starting and ending within one day is not billable",
			license: entitlements.AccountLicense{
				StartsAt:  day(10).Add(9 * time.Hour),
				RevokedAt: day(10).Add(17 * time.Hour),
				ExpiresAt: periodEnd.AddDate(1, 0, 0),
			},
			ok: false,
		},
		{
			name:    "starts after the period is not billable",
			license: entitlements.AccountLicense{StartsAt: periodEnd, ExpiresAt: periodEnd.AddDate(1, 0, 0)},
			ok:      false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			billedDays, daysInPeriod, ok := tt.license.BillableSeatDays(periodStart, periodEnd)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, 31, daysInPeriod)
			if !tt.ok {
				return
			}
			require.Equal(t, tt.billedDays, billedDays)
		})
	}

	t.Run("invalid period", func(t *testing.T) {
		license := entitlements.AccountLicense{StartsAt: periodStart}
		_, daysInPeriod, ok := license.BillableSeatDays(periodEnd, periodStart)
		require.False(t, ok)
		require.Zero(t, daysInPeriod)
	})

	t.Run("revoking and re-granting in one period bills the period once", func(t *testing.T) {
		// Granting and revoking both stamp the wall clock, so the times a license
		// changes hands are not midnight aligned.
		for _, tt := range []struct {
			name                 string
			revokedAt, grantedAt time.Time
		}{
			{
				name:      "midnight",
				revokedAt: day(10),
				grantedAt: day(10),
			},
			{
				name:      "wall clock",
				revokedAt: day(10).Add(14*time.Hour + 30*time.Minute),
				grantedAt: day(10).Add(14*time.Hour + 35*time.Minute),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				replaced := entitlements.AccountLicense{
					StartsAt:  periodStart,
					ExpiresAt: periodEnd.AddDate(1, 0, 0),
					RevokedAt: tt.revokedAt,
				}
				replacement := entitlements.AccountLicense{
					StartsAt:  tt.grantedAt,
					ExpiresAt: periodEnd.AddDate(1, 0, 0),
				}

				oldDays, daysInPeriod, ok := replaced.BillableSeatDays(periodStart, periodEnd)
				require.True(t, ok)
				require.Equal(t, 9, oldDays)

				newDays, _, ok := replacement.BillableSeatDays(periodStart, periodEnd)
				require.True(t, ok)
				require.Equal(t, 22, newDays)

				// The two cover the period exactly, with no day billed twice.
				require.Equal(t, daysInPeriod, oldDays+newDays)
			})
		}
	})
}
