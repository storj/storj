// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb/consoledb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestTenantWhiteLabelConfigs(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		configs := db.Console().TenantWhiteLabelConfigs()

		_, err := configs.Get(ctx, "acme")
		require.ErrorIs(t, err, console.ErrTenantWhiteLabelConfigNotFound)

		cfg := console.WhiteLabelConfig{
			TenantID:    "acme",
			HostName:    "acme.example.test",
			Name:        "Acme",
			LogoURLs:    map[string]string{"full-light": "https://acme.example.test/logo.png"},
			FaviconURLs: map[string]string{"16x16": "https://acme.example.test/favicon.ico"},
			Colors: map[string]string{
				"primary-light": "#FF0000",
				"primary-dark":  "#00FF00",
			},
			SupportURL:        "https://acme.example.test/support",
			CompanyName:       "Acme Inc.",
			FreeTrialsEnabled: true,
		}

		upserted, err := configs.Upsert(ctx, "acme", cfg)
		require.NoError(t, err)
		require.Equal(t, "acme", upserted.TenantID)
		require.Equal(t, cfg, upserted.Config)

		got, err := configs.Get(ctx, "acme")
		require.NoError(t, err)
		require.Equal(t, cfg, got.Config)

		// Replacing the config overwrites it.
		cfg2 := console.WhiteLabelConfig{
			Name:   "Acme 2",
			Colors: map[string]string{"primary-light": "#0000FF"},
		}
		_, err = configs.Upsert(ctx, "acme", cfg2)
		require.NoError(t, err)

		got, err = configs.Get(ctx, "acme")
		require.NoError(t, err)
		require.Equal(t, cfg2, got.Config)

		_, err = configs.Upsert(ctx, "empty", console.WhiteLabelConfig{})
		require.NoError(t, err)

		list, err := configs.List(ctx)
		require.NoError(t, err)
		require.Len(t, list, 2)
		require.Equal(t, "acme", list[0].TenantID)
		require.Equal(t, cfg2, list[0].Config)
		require.Equal(t, "empty", list[1].TenantID)
		require.Equal(t, console.WhiteLabelConfig{}, list[1].Config)

		require.NoError(t, configs.Delete(ctx, "acme"))
		_, err = configs.Get(ctx, "acme")
		require.ErrorIs(t, err, console.ErrTenantWhiteLabelConfigNotFound)
	})
}

func TestDecodeWhiteLabelConfig(t *testing.T) {
	expected := console.WhiteLabelConfig{
		TenantID: "acme",
		Name:     "Acme",
		Colors:   map[string]string{"primary-light": "#FF0000"},
	}

	for _, tt := range []struct {
		name string
		data string
	}{
		{"json", `{"tenant-id":"acme","name":"Acme","colors":{"primary-light":"#FF0000"}}`},
		{"yaml", "tenant-id: acme\nname: Acme\ncolors:\n  primary-light: '#FF0000'\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := consoledb.DecodeWhiteLabelConfig([]byte(tt.data))
			require.NoError(t, err)
			require.Equal(t, expected, cfg)
		})
	}

	cfg, err := consoledb.DecodeWhiteLabelConfig(nil)
	require.NoError(t, err)
	require.Equal(t, console.WhiteLabelConfig{}, cfg)

	cfg, err = consoledb.DecodeWhiteLabelConfig([]byte("{}"))
	require.NoError(t, err)
	require.Equal(t, console.WhiteLabelConfig{}, cfg)

	_, err = consoledb.DecodeWhiteLabelConfig([]byte("name: [unclosed"))
	require.Error(t, err)

	// Valid JSON with a wrong type is an error, not decoded as YAML.
	_, err = consoledb.DecodeWhiteLabelConfig([]byte(`{"name": 123}`))
	require.Error(t, err)
}
