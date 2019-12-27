// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/versioncontrol"
)

var testHexSeed = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestClient_All(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	peer := newTestPeer(t, ctx)
	defer ctx.Check(peer.Close)

	clientConfig := checker.ClientConfig{
		ServerAddress:  "http://" + peer.Addr(),
		RequestTimeout: 0,
	}
	client := checker.New(clientConfig)

	versions, err := client.All(ctx)
	require.NoError(t, err)

	processesValue := reflect.ValueOf(&versions.Processes)
	fieldCount := reflect.Indirect(processesValue).NumField()

	for i := 0; i < fieldCount; i++ {
		field := reflect.Indirect(processesValue).Field(i)

		versionString := fmt.Sprintf("v%d.%d.%d", i+1, i+2, i+3)

		process, ok := field.Interface().(version.Process)
		require.True(t, ok)
		require.Equal(t, versionString, process.Minimum.Version)
		require.Equal(t, versionString, process.Suggested.Version)
	}
}

func TestClient_Process(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	peer := newTestPeer(t, ctx)
	defer ctx.Check(peer.Close)

	clientConfig := checker.ClientConfig{
		ServerAddress:  "http://" + peer.Addr(),
		RequestTimeout: 0,
	}
	client := checker.New(clientConfig)

	processesType := reflect.TypeOf(version.Processes{})
	fieldCount := processesType.NumField()
	for i := 0; i < fieldCount; i++ {
		field := processesType.Field(i)

		expectedVersionStr := fmt.Sprintf("v%d.%d.%d", i+1, i+2, i+3)

		process, err := client.Process(ctx, field.Name)
		require.NoError(t, err)

		require.Equal(t, expectedVersionStr, process.Minimum.Version)
		require.Equal(t, expectedVersionStr, process.Suggested.Version)

		actualHexSeed := hex.EncodeToString(process.Rollout.Seed[:])
		require.NoError(t, err)

		require.Equal(t, testHexSeed, actualHexSeed)
		// TODO: find a better way to test this
		require.NotEmpty(t, process.Rollout.Cursor)
	}
}

func newTestPeer(t *testing.T, ctx *testcontext.Context) *versioncontrol.Peer {
	t.Helper()

	testVersions := newTestVersions(t)
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
	peer, err := versioncontrol.New(zaptest.NewLogger(t), serverConfig)
	require.NoError(t, err)

	ctx.Go(func() error {
		return peer.Run(ctx)
	})

	return peer
}

func newTestVersions(t *testing.T) (versions versioncontrol.ProcessesConfig) {
	t.Helper()

	versionsValue := reflect.ValueOf(&versions)
	versionsElem := versionsValue.Elem()
	fieldCount := versionsElem.NumField()

	for i := 0; i < fieldCount; i++ {
		field := versionsElem.Field(i)

		versionString := fmt.Sprintf("v%d.%d.%d", i+1, i+2, i+3)
		binary := versioncontrol.ProcessConfig{
			Minimum: versioncontrol.VersionConfig{
				Version: versionString,
			},
			Suggested: versioncontrol.VersionConfig{
				Version: versionString,
			},
			Rollout: versioncontrol.RolloutConfig{
				Seed:   testHexSeed,
				Cursor: 100,
			},
		}

		field.Set(reflect.ValueOf(binary))
	}
	return versions
}
