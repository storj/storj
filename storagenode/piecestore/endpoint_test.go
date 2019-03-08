// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package piecestore_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestEndpointConnect(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 0, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	client, err := planet.Uplinks[0].DialPiecestore(ctx, planet.StorageNodes[0])
	require.NoError(t, err)
	defer ctx.Check(client.Close)

	err = client.Delete(ctx, &pb.OrderLimit2{})
	t.Log(err)
	require.Error(t, err)
}
