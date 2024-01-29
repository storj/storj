// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/versioncontrol"
)

func TestVersion(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	minimum := "v1.89.5"
	suggested := "v1.90.2"

	testVersions := newTestVersions(t)
	testVersions.Storagenode.Minimum.Version = minimum
	testVersions.Storagenode.Suggested.Version = suggested

	serverConfig := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		Versions: versioncontrol.OldVersionConfig{
			Satellite:   "v0.0.1",
			Storagenode: "v0.0.1",
			Uplink:      "v0.0.1",
			Gateway:     "v0.0.1",
			Identity:    "v0.0.1",
		},
		Binary: testVersions,
	}

	peer := newTestPeerWithConfig(t, ctx, serverConfig)
	defer ctx.Check(peer.Close)

	clientConfig := checker.ClientConfig{
		ServerAddress:  "http://" + peer.Addr(),
		RequestTimeout: 0,
	}
	config := checker.Config{
		ClientConfig: clientConfig,
	}

	t.Run("CheckVersion", func(t *testing.T) {
		type args struct {
			name              string
			version           string
			errorMsg          string
			isAcceptedVersion bool
		}

		tests := []args{
			{
				name:              "runs outdated version",
				version:           "1.80.0",
				errorMsg:          "outdated software version (v1.80.0), please update",
				isAcceptedVersion: false,
			},
			{
				name:              "runs minimum version",
				version:           minimum,
				isAcceptedVersion: true,
			},
			{
				name:              "runs suggested version",
				version:           suggested,
				isAcceptedVersion: true,
			},
			{
				name:              "runs version newer than minimum",
				version:           "v1.90.2",
				isAcceptedVersion: true,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ver, err := version.NewSemVer(test.version)
				require.NoError(t, err)

				versionInfo := version.Info{
					Version: ver,
					Release: true,
				}

				service := checker.NewService(zaptest.NewLogger(t), config, versionInfo, "storagenode")
				latest, err := service.CheckVersion(ctx)
				if test.errorMsg != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), test.errorMsg)
				} else {
					require.NoError(t, err)
				}

				require.Equal(t, suggested, latest.String())

				minVersion, isAllowed := service.IsAllowed(ctx)
				require.Equal(t, isAllowed, test.isAcceptedVersion)
				require.Equal(t, minimum, minVersion.String())
			})
		}
	})
}
