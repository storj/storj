// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tally_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/private/teststorj"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/accounting/tally"
	"storj.io/storj/storagenode"
)

func TestDeleteTalliesBefore(t *testing.T) {
	tests := []struct {
		eraseBefore  time.Time
		expectedRaws int
	}{
		{
			eraseBefore:  time.Now(),
			expectedRaws: 1,
		},
		{
			eraseBefore:  time.Now().Add(24 * time.Hour),
			expectedRaws: 0,
		},
	}

	for _, tt := range tests {
		test := tt
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			id := teststorj.NodeIDFromBytes([]byte{})
			nodeData := make(map[storj.NodeID]float64)
			nodeData[id] = float64(1000)

			err := planet.Satellites[0].DB.StoragenodeAccounting().SaveTallies(ctx, time.Now(), nodeData)
			require.NoError(t, err)

			err = planet.Satellites[0].DB.StoragenodeAccounting().DeleteTalliesBefore(ctx, test.eraseBefore)
			require.NoError(t, err)

			raws, err := planet.Satellites[0].DB.StoragenodeAccounting().GetTallies(ctx)
			require.NoError(t, err)
			assert.Len(t, raws, test.expectedRaws)
		})
	}
}

func TestOnlyInline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		planet.Satellites[0].Accounting.Tally.Loop.Pause()
		uplink := planet.Uplinks[0]
		projectID := planet.Uplinks[0].ProjectID[planet.Satellites[0].ID()]

		// Setup: create data for the uplink to upload
		expectedData := testrand.Bytes(1 * memory.KiB)

		// Setup: get the expected size of the data that will be stored in pointer
		// Since the data is small enough to be stored inline, when it is encrypted, we only
		// add 16 bytes of encryption authentication overhead.  No encryption block
		// padding will be added since we are not chunking data that we store inline.
		const encryptionAuthOverhead = 16 // bytes
		expectedTotalBytes := len(expectedData) + encryptionAuthOverhead

		// Setup: The data in this tally should match the pointer that the uplink.upload created
		expectedBucketName := "testbucket"
		expectedTally := &accounting.BucketTally{
			BucketName:     []byte(expectedBucketName),
			ProjectID:      projectID,
			ObjectCount:    1,
			InlineSegments: 1,
			InlineBytes:    int64(expectedTotalBytes),
			MetadataSize:   113, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}

		// Execute test: upload a file, then calculate at rest data
		err := uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)

		// run multiple times to ensure we add tallies
		for i := 0; i < 2; i++ {
			obs := tally.NewObserver(planet.Satellites[0].Log.Named("observer"))
			err := planet.Satellites[0].Metainfo.Loop.Join(ctx, obs)
			require.NoError(t, err)

			now := time.Now().Add(time.Duration(i) * time.Second)
			err = planet.Satellites[0].DB.ProjectAccounting().SaveTallies(ctx, now, obs.Bucket)
			require.NoError(t, err)

			assert.Equal(t, 1, len(obs.Bucket))
			for _, actualTally := range obs.Bucket {
				assert.Equal(t, expectedTally, actualTally)
			}
		}
	})
}

func TestCalculateNodeAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		tallySvc.Loop.Pause()
		uplink := planet.Uplinks[0]

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Setup: get the expected size of the data that will be stored in pointer
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), uplinkConfig.GetEncryptionParameters())
		require.NoError(t, err)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		require.NoError(t, err)

		obs := tally.NewObserver(planet.Satellites[0].Log.Named("observer"))
		err = planet.Satellites[0].Metainfo.Loop.Join(ctx, obs)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		uplinkRS := uplinkConfig.GetRedundancyScheme()
		if !correctRedundencyScheme(len(obs.Node), uplinkRS) {
			t.Fatalf("expected between: %d and %d, actual: %d", uplinkRS.RepairShares, uplinkRS.TotalShares, len(obs.Node))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range obs.Node {
			assert.Equal(t, expectedTotalBytes, int64(actualTotalBytes))
		}
	})
}

func TestCalculateBucketAtRestData(t *testing.T) {
	var testCases = []struct {
		name         string
		project      string
		segmentIndex string
		bucketName   string
		objectName   string
		inline       bool
		last         bool
	}{
		{"inline, same project, same bucket", "9656af6e-2d9c-42fa-91f2-bfd516a722d7", "l", "mockBucketName", "mockObjectName", true, true},
		{"remote, same project, same bucket", "9656af6e-2d9c-42fa-91f2-bfd516a722d7", "s0", "mockBucketName", "mockObjectName1", false, false},
		{"last segment, same project, different bucket", "9656af6e-2d9c-42fa-91f2-bfd516a722d7", "l", "mockBucketName1", "mockObjectName2", false, true},
		{"different project", "9656af6e-2d9c-42fa-91f2-bfd516a722d1", "s0", "mockBucketName", "mockObjectName", false, false},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		redundancyScheme := planet.Uplinks[0].GetConfig(satellitePeer).GetRedundancyScheme()
		expectedBucketTallies := make(map[string]*accounting.BucketTally)
		for _, tt := range testCases {
			tt := tt // avoid scopelint error

			t.Run(tt.name, func(t *testing.T) {
				projectID, err := uuid.Parse(tt.project)
				require.NoError(t, err)

				// setup: create a pointer and save it to pointerDB
				pointer := makePointer(planet.StorageNodes, redundancyScheme, int64(2), tt.inline)
				metainfo := satellitePeer.Metainfo.Service
				objectPath := fmt.Sprintf("%s/%s/%s/%s", tt.project, tt.segmentIndex, tt.bucketName, tt.objectName)
				err = metainfo.Put(ctx, objectPath, pointer)
				require.NoError(t, err)

				bucketID := fmt.Sprintf("%s/%s", tt.project, tt.bucketName)
				newTally := addBucketTally(expectedBucketTallies[bucketID], tt.inline, tt.last)
				newTally.BucketName = []byte(tt.bucketName)
				newTally.ProjectID = *projectID
				expectedBucketTallies[bucketID] = newTally

				obs := tally.NewObserver(satellitePeer.Log.Named("observer"))
				err = satellitePeer.Metainfo.Loop.Join(ctx, obs)
				require.NoError(t, err)
				require.Equal(t, expectedBucketTallies, obs.Bucket)
			})
		}
	})
}

// addBucketTally creates a new expected bucket tally based on the
// pointer that was just created for the test case
func addBucketTally(existingTally *accounting.BucketTally, inline, last bool) *accounting.BucketTally {
	// if there is already an existing tally for this project and bucket, then
	// add the new pointer data to the existing tally
	if existingTally != nil {
		existingTally.MetadataSize += int64(12)
		existingTally.RemoteSegments++
		existingTally.RemoteBytes += int64(2)
		return existingTally
	}

	// if the pointer was inline, create a tally with inline info
	if inline {
		return &accounting.BucketTally{
			ObjectCount:    int64(1),
			InlineSegments: int64(1),
			InlineBytes:    int64(2),
			MetadataSize:   int64(12),
		}
	}

	// if the pointer was remote, create a tally with remote info
	newRemoteTally := &accounting.BucketTally{
		RemoteSegments: int64(1),
		RemoteBytes:    int64(2),
		MetadataSize:   int64(12),
	}

	if last {
		newRemoteTally.ObjectCount++
	}

	return newRemoteTally
}

// makePointer creates a pointer
func makePointer(storageNodes []*storagenode.Peer, rs storj.RedundancyScheme, segmentSize int64, inline bool) *pb.Pointer {
	if inline {
		inlinePointer := &pb.Pointer{
			CreationDate:  time.Now(),
			Type:          pb.Pointer_INLINE,
			InlineSegment: make([]byte, segmentSize),
			SegmentSize:   segmentSize,
			Metadata:      []byte("fakemetadata"),
		}
		return inlinePointer
	}

	pieces := make([]*pb.RemotePiece, rs.TotalShares)
	for i := range pieces {
		pieces[i] = &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   storageNodes[i].ID(),
		}
	}

	return &pb.Pointer{
		CreationDate: time.Now(),
		Type:         pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
			RootPieceId: storj.PieceID{0xFF},
			Redundancy: &pb.RedundancyScheme{
				Type:             pb.RedundancyScheme_RS,
				MinReq:           int32(rs.RequiredShares),
				Total:            int32(rs.TotalShares),
				RepairThreshold:  int32(rs.RepairShares),
				SuccessThreshold: int32(rs.OptimalShares),
				ErasureShareSize: rs.ShareSize,
			},
			RemotePieces: pieces,
		},
		SegmentSize: segmentSize,
		Metadata:    []byte("fakemetadata"),
	}
}

func correctRedundencyScheme(shareCount int, uplinkRS storj.RedundancyScheme) bool {
	// The shareCount should be a value between RequiredShares and TotalShares where
	// RequiredShares is the min number of shares required to recover a segment and
	// TotalShares is the number of shares to encode
	return int(uplinkRS.RepairShares) <= shareCount && shareCount <= int(uplinkRS.TotalShares)
}
