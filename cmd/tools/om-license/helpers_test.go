// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/entitlements"
)

func TestHasActiveOMLicense(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour)
	past := now.Add(-24 * time.Hour)

	tests := []struct {
		name     string
		licenses []entitlements.AccountLicense
		want     bool
	}{
		{"empty returns false", nil, false},
		{"active OM license returns true", []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: future}}, true},
		{"zero ExpiresAt counts as active", []entitlements.AccountLicense{{Type: omLicenseType}}, true},
		{"expired OM license returns false", []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: past}}, false},
		{"revoked OM license returns false", []entitlements.AccountLicense{{Type: omLicenseType, ExpiresAt: future, RevokedAt: past}}, false},
		{"non-OM active license returns false", []entitlements.AccountLicense{{Type: "enterprise", ExpiresAt: future}}, false},
		{
			name: "expired OM alongside active non-OM returns false",
			licenses: []entitlements.AccountLicense{
				{Type: "enterprise", ExpiresAt: future},
				{Type: omLicenseType, ExpiresAt: past},
			},
			want: false,
		},
		{
			name: "expired OM alongside active OM returns true",
			licenses: []entitlements.AccountLicense{
				{Type: omLicenseType, ExpiresAt: past},
				{Type: omLicenseType, ExpiresAt: future},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasActiveOMLicense(entitlements.AccountLicenses{Licenses: tt.licenses}, now)
			require.Equal(t, tt.want, got)
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
		err := (&Config{BatchSize: 100}).Verify()
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
