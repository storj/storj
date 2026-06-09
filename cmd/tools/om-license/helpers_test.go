// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/entitlements"
)

func TestFindActiveFreeOMLicense(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name        string
		licenses    []entitlements.AccountLicense
		wantFound   bool
		wantIndexes []int
	}{
		{
			name:        "empty returns not found",
			licenses:    nil,
			wantFound:   false,
			wantIndexes: nil,
		},
		{
			name:        "active free OM license returns found",
			licenses:    []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: future}},
			wantFound:   true,
			wantIndexes: []int{0},
		},
		{
			name:        "zero ExpiresAt counts as active",
			licenses:    []entitlements.AccountLicense{{Type: omLicenseType}},
			wantFound:   true,
			wantIndexes: []int{0},
		},
		{
			name:      "expired OM license returns not found",
			licenses:  []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: past}},
			wantFound: false,
		},
		{
			name:      "revoked OM license returns not found",
			licenses:  []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: future, RevokedAt: past}},
			wantFound: false,
		},
		{
			name:      "non-OM active license returns not found",
			licenses:  []entitlements.AccountLicense{{Type: "enterprise", ExpiresAt: future}},
			wantFound: false,
		},
		{
			name:      "OM license (ProductID != 0) returns not found",
			licenses:  []entitlements.AccountLicense{{Type: omLicenseType, ProductID: 42, ExpiresAt: future}},
			wantFound: false,
		},
		{
			name:        "free OM with Count > 1 returns found",
			licenses:    []entitlements.AccountLicense{{Type: omLicenseType, ProductID: 0, Count: 3, ExpiresAt: future}},
			wantFound:   true,
			wantIndexes: []int{0},
		},
		{
			name: "2 free OM with only one Count > 1 returns found with the two",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future.Add(5 * time.Minute)},
				{Type: omLicenseType, ProductID: 0, Count: 7, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{0, 1},
		},
		{
			name: "2 free OM with Count > 1 returns found with the 2",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 3, ExpiresAt: future.Add(10 * time.Minute)},
				{Type: omLicenseType, ProductID: 0, Count: 5, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{0, 1},
		},
		{
			name: "3 free OM with two Count > 1 and one under returns found with the 3",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 3, ExpiresAt: future.Add(10 * time.Minute)},
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future.Add(1 * time.Hour)},
				{Type: omLicenseType, ProductID: 0, Count: 5, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{0, 1, 2},
		},
		{
			name: "expired OM alongside active non-OM returns not found",
			licenses: []entitlements.AccountLicense{
				{Type: "enterprise", ExpiresAt: future},
				{Type: omLicenseType, ExpiresAt: past},
			},
			wantFound: false,
		},
		{
			name: "expired OM alongside active free OM returns found at correct index",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ExpiresAt: past},
				{Type: omLicenseType, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{1},
		},
		{
			name: "OM (ProductID != 0) alongside free OM returns the free row",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 7, Count: 10, ExpiresAt: future},
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{1},
		},
		{
			name: "OM (ProductID != 0) alongside 2 free OM returns the 2 free rows",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future},
				{Type: omLicenseType, ProductID: 7, Count: 10, ExpiresAt: future},
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future.Add(1 * time.Minute)},
			},
			wantFound:   true,
			wantIndexes: []int{0, 2},
		},
		{
			name: "OM (ProductID != 0) alongside 2 free OM with one over the established count returns the 2 free rows",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ProductID: 7, Count: 10, ExpiresAt: future},
				{Type: omLicenseType, ProductID: 0, Count: 1, ExpiresAt: future.Add(1 * time.Minute)},
				{Type: omLicenseType, ProductID: 0, Count: 5, ExpiresAt: future},
			},
			wantFound:   true,
			wantIndexes: []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx, ok := findActiveFreeOMLicense(entitlements.AccountLicenses{Licenses: tt.licenses}, now)
			require.Equal(t, tt.wantFound, ok)
			require.EqualValues(t, tt.wantIndexes, idx)
		})
	}
}

func TestConfigVerify(t *testing.T) {
	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)

	validBase := func() Config {
		return Config{
			SatelliteDB: "postgres://test",
			ExpiresAt:   future,
			BatchSize:   100,
			Count:       1,
		}
	}

	t.Run("valid config populates parsed fields", func(t *testing.T) {
		cfg := validBase()
		cfg.EmailPattern = "*@example.com"
		require.NoError(t, cfg.Verify())
		require.False(t, cfg.parsedExpiresAt.IsZero())
		require.True(t, cfg.matchesEmail("USER@Example.com"))
		require.False(t, cfg.matchesEmail("user@other.com"))
	})

	t.Run("no pattern matches every email", func(t *testing.T) {
		cfg := validBase()
		require.NoError(t, cfg.Verify())
		require.True(t, cfg.matchesEmail("anyone@anywhere.test"))
	})

	t.Run("missing required flags reported together", func(t *testing.T) {
		err := (&Config{BatchSize: 100, Count: 1}).Verify()
		require.Error(t, err)
		msg := err.Error()
		require.Contains(t, msg, "--satellitedb")
		require.Contains(t, msg, "--expires-at")
	})

	t.Run("batch size must be positive", func(t *testing.T) {
		cfg := validBase()
		cfg.BatchSize = 0
		require.ErrorContains(t, cfg.Verify(), "--batch-size")
	})

	t.Run("count must be >= 1", func(t *testing.T) {
		cfg := validBase()
		cfg.Count = 0
		require.ErrorContains(t, cfg.Verify(), "--count")
		cfg.Count = -3
		require.ErrorContains(t, cfg.Verify(), "--count")
	})

	t.Run("expires-at must parse as RFC3339", func(t *testing.T) {
		cfg := validBase()
		cfg.ExpiresAt = "not-a-time"
		require.ErrorContains(t, cfg.Verify(), "RFC3339")
	})

	t.Run("expires-at must be in the future", func(t *testing.T) {
		cfg := validBase()
		cfg.ExpiresAt = past
		require.ErrorContains(t, cfg.Verify(), "future")
	})

	t.Run("invalid email pattern reports error", func(t *testing.T) {
		cfg := validBase()
		cfg.EmailPattern = "[bad"
		require.ErrorContains(t, cfg.Verify(), "--email-pattern")
	})
}
