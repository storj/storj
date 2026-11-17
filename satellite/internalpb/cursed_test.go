// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package internalpb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/internalpb"
	"storj.io/uplink/private/metaclient/cursed"
)

// TestCursedCompatibility verifies that the copy of StreamID and SegmentID in
// uplink encodes the same bytes as the definition here. The satellite signs the
// serialized form and uplink unpacks and repacks it, so the two have to agree.
func TestCursedCompatibility(t *testing.T) {
	created := time.Unix(1600000000, 12345).UTC()
	expires := time.Unix(1700000000, 54321).UTC()
	signature := testrand.BytesInt(64)
	pieceID := testrand.PieceID()
	satelliteID := testrand.NodeID()
	storageNodeID := testrand.NodeID()

	encryption := func() *pb.EncryptionParameters {
		return &pb.EncryptionParameters{CipherSuite: pb.CipherSuite_ENC_AESGCM, BlockSize: 4096}
	}
	limits := func() []*pb.AddressedOrderLimit {
		return []*pb.AddressedOrderLimit{{
			Limit: &pb.OrderLimit{
				SerialNumber:    storj.SerialNumber{1, 2, 3},
				SatelliteId:     satelliteID,
				StorageNodeId:   storageNodeID,
				PieceId:         pieceID,
				Limit:           1024,
				Action:          pb.PieceAction_PUT,
				PieceExpiration: expires,
				OrderExpiration: expires,
				OrderCreation:   created,
			},
			StorageNodeAddress: &pb.NodeAddress{Address: "example.test:7777"},
			Tags:               map[string][]byte{"tag": []byte("value")},
		}}
	}
	redundancy := func() *pb.RedundancyScheme {
		return &pb.RedundancyScheme{
			Type:             pb.RedundancyScheme_RS,
			MinReq:           29,
			Total:            80,
			RepairThreshold:  35,
			SuccessThreshold: 52,
			ErasureShareSize: 256,
		}
	}

	t.Run("StreamID", func(t *testing.T) {
		satellite := &internalpb.StreamID{
			Bucket:               []byte("bucket"),
			EncryptedObjectKey:   []byte("encrypted-object-key"),
			Version:              7,
			EncryptionParameters: encryption(),
			CreationDate:         created,
			ExpirationDate:       expires,
			MultipartObject:      true,
			SatelliteSignature:   signature,
			StreamId:             []byte("stream-id"),
			Placement:            3,
			Versioned:            true,
		}
		uplink := &cursed.StreamID{
			Bucket:               []byte("bucket"),
			EncryptedObjectKey:   []byte("encrypted-object-key"),
			Version:              7,
			EncryptionParameters: encryption(),
			CreationDate:         created,
			ExpirationDate:       expires,
			MultipartObject:      true,
			SatelliteSignature:   signature,
			StreamId:             []byte("stream-id"),
			Placement:            3,
			Versioned:            true,
		}

		requireSameEncoding(t, satellite, uplink, &internalpb.StreamID{})
	})

	t.Run("SegmentID", func(t *testing.T) {
		satellite := &internalpb.SegmentID{
			StreamId: &internalpb.StreamID{
				Bucket:             []byte("bucket"),
				EncryptedObjectKey: []byte("encrypted-object-key"),
				CreationDate:       created,
				ExpirationDate:     expires,
			},
			PartNumber:          2,
			Index:               5,
			RootPieceId:         pieceID,
			OriginalOrderLimits: limits(),
			CreationDate:        created,
			RedundancyScheme:    redundancy(),
			SatelliteSignature:  signature,
		}
		uplink := &cursed.SegmentID{
			StreamId: &cursed.StreamID{
				Bucket:             []byte("bucket"),
				EncryptedObjectKey: []byte("encrypted-object-key"),
				CreationDate:       created,
				ExpirationDate:     expires,
			},
			PartNumber:          2,
			Index:               5,
			RootPieceId:         pieceID,
			OriginalOrderLimits: limits(),
			CreationDate:        created,
			RedundancyScheme:    redundancy(),
			SatelliteSignature:  signature,
		}

		requireSameEncoding(t, satellite, uplink, &internalpb.SegmentID{})
	})

	// the fields that only the satellite side knows about must survive being
	// decoded and re-encoded by uplink, otherwise the signature stops
	// verifying whenever the satellite adds a field.
	t.Run("UnknownFields", func(t *testing.T) {
		satellite := &internalpb.StreamID{
			Bucket:       []byte("bucket"),
			CreationDate: created,
			Retention: &pb.Retention{
				Mode:        1,
				RetainUntil: expires,
			},
			LegalHold: true,
		}

		data, err := pb.Marshal(satellite)
		require.NoError(t, err)

		uplink := &cursed.StreamID{}
		require.NoError(t, pb.Unmarshal(data, uplink))
		require.NotEmpty(t, uplink.XXX_unrecognized)

		repacked, err := pb.Marshal(uplink)
		require.NoError(t, err)
		require.Equal(t, data, repacked)
	})
}

// requireSameEncoding requires that satellite and uplink encode to the same
// bytes, and that decoding those bytes into empty re-encodes to them as well.
func requireSameEncoding(t *testing.T, satellite, uplink, empty pb.Message) {
	t.Helper()

	satelliteData, err := pb.Marshal(satellite)
	require.NoError(t, err)
	uplinkData, err := pb.Marshal(uplink)
	require.NoError(t, err)
	require.Equal(t, satelliteData, uplinkData)

	require.NoError(t, pb.Unmarshal(uplinkData, empty))
	roundtripped, err := pb.Marshal(empty)
	require.NoError(t, err)
	require.Equal(t, satelliteData, roundtripped)
}
