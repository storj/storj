// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
)

func TestSatelliteContactEndpoint(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satDossier := planet.Satellites[0].Local()
		nodeDossier := planet.StorageNodes[0].Local()

		conn, err := planet.StorageNodes[0].Transport.DialNode(ctx, &satDossier.Node)
		require.NoError(t, err)
		defer ctx.Check(conn.Close)

		resp, err := pb.NewNodeClient(conn).Checkin(ctx, &pb.CheckinRequest{
			Address:  nodeDossier.GetAddress(),
			Capacity: &nodeDossier.Capacity,
			Operator: &nodeDossier.Operator,
		})
		require.NotNil(t, resp)
		require.NoError(t, err)
	})
}
