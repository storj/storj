// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package ecclient_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vivint/infectious"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/auth/signing"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

const (
	dataSize     = 32 * memory.KiB
	storageNodes = 4
)

func TestECClient(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, storageNodes, 1)
	require.NoError(t, err)

	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	ec := ecclient.NewClient(planet.Uplinks[0].Transport, 0)

	k := storageNodes / 2
	n := storageNodes
	fc, err := infectious.NewFEC(k, n)
	require.NoError(t, err)

	es := eestream.NewRSScheme(fc, dataSize.Int()/n)
	rs, err := eestream.NewRedundancyStrategy(es, 0, 0)
	require.NoError(t, err)

	data, err := ioutil.ReadAll(io.LimitReader(rand.Reader, dataSize.Int64()))
	require.NoError(t, err)

	// Erasure encode some random data and upload the pieces
	successfulNodes, successfulHashes := testPut(ctx, t, planet, ec, rs, data)

	// Download the pieces and erasure decode the data
	testGet(ctx, t, planet, ec, es, data, successfulNodes, successfulHashes)

	// Delete the pieces
	testDelete(ctx, t, planet, ec, successfulNodes, successfulHashes)
}

func testPut(ctx context.Context, t *testing.T, planet *testplanet.Planet, ec ecclient.Client, rs eestream.RedundancyStrategy, data []byte) ([]*pb.Node, []*pb.PieceHash) {
	var err error
	limits := make([]*pb.AddressedOrderLimit, rs.TotalCount())
	for i := 0; i < len(limits); i++ {
		limits[i], err = newAddressedOrderLimit(pb.PieceAction_PUT, planet.Satellites[0], planet.Uplinks[0], planet.StorageNodes[i], storj.NewPieceID())
		require.NoError(t, err)
	}

	ttl := time.Now()

	r := bytes.NewReader(data)

	successfulNodes, successfulHashes, err := ec.Put(ctx, limits, rs, r, ttl)

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
	var err error
	limits := make([]*pb.AddressedOrderLimit, es.TotalCount())
	for i := 0; i < len(limits); i++ {
		if successfulNodes[i] != nil {
			limits[i], err = newAddressedOrderLimit(pb.PieceAction_GET, planet.Satellites[0], planet.Uplinks[0], planet.StorageNodes[i], successfulHashes[i].PieceId)
			require.NoError(t, err)
		}
	}

	rr, err := ec.Get(ctx, limits, es, dataSize.Int64())
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
	var err error
	limits := make([]*pb.AddressedOrderLimit, len(successfulNodes))
	for i := 0; i < len(limits); i++ {
		if successfulNodes[i] != nil {
			limits[i], err = newAddressedOrderLimit(pb.PieceAction_DELETE, planet.Satellites[0], planet.Uplinks[0], planet.StorageNodes[i], successfulHashes[i].PieceId)
			require.NoError(t, err)
		}
	}

	err = ec.Delete(ctx, limits)

	require.NoError(t, err)
}

func newAddressedOrderLimit(action pb.PieceAction, satellite *satellite.Peer, uplink *testplanet.Uplink, storageNode *storagenode.Peer, pieceID storj.PieceID) (*pb.AddressedOrderLimit, error) {
	// TODO refactor to avoid OrderLimit duplication
	serialNumber, err := uuid.New()
	if err != nil {
		return nil, err
	}

	limit := &pb.OrderLimit2{
		SerialNumber:    storj.SerialNumber(*serialNumber),
		SatelliteId:     satellite.ID(),
		UplinkId:        uplink.ID(),
		StorageNodeId:   storageNode.ID(),
		PieceId:         pieceID,
		Action:          action,
		Limit:           dataSize.Int64(),
		PieceExpiration: new(timestamp.Timestamp),
		OrderExpiration: new(timestamp.Timestamp),
	}

	limit, err = signing.SignOrderLimit(signing.SignerFromFullIdentity(satellite.Identity), limit)
	if err != nil {
		return nil, err
	}

	return &pb.AddressedOrderLimit{
		StorageNodeAddress: storageNode.Local().Address,
		Limit:              limit,
	}, nil
}
