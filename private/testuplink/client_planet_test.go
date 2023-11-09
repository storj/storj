// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package testuplink_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/internalpb"
	"storj.io/uplink/private/ecclient"
	"storj.io/uplink/private/eestream"
)

const (
	dataSize     = 32 * memory.KiB
	storageNodes = 4
)

func TestECClient(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: storageNodes, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		ec := ecclient.New(planet.Uplinks[0].Dialer, 0)

		k := storageNodes / 2
		n := storageNodes
		fc, err := eestream.NewFEC(k, n)
		require.NoError(t, err)

		es := eestream.NewRSScheme(fc, dataSize.Int()/n)
		rs, err := eestream.NewRedundancyStrategy(es, 0, 0)
		require.NoError(t, err)

		data, err := io.ReadAll(io.LimitReader(testrand.Reader(), dataSize.Int64()))
		require.NoError(t, err)

		// Erasure encode some random data and upload the pieces
		results := testPut(ctx, t, planet, ec, rs, data)

		// Download the pieces and erasure decode the data
		testGet(ctx, t, planet, ec, es, data, results)
	})
}

func testPut(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, rs eestream.RedundancyStrategy, data []byte) []*pb.SegmentPieceUploadResult {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	limits := make([]*pb.AddressedOrderLimit, rs.TotalCount())
	for i := 0; i < len(limits); i++ {
		limits[i], err = newAddressedOrderLimit(ctx, pb.PieceAction_PUT, planet.Satellites[0], piecePublicKey, planet.StorageNodes[i], storj.NewPieceID())
		require.NoError(t, err)
	}

	r := bytes.NewReader(data)

	results, err := ec.PutSingleResult(ctx, limits, piecePrivateKey, rs, r)

	require.NoError(t, err)
	assert.Equal(t, len(limits), len(results))

	slowNodes := 0
	for i := range limits {
		if results[i] == nil && limits[i] != nil {
			slowNodes++
		} else {
			assert.Equal(t, limits[i].GetLimit().StorageNodeId, results[i].NodeId)
			if results[i] != nil {
				assert.NotNil(t, results[i].Hash)
				assert.Equal(t, limits[i].GetLimit().PieceId, results[i].Hash.PieceId)
			}
		}
	}

	if slowNodes > rs.TotalCount()-rs.RequiredCount() {
		assert.Fail(t, fmt.Sprintf("Too many slow nodes: \n"+
			"expected: <= %d\n"+
			"actual  :    %d", rs.TotalCount()-rs.RequiredCount(), slowNodes))
	}

	return results
}

func testGet(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, es eestream.ErasureScheme, data []byte, results []*pb.SegmentPieceUploadResult) {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	limits := make([]*pb.AddressedOrderLimit, es.TotalCount())
	for i := 0; i < len(limits); i++ {
		if results[i] != nil {
			limits[i], err = newAddressedOrderLimit(ctx, pb.PieceAction_GET, planet.Satellites[0], piecePublicKey, planet.StorageNodes[i], results[i].Hash.PieceId)
			require.NoError(t, err)
		}
	}

	rr, err := ec.Get(ctx, limits, piecePrivateKey, es, dataSize.Int64())
	require.NoError(t, err)

	r, err := rr.Range(ctx, 0, rr.Size())
	require.NoError(t, err)
	readData, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, data, readData)
	assert.NoError(t, r.Close())
	require.NoError(t, err)
}

func newAddressedOrderLimit(ctx context.Context, action pb.PieceAction, satellite *testplanet.Satellite, piecePublicKey storj.PiecePublicKey, storageNode *testplanet.StorageNode, pieceID storj.PieceID) (*pb.AddressedOrderLimit, error) {
	// TODO refactor to avoid OrderLimit duplication
	serialNumber := testrand.SerialNumber()

	now := time.Now()
	key := satellite.Config.Orders.EncryptionKeys.Default
	encrypted, err := key.EncryptMetadata(
		serialNumber,
		&internalpb.OrderLimitMetadata{
			CompactProjectBucketPrefix: []byte("0000111122223333testbucketname"),
		},
	)
	if err != nil {
		return nil, err
	}

	limit := &pb.OrderLimit{
		SerialNumber:           serialNumber,
		SatelliteId:            satellite.ID(),
		UplinkPublicKey:        piecePublicKey,
		StorageNodeId:          storageNode.ID(),
		PieceId:                pieceID,
		Action:                 action,
		Limit:                  dataSize.Int64(),
		PieceExpiration:        time.Time{},
		OrderCreation:          now,
		OrderExpiration:        now.Add(24 * time.Hour),
		EncryptedMetadataKeyId: key.ID[:],
		EncryptedMetadata:      encrypted,
	}

	limit, err = signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit)
	if err != nil {
		return nil, err
	}

	return &pb.AddressedOrderLimit{
		StorageNodeAddress: &pb.NodeAddress{Address: storageNode.Addr()},
		Limit:              limit,
	}, nil
}
