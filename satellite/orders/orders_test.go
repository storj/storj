// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestOrders(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, storageNode := range planet.StorageNodes {
		storageNode.Storage2.Sender.Loop.Pause()
	}

	expectedData := make([]byte, 50*memory.KiB)
	_, err = rand.Read(expectedData)
	require.NoError(t, err)

	err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
	require.NoError(t, err)

	sumBeforeSend := 0
	for _, storageNode := range planet.StorageNodes {
		infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		sumBeforeSend += len(infos)
	}
	require.NotZero(t, sumBeforeSend)

	sumUnsent := 0
	sumArchived := 0

	for _, storageNode := range planet.StorageNodes {
		storageNode.Storage2.Sender.Loop.TriggerWait()

		infos, err := storageNode.DB.Orders().ListUnsent(ctx, 10)
		require.NoError(t, err)
		sumUnsent += len(infos)

		archivedInfos, err := storageNode.DB.Orders().ListArchived(ctx, sumBeforeSend)
		require.NoError(t, err)
		sumArchived += len(archivedInfos)
	}

	require.Zero(t, sumUnsent)
	require.Equal(t, sumBeforeSend, sumArchived)
}
