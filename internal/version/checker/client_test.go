// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/version"
	"storj.io/storj/internal/version/checker"
	"storj.io/storj/versioncontrol"
)

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
	}
}

func newTestPeer(t *testing.T, ctx *testcontext.Context) *versioncontrol.Peer {
	t.Helper()

	testVersions := newTestVersions(t)
	serverConfig := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		Versions: versioncontrol.ServiceVersions{
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

func newTestVersions(t *testing.T) (versions versioncontrol.Versions) {
	t.Helper()

	versionsValue := reflect.ValueOf(&versions)
	versionsElem := versionsValue.Elem()
	fieldCount := versionsElem.NumField()

	for i := 0; i < fieldCount; i++ {
		field := versionsElem.Field(i)

		versionString := fmt.Sprintf("v%d.%d.%d", i+1, i+2, i+3)
		binary := versioncontrol.Binary{
			Minimum: versioncontrol.Version{
				Version: versionString,
			},
			Suggested: versioncontrol.Version{
				Version: versionString,
			},
		}

		field.Set(reflect.ValueOf(binary))
	}
	return versions
}
