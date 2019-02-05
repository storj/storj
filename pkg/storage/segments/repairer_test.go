// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"math/rand"
	"testing"
	time "time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	storj "storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestSegmentStoreRepairRemote(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		time.Sleep(2 * time.Second)

		// generate test data
		expectedData := make([]byte, 5*memory.MiB.Int())
		_, err := rand.Read(expectedData)
		assert.NoError(t, err)

		// upload test data
		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", expectedData)
		assert.NoError(t, err)

		// get uploaded data pointer
		var item storage.ListItem
		pointerdb := planet.Satellites[0].Metainfo.Service
		err = pointerdb.Iterate("", "", true, false,
			func(it storage.Iterator) error {
				// bucket
				it.Next(&item)
				// object
				it.Next(&item)
				return nil
			})
		pointer := &pb.Pointer{}
		err = proto.Unmarshal(item.Value, pointer)
		assert.NoError(t, err)

		unusedNodes := make(map[storj.NodeID]bool)
		for _, node := range planet.StorageNodes {
			unusedNodes[node.ID()] = true
		}

		const numberOfLostPieces = 2
		lostPieces := make([]int32, 0)
		oldNodes := make(map[storj.NodeID]bool)
		for _, piece := range pointer.Remote.RemotePieces {
			if piece != nil {
				// defines how many pieces will be marked as lost
				if len(lostPieces) < numberOfLostPieces {
					lostPieces = append(lostPieces, piece.PieceNum)
				} else {
					oldNodes[piece.NodeId] = true
				}
			}
			delete(unusedNodes, piece.NodeId)
		}

		// create segment repairer
		oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
		assert.NoError(t, err)
		pdb, err := planet.Uplinks[0].DialPointerDB(planet.Satellites[0], "")
		assert.NoError(t, err)
		ec := ecclient.NewClient(planet.Uplinks[0].Identity, 4*memory.MiB.Int())
		repairer := segments.NewSegmentRepairer(oc, ec, pdb)
		assert.NotNil(t, repairer)

		// repair
		err = repairer.Repair(ctx, string(item.Key), lostPieces)
		assert.NoError(t, err)

		updatedPointer, err := pointerdb.Get(string(item.Key))

		// check if unused nodes were used for storing repaired pieces
		newPieces := 0
		unknownePiece := 0
		for _, piece := range updatedPointer.Remote.RemotePieces {
			if piece != nil {
				if unusedNodes[piece.NodeId] {
					newPieces++
				} else if !oldNodes[piece.NodeId] {
					unknownePiece++
				}
			}
		}

		assert.True(t, newPieces > 0) // TODO more accurate assertion?
		assert.Equal(t, 0, unknownePiece)
		assert.Equal(t, len(updatedPointer.Remote.RemotePieces), len(oldNodes)+newPieces)

		// download and check if its equal to input data
		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test/bucket", "test/path")
		assert.NoError(t, err)

		assert.Equal(t, expectedData, data)
	})
}
