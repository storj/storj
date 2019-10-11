package versioncontrol_test

import (
	"encoding/hex"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/versioncontrol"
)

func TestVersions_ValidateRollouts(t *testing.T) {
	versions := versioncontrol.Versions{
		Bootstrap:   versioncontrol.Binary{
			Rollout: randRollout(t),
		},
		Satellite:   versioncontrol.Binary{
			Rollout: randRollout(t),
		},
		Storagenode: versioncontrol.Binary{
			Rollout: randRollout(t),
		},
		Uplink:      versioncontrol.Binary{
			Rollout: randRollout(t),
		},
		Gateway:     versioncontrol.Binary{
			Rollout: randRollout(t),
		},
		Identity:    versioncontrol.Binary{
			Rollout: randRollout(t),
		},
	}
	
	err := versions.ValidateRollouts()
	require.NoError(t, err)
}

func TestRollout_Validate(t *testing.T) {
	for i := 0; i < 100; i ++ {
		rollout := versioncontrol.Rollout{
			Seed:   randSeedString(t),
			Cursor: i,
		}

		err := rollout.Validate()
		require.NoError(t, err)
	}
}

func TestRollout_Validate_error(t *testing.T) {
	scenarios := []struct {
		name        string
		rollout     versioncontrol.Rollout
		errContains string
	}{
		{
			"empty seed",
			versioncontrol.Rollout{
				Seed:   "",
				Cursor: 0,
			},
			"invalid seed length",
		},
		{
			"short seed",
			versioncontrol.Rollout{
				// 31 byte seed
				Seed:   "00000000000000000000000000000000000000000000000000000000000000",
				Cursor: 0,
			},
			"invalid seed length:",
		},
		{
			"long seed",
			versioncontrol.Rollout{
				// 33 byte seed
				Seed:   "000000000000000000000000000000000000000000000000000000000000000000",
				Cursor: 0,
			},
			"invalid seed length:",
		},
		{
			"invalid seed",
			versioncontrol.Rollout{
				// non-hex seed
				Seed:   "G000000000000000000000000000000000000000000000000000000000000000",
				Cursor: 0,
			},
			"invalid seed:",
		},
		{
			"negative cursor",
			versioncontrol.Rollout{
				Seed:   "0000000000000000000000000000000000000000000000000000000000000000",
				Cursor: -1,
			},
			"invalid cursor percentage:",
		},
		{
			"cursor too big",
			versioncontrol.Rollout{
				Seed:   "0000000000000000000000000000000000000000000000000000000000000000",
				Cursor: 101,
			},
			"invalid cursor percentage:",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			err := scenario.rollout.Validate()
			require.Error(t, err)
			require.True(t, versioncontrol.Error.Has(err))
			require.Contains(t, err.Error(), scenario.errContains)
		})
	}
}

func randRollout(t *testing.T) versioncontrol.Rollout {
	return versioncontrol.Rollout{
		Seed:   randSeedString(t),
		Cursor: rand.Intn(100),
	}
}

func randSeedString(t *testing.T) string {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)

	return hex.EncodeToString(seed)
}
