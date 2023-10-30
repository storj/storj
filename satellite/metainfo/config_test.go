// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
)

func TestRSConfigValidation(t *testing.T) {
	tests := []struct {
		description    string
		configString   string
		expectedConfig metainfo.RSConfig
		expectError    bool
	}{
		{
			description:  "valid rs config",
			configString: "4/8/10/20-256B",
			expectedConfig: metainfo.RSConfig{
				ErasureShareSize: 256 * memory.B, Min: 4, Repair: 8, Success: 10, Total: 20,
			},
			expectError: false,
		},
		{
			description:  "invalid rs config - numbers decrease",
			configString: "4/8/5/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - starts at 0",
			configString: "0/2/4/6-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - strings",
			configString: "4/a/b/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - floating-point numbers",
			configString: "4/5.2/7/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - not enough items",
			configString: "4/5/20-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - too many items",
			configString: "4/5/20/30/50-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty numbers",
			configString: "-256B",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty size",
			configString: "1/2/3/4-",
			expectError:  true,
		},
		{
			description:  "invalid rs config - empty",
			configString: "",
			expectError:  true,
		},
		{
			description:  "invalid valid rs config - invalid share size",
			configString: "4/8/10/20-256A",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Log(tt.description)

		rsConfig := metainfo.RSConfig{}
		err := rsConfig.Set(tt.configString)
		if tt.expectError {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.EqualValues(t, tt.expectedConfig.ErasureShareSize, rsConfig.ErasureShareSize)
			require.EqualValues(t, tt.expectedConfig.Min, rsConfig.Min)
			require.EqualValues(t, tt.expectedConfig.Repair, rsConfig.Repair)
			require.EqualValues(t, tt.expectedConfig.Success, rsConfig.Success)
			require.EqualValues(t, tt.expectedConfig.Total, rsConfig.Total)
		}
	}
}

func TestExtendedConfig_UsePendingObjectsTable(t *testing.T) {
	projectA := testrand.UUID()
	projectB := testrand.UUID()
	projectC := testrand.UUID()
	config, err := metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable: false,
		UsePendingObjectsTableProjects: []string{
			projectA.String(),
			projectB.String(),
		},
	})
	require.NoError(t, err)

	require.True(t, config.UsePendingObjectsTableByProject(projectA))
	require.True(t, config.UsePendingObjectsTableByProject(projectB))
	require.False(t, config.UsePendingObjectsTableByProject(projectC))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable: true,
		UsePendingObjectsTableProjects: []string{
			projectA.String(),
		},
	})
	require.NoError(t, err)

	require.True(t, config.UsePendingObjectsTableByProject(projectA))
	require.True(t, config.UsePendingObjectsTableByProject(projectB))
	require.True(t, config.UsePendingObjectsTableByProject(projectC))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable: false,
		UsePendingObjectsTableProjects: []string{
			"01000000-0000-0000-0000-000000000000",
		},
	})
	require.NoError(t, err)
	require.True(t, config.UsePendingObjectsTableByProject(uuid.UUID{1}))
}

func TestExtendedConfig_UsePendingObjectsTableRollout(t *testing.T) {
	uuidA := testrand.UUID()
	config, err := metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable:        false,
		UsePendingObjectsTableRollout: 0,
	})
	require.NoError(t, err)

	require.False(t, config.UsePendingObjectsTableByProject(uuidA))
	require.False(t, config.UsePendingObjectsTableByProject(makeUUID("00000001-0000-0000-0000-000000000000")))
	require.False(t, config.UsePendingObjectsTableByProject(makeUUID("FFFFFFFF-0000-0000-0000-000000000000")))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable:        false,
		UsePendingObjectsTableRollout: 50,
	})
	require.NoError(t, err)

	require.True(t, config.UsePendingObjectsTableByProject(makeUUID("00000001-0000-0000-0000-000000000000")))
	require.False(t, config.UsePendingObjectsTableByProject(makeUUID("FFFFFFFF-0000-0000-0000-000000000000")))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable:        false,
		UsePendingObjectsTableRollout: 25,
	})
	require.NoError(t, err)

	require.True(t, config.UsePendingObjectsTableByProject(makeUUID("00000001-0000-0000-0000-000000000000")))
	require.True(t, config.UsePendingObjectsTableByProject(makeUUID("3FFFFFFF-0000-0000-0000-000000000000")))
	require.False(t, config.UsePendingObjectsTableByProject(makeUUID("40000000-0000-0000-0000-000000000000")))
	require.False(t, config.UsePendingObjectsTableByProject(makeUUID("FFFFFFFF-0000-0000-0000-000000000000")))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable:        false,
		UsePendingObjectsTableRollout: 100,
	})
	require.NoError(t, err)

	require.True(t, config.UsePendingObjectsTableByProject(makeUUID("00000001-0000-0000-0000-000000000000")))
	require.True(t, config.UsePendingObjectsTableByProject(makeUUID("FFFFFFFF-0000-0000-0000-000000000000")))
}

func TestExtendedConfig_UseBucketLevelObjectVersioning(t *testing.T) {
	projectA := testrand.UUID()
	projectB := testrand.UUID()
	projectC := testrand.UUID()
	config, err := metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioningProjects: []string{
			projectA.String(),
			projectB.String(),
		},
	})
	require.NoError(t, err)

	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectA))
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectB))
	require.False(t, config.UseBucketLevelObjectVersioningByProject(projectC))

	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UsePendingObjectsTable: false,
		UseBucketLevelObjectVersioningProjects: []string{
			"01000000-0000-0000-0000-000000000000",
		},
	})
	require.NoError(t, err)
	require.True(t, config.UseBucketLevelObjectVersioningByProject(uuid.UUID{1}))
}

func makeUUID(uuidString string) uuid.UUID {
	value, _ := uuid.FromString(uuidString)
	return value
}
