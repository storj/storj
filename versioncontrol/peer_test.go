// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol_test

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
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

func TestPeerEndpoint(t *testing.T) {
	minimumVersion := "v0.0.1"
	suggestedVersion := "v0.0.2"

	createURL := func(process, version string) string {
		urlTmpl := "http://example.com/{version}/{process}_{os}_{arch}"
		url := strings.Replace(urlTmpl, "{version}", version, 1)
		url = strings.Replace(url, "{process}", process, 1)
		return url
	}

	config := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		Versions: versioncontrol.OldVersionConfig{
			Satellite:   minimumVersion,
			Storagenode: minimumVersion,
			Uplink:      minimumVersion,
			Gateway:     minimumVersion,
			Identity:    minimumVersion,
		},
		Binary: versioncontrol.ProcessesConfig{
			Storagenode: versioncontrol.ProcessConfig{
				Minimum: versioncontrol.VersionConfig{
					Version: minimumVersion,
					URL:     createURL("storagenode", minimumVersion),
				},
				Suggested: versioncontrol.VersionConfig{
					Version: suggestedVersion,
					URL:     createURL("storagenode", suggestedVersion),
				},
			},
			StoragenodeUpdater: versioncontrol.ProcessConfig{
				Minimum: versioncontrol.VersionConfig{
					Version: minimumVersion,
					URL:     createURL("storagenode-updater", minimumVersion),
				},
				Suggested: versioncontrol.VersionConfig{
					Version: suggestedVersion,
					URL:     createURL("storagenode-updater", suggestedVersion),
				},
			},
			Uplink: versioncontrol.ProcessConfig{
				Minimum: versioncontrol.VersionConfig{
					Version: minimumVersion,
					URL:     createURL("uplink", minimumVersion),
				},
				Suggested: versioncontrol.VersionConfig{
					Version: suggestedVersion,
					URL:     createURL("uplink", suggestedVersion),
				},
			},
			Gateway: versioncontrol.ProcessConfig{
				Minimum: versioncontrol.VersionConfig{
					Version: minimumVersion,
					URL:     createURL("gateway", minimumVersion),
				},
				Suggested: versioncontrol.VersionConfig{
					Version: suggestedVersion,
					URL:     createURL("gateway", suggestedVersion),
				},
			},
			Identity: versioncontrol.ProcessConfig{
				Minimum: versioncontrol.VersionConfig{
					Version: minimumVersion,
					URL:     createURL("identity", minimumVersion),
				},
				Suggested: versioncontrol.VersionConfig{
					Version: suggestedVersion,
					URL:     createURL("identity", suggestedVersion),
				},
			},
		},
	}

	log := zaptest.NewLogger(t)

	peer, err := versioncontrol.New(log, config)
	require.NoError(t, err)
	require.NotNil(t, peer)

	testCtx := testcontext.New(t)
	ctx, cancel := context.WithCancel(testCtx)

	var wg errgroup.Group
	wg.Go(func() error {
		return peer.Run(ctx)
	})

	defer testCtx.Check(peer.Close)
	defer cancel()

	baseURL := "http://" + peer.Addr()

	t.Run("resolve process url", func(t *testing.T) {
		queryTmpl := "processes/{service}/{version}/url?os={os}&arch={arch}"

		urls := make(map[string]string)
		for _, supportedBinary := range versioncontrol.SupportedBinaries {
			splitted := strings.SplitN(supportedBinary, "_", 3)

			service := splitted[0]
			os := splitted[1]
			arch := splitted[2]

			for _, versionType := range []string{"minimum", "suggested"} {
				query := strings.Replace(queryTmpl, "{service}", service, 1)
				query = strings.Replace(query, "{version}", versionType, 1)
				query = strings.Replace(query, "{os}", os, 1)
				query = strings.Replace(query, "{arch}", arch, 1)

				var url string
				switch versionType {
				case "minimum":
					url = createURL(service, minimumVersion)
				case "suggested":
					url = createURL(service, suggestedVersion)
				}

				url = strings.Replace(url, "{os}", os, 1)
				url = strings.Replace(url, "{arch}", arch, 1)
				urls[query] = url
			}
		}

		for query, url := range urls {
			query, url := query, url

			t.Run(query, func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/"+query, nil)
				require.NoError(t, err)
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, resp.StatusCode)

				b, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.NotNil(t, b)
				require.NoError(t, resp.Body.Close())

				require.Equal(t, url, string(b))
				log.Debug(string(b))
			})
		}
	})
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
				Address:  "127.0.0.1:0",
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
				Address:  "127.0.0.1:0",
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
					Address: "127.0.0.1:0",
					Binary:  versions,
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
		Cursor: testrand.Intn(101),
	}
}

func randSeedString(t *testing.T) string {
	t.Helper()

	seed := make([]byte, 32)
	testrand.Read(seed)
	return hex.EncodeToString(seed)
}
