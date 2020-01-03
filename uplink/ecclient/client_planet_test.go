// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/uplink/ecclient"
	"storj.io/storj/uplink/eestream"
)

const (
	dataSize     = 32 * memory.KiB
	storageNodes = 4
)

func TestECClient(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: storageNodes, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		ec := ecclient.NewClient(planet.Uplinks[0].Log.Named("ecclient"), planet.Uplinks[0].Dialer, 0)

		k := storageNodes / 2
		n := storageNodes
		fc, err := infectious.NewFEC(k, n)
		require.NoError(t, err)

		es := eestream.NewRSScheme(fc, dataSize.Int()/n)
		rs, err := eestream.NewRedundancyStrategy(es, 0, 0)
		require.NoError(t, err)

		data, err := ioutil.ReadAll(io.LimitReader(testrand.Reader(), dataSize.Int64()))
		require.NoError(t, err)

		// Erasure encode some random data and upload the pieces
		successfulNodes, successfulHashes := testPut(ctx, t, planet, ec, rs, data)

		// Download the pieces and erasure decode the data
		testGet(ctx, t, planet, ec, es, data, successfulNodes, successfulHashes)

		// Delete the pieces
		testDelete(ctx, t, planet, ec, successfulNodes, successfulHashes)
	})
}

func testPut(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, rs eestream.RedundancyStrategy, data []byte) ([]*pb.Node, []*pb.PieceHash) {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	limits := make([]*pb.AddressedOrderLimit, rs.TotalCount())
	for i := 0; i < len(limits); i++ {
		limits[i], err = newAddressedOrderLimit(ctx, pb.PieceAction_PUT, planet.Satellites[0], piecePublicKey, planet.StorageNodes[i], storj.NewPieceID())
		require.NoError(t, err)
	}

	ttl := time.Now()

	r := bytes.NewReader(data)

	successfulNodes, successfulHashes, err := ec.Put(ctx, limits, piecePrivateKey, rs, r, ttl)

	require.NoError(t, err)
	assert.Equal(t, len(limits), len(successfulNodes))

	slowNodes := 0
	for i := range limits {
		if successfulNodes[i] == nil && limits[i] != nil {
			slowNodes++
		} else {
			assert.Equal(t, limits[i].GetLimit().StorageNodeId, successfulNodes[i].Id)
			if successfulNodes[i] != nil {
				assert.NotNil(t, successfulHashes[i])
				assert.Equal(t, limits[i].GetLimit().PieceId, successfulHashes[i].PieceId)
			}
		}
	}

	if slowNodes > rs.TotalCount()-rs.RequiredCount() {
		assert.Fail(t, fmt.Sprintf("Too many slow nodes: \n"+
			"expected: <= %d\n"+
			"actual  :    %d", rs.TotalCount()-rs.RequiredCount(), slowNodes))
	}

	return successfulNodes, successfulHashes
}

func testGet(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, es eestream.ErasureScheme, data []byte, successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash) {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	limits := make([]*pb.AddressedOrderLimit, es.TotalCount())
	for i := 0; i < len(limits); i++ {
		if successfulNodes[i] != nil {
			limits[i], err = newAddressedOrderLimit(ctx, pb.PieceAction_GET, planet.Satellites[0], piecePublicKey, planet.StorageNodes[i], successfulHashes[i].PieceId)
			require.NoError(t, err)
		}
	}

	rr, err := ec.Get(ctx, limits, piecePrivateKey, es, dataSize.Int64())
	require.NoError(t, err)

	r, err := rr.Range(ctx, 0, rr.Size())
	require.NoError(t, err)
	readData, err := ioutil.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, data, readData)
	assert.NoError(t, r.Close())
	require.NoError(t, err)
}

func testDelete(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, successfulNodes []*pb.Node, successfulHashes []*pb.PieceHash) {
	piecePublicKey, piecePrivateKey, err := storj.NewPieceKey()
	require.NoError(t, err)

	limits := make([]*pb.AddressedOrderLimit, len(successfulNodes))
	for i := 0; i < len(limits); i++ {
		if successfulNodes[i] != nil {
			limits[i], err = newAddressedOrderLimit(ctx, pb.PieceAction_DELETE, planet.Satellites[0], piecePublicKey, planet.StorageNodes[i], successfulHashes[i].PieceId)
			require.NoError(t, err)
		}
	}

	err = ec.Delete(ctx, limits, piecePrivateKey)

	require.NoError(t, err)
}

func newAddressedOrderLimit(ctx context.Context, action pb.PieceAction, satellite *testplanet.SatelliteSystem, piecePublicKey storj.PiecePublicKey, storageNode *storagenode.Peer, pieceID storj.PieceID) (*pb.AddressedOrderLimit, error) {
	// TODO refactor to avoid OrderLimit duplication
	serialNumber := testrand.SerialNumber()

	now := time.Now()

	limit := &pb.OrderLimit{
		SerialNumber:    serialNumber,
		SatelliteId:     satellite.ID(),
		UplinkPublicKey: piecePublicKey,
		StorageNodeId:   storageNode.ID(),
		PieceId:         pieceID,
		Action:          action,
		Limit:           dataSize.Int64(),
		PieceExpiration: time.Time{},
		OrderCreation:   now,
		OrderExpiration: now.Add(24 * time.Hour),
	}

	limit, err := signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satellite.Identity), limit)
	if err != nil {
		return nil, err
	}

	return &pb.AddressedOrderLimit{
		StorageNodeAddress: storageNode.Local().Address,
		Limit:              limit,
	}, nil
}
