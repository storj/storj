// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol_test

import (
	"encoding/hex"
	"math/rand"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/versioncontrol"
)

var rolloutErrScenarios = []struct {
	name        string
	rollout     versioncontrol.RolloutConfig
	errContains string
}{
	{
		"short seed",
		versioncontrol.RolloutConfig{
			// 31 byte seed
			Seed:   "00000000000000000000000000000000000000000000000000000000000000",
			Cursor: 0,
		},
		"invalid seed length:",
	},
	{
		"long seed",
		versioncontrol.RolloutConfig{
			// 33 byte seed
			Seed:   "000000000000000000000000000000000000000000000000000000000000000000",
			Cursor: 0,
		},
		"invalid seed length:",
	},
	{
		"invalid seed",
		versioncontrol.RolloutConfig{
			// non-hex seed
			Seed:   "G000000000000000000000000000000000000000000000000000000000000000",
			Cursor: 0,
		},
		"invalid seed:",
	},
	{
		"negative cursor",
		versioncontrol.RolloutConfig{
			Seed:   "0000000000000000000000000000000000000000000000000000000000000000",
			Cursor: -1,
		},
		"invalid cursor percentage:",
	},
	{
		"cursor too big",
		versioncontrol.RolloutConfig{
			Seed:   "0000000000000000000000000000000000000000000000000000000000000000",
			Cursor: 101,
		},
		"invalid cursor percentage:",
	},
}

func TestPeer_Run(t *testing.T) {
	testVersion := "v0.0.1"
	testServiceVersions := versioncontrol.OldVersionConfig{
		Gateway:     testVersion,
		Identity:    testVersion,
		Satellite:   testVersion,
		Storagenode: testVersion,
		Uplink:      testVersion,
	}

	t.Run("random rollouts", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			config := versioncontrol.Config{
				Versions: testServiceVersions,
				Binary:   validRandVersions(t),
			}

			peer, err := versioncontrol.New(zaptest.NewLogger(t), &config)
			require.NoError(t, err)
			require.NotNil(t, peer)
		}
	})

	t.Run("empty rollout seed", func(t *testing.T) {
		versionsType := reflect.TypeOf(versioncontrol.ProcessesConfig{})
		fieldCount := versionsType.NumField()

		// test invalid rollout for each binary
		for i := 0; i < fieldCount; i++ {
			versions := versioncontrol.ProcessesConfig{}
			versionsValue := reflect.ValueOf(&versions)
			field := versionsValue.Elem().Field(i)

			binary := versioncontrol.ProcessConfig{
				Rollout: versioncontrol.RolloutConfig{
					Seed:   "",
					Cursor: 0,
				},
			}

			field.Set(reflect.ValueOf(binary))

			config := versioncontrol.Config{
				Versions: testServiceVersions,
				Binary:   versions,
			}

			peer, err := versioncontrol.New(zaptest.NewLogger(t), &config)
			require.NoError(t, err)
			require.NotNil(t, peer)
		}
	})
}

func TestPeer_Run_error(t *testing.T) {
	for _, scenario := range rolloutErrScenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			versionsType := reflect.TypeOf(versioncontrol.ProcessesConfig{})
			fieldCount := versionsType.NumField()

			// test invalid rollout for each binary
			for i := 0; i < fieldCount; i++ {
				versions := versioncontrol.ProcessesConfig{}
				versionsValue := reflect.ValueOf(&versions)
				field := reflect.Indirect(versionsValue).Field(i)

				binary := versioncontrol.ProcessConfig{
					Rollout: scenario.rollout,
				}

				field.Set(reflect.ValueOf(binary))

				config := versioncontrol.Config{
					Binary: versions,
				}

				peer, err := versioncontrol.New(zaptest.NewLogger(t), &config)
				require.Nil(t, peer)
				require.Error(t, err)
				require.Contains(t, err.Error(), scenario.errContains)
			}
		})
	}
}

func TestVersions_ValidateRollouts(t *testing.T) {
	versions := validRandVersions(t)
	err := versions.ValidateRollouts(zaptest.NewLogger(t))
	require.NoError(t, err)
}

func TestRollout_Validate(t *testing.T) {
	for i := 0; i < 100; i++ {
		rollout := versioncontrol.RolloutConfig{
			Seed:   randSeedString(t),
			Cursor: i,
		}

		err := rollout.Validate()
		require.NoError(t, err)
	}
}

func TestRollout_Validate_error(t *testing.T) {
	for _, scenario := range rolloutErrScenarios {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.rollout.Validate()
			require.Error(t, err)
			require.True(t, versioncontrol.RolloutErr.Has(err))
			require.Contains(t, err.Error(), scenario.errContains)
		})
	}
}

func validRandVersions(t *testing.T) versioncontrol.ProcessesConfig {
	t.Helper()

	return versioncontrol.ProcessesConfig{
		Satellite: versioncontrol.ProcessConfig{
			Rollout: randRollout(t),
		},
		Storagenode: versioncontrol.ProcessConfig{
			Rollout: randRollout(t),
		},
		Uplink: versioncontrol.ProcessConfig{
			Rollout: randRollout(t),
		},
		Gateway: versioncontrol.ProcessConfig{
			Rollout: randRollout(t),
		},
		Identity: versioncontrol.ProcessConfig{
			Rollout: randRollout(t),
		},
	}
}

func randRollout(t *testing.T) versioncontrol.RolloutConfig {
	t.Helper()

	return versioncontrol.RolloutConfig{
		Seed:   randSeedString(t),
		Cursor: rand.Intn(101),
	}
}

func randSeedString(t *testing.T) string {
	t.Helper()

	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)

	return hex.EncodeToString(seed)
}
