// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/console"
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

func TestExtendedConfig_UseBucketLevelObjectVersioning(t *testing.T) {
	projectA := &console.Project{
		ID: testrand.UUID(),
	}
	projectB := &console.Project{
		ID: testrand.UUID(),
	}

	// 1. Versioning globally enabled
	config, err := metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioning: true,
	})
	require.NoError(t, err)
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectA))
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectB))

	// 2.1. Versioning disabled globally, but enabled for project A (closed beta)
	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioning: false,
		UseBucketLevelObjectVersioningProjects: []string{
			projectA.ID.String(),
		},
	})
	require.NoError(t, err)
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectA))
	require.False(t, config.UseBucketLevelObjectVersioningByProject(projectB))

	// 2.2. Versioning disabled globally, but enabled for project B (closed beta)
	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioning: false,
		UseBucketLevelObjectVersioningProjects: []string{
			projectB.ID.String(),
		},
	})
	require.NoError(t, err)
	require.False(t, config.UseBucketLevelObjectVersioningByProject(projectA))
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectB))

	// 3. Versioning disabled globally
	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioning: false,
	})
	require.NoError(t, err)

	// 3.1. Project A is prompted for versioning beta, but has not opted in
	projectA.PromptedForVersioningBeta = true
	projectA.DefaultVersioning = console.VersioningUnsupported
	// 3.2. Project B is prompted for versioning beta, and has opted in
	projectB.PromptedForVersioningBeta = true
	projectB.DefaultVersioning = console.Unversioned
	require.False(t, config.UseBucketLevelObjectVersioningByProject(projectA))
	require.True(t, config.UseBucketLevelObjectVersioningByProject(projectB))

	// 3.3. Project A is not prompted for versioning beta
	projectA.PromptedForVersioningBeta = false
	projectA.DefaultVersioning = console.Unversioned
	require.False(t, config.UseBucketLevelObjectVersioningByProject(projectA))
}

func TestExtendedConfig_ObjectLockSupported(t *testing.T) {
	projectA := &console.Project{
		ID: testrand.UUID(),
	}
	projectB := &console.Project{
		ID: testrand.UUID(),
	}

	// 1. Versioning enabled && object lock disabled
	config, err := metainfo.NewExtendedConfig(metainfo.Config{
		UseBucketLevelObjectVersioning: true,
	})
	require.NoError(t, err)
	require.False(t, config.ObjectLockEnabledByProject(projectA))

	// 2. project A in versioning closed beta
	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		ObjectLockEnabled: true,
		UseBucketLevelObjectVersioningProjects: []string{
			projectA.ID.String(),
		},
	})
	require.NoError(t, err)
	require.True(t, config.ObjectLockEnabledByProject(projectA))
	require.False(t, config.ObjectLockEnabledByProject(projectB))

	// 2.1 Versioning disabled globally
	config, err = metainfo.NewExtendedConfig(metainfo.Config{
		ObjectLockEnabled: true,
	})
	require.NoError(t, err)

	// 2.2. Project A not in versioning beta
	projectA.PromptedForVersioningBeta = true
	projectA.DefaultVersioning = console.VersioningUnsupported
	// 2.3. Project B is in versioning public beta
	projectB.PromptedForVersioningBeta = true
	projectB.DefaultVersioning = console.Unversioned
	require.False(t, config.ObjectLockEnabledByProject(projectA))
	require.True(t, config.ObjectLockEnabledByProject(projectB))
}

func TestUUIDsFlag(t *testing.T) {
	var UUIDs metainfo.UUIDsFlag
	err := UUIDs.Set("")
	require.NoError(t, err)
	require.Len(t, UUIDs, 0)

	testIDA := testrand.UUID()
	err = UUIDs.Set(testIDA.String())
	require.NoError(t, err)
	require.Equal(t, metainfo.UUIDsFlag{
		testIDA: {},
	}, UUIDs)

	testIDB := testrand.UUID()
	err = UUIDs.Set(testIDA.String() + "," + testIDB.String())
	require.NoError(t, err)
	require.Equal(t, metainfo.UUIDsFlag{
		testIDA: {},
		testIDB: {},
	}, UUIDs)
}
