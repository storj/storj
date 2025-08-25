// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"storj.io/storj/satellite/console"
)

func TestPlacementDetailsConfig(t *testing.T) {
	placementDetail := console.PlacementDetail{
		ID:          1,
		IdName:      "test-placement",
		Name:        "Test Placement",
		Title:       "Test Placement Title",
		Description: "Test Placement Description",
		WaitlistURL: "some-url",
	}
	details := []console.PlacementDetail{placementDetail}

	bytes, err := yaml.Marshal(details)
	require.NoError(t, err)
	validYaml := string(bytes)

	tmpFile, err := os.CreateTemp(t.TempDir(), "mapping_*.yaml")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, os.Remove(tmpFile.Name()))
		require.NoError(t, tmpFile.Close())
	}()

	bytes, err = yaml.Marshal(details)
	require.NoError(t, err)
	_, err = tmpFile.Write(bytes)
	require.NoError(t, err)

	tests := []struct {
		id     string
		config string
		// in the case of JSON, we only allow using it for backwards compatibility
		// the expected config string of cfg.String() will be in YAML format.
		expectStr string
		expectErr bool
	}{
		{
			id:     "empty string",
			config: "",
		},
		{
			id:     "valid YAML",
			config: validYaml,
		},
		{
			id:        "YAML file",
			config:    tmpFile.Name(),
			expectStr: validYaml,
		},
		{
			id:        "invalid string",
			config:    "invalid string",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			mapFromCfg := &console.PlacementDetails{}
			err := mapFromCfg.Set(tt.config)
			if tt.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.expectStr != "" {
				require.Equal(t, tt.expectStr, mapFromCfg.String())
				return
			}
			require.Equal(t, tt.config, mapFromCfg.String())
		})
	}
}
