// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
)

func TestResolvePartnerID(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		endpoint := planet.Satellites[0].Metainfo.Endpoint2

		zenkoPartnerID, err := uuid.FromString("8cd605fa-ad00-45b6-823e-550eddc611d6")
		require.NoError(t, err)

		// no header
		_, err = endpoint.ResolvePartnerID(ctx, nil, []byte{1, 2, 3})
		require.Error(t, err)

		// bad uuid
		_, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{}, []byte{1, 2, 3})
		require.Error(t, err)

		randomUUID := testrand.UUID()

		// good uuid
		result, err := endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{}, randomUUID[:])
		require.NoError(t, err)
		require.Equal(t, randomUUID, result)

		_, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("not-a-partner"),
		}, nil)
		require.Error(t, err)

		partnerID, err := endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("Zenko uplink/v1.0.0 (drpc/v0.10.0) (common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, zenkoPartnerID, partnerID)

		partnerID, err = endpoint.ResolvePartnerID(ctx, &pb.RequestHeader{
			UserAgent: []byte("uplink/v1.0.0 (drpc/v0.10.0 common/v0.0.0-00010101000000-000000000000)"),
		}, nil)
		require.NoError(t, err)
		require.Equal(t, uuid.UUID{}, partnerID)
	})
}
