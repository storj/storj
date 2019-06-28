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

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
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
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		projects, err1 := planet.Satellites[0].DB.Console().Projects().GetAll(ctx)
		if err1 != nil {
			assert.NoError(t, err1)
		}
		projectID := projects[0].ID

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
		expectedTally := accounting.BucketTally{
			BucketName:     []byte(expectedBucketName),
			ProjectID:      projectID[:],
			Segments:       1,
			InlineSegments: 1,
			Files:          1,
			InlineFiles:    1,
			Bytes:          int64(expectedTotalBytes),
			InlineBytes:    int64(expectedTotalBytes),
			MetadataSize:   111, // brittle, this is hardcoded since its too difficult to get this value progamatically
		}
		// The projectID should be the 16 bytes uuid representation, not 36 byte string representation
		assert.Equal(t, 16, len(projectID[:]))

		// Execute test: upload a file, then calculate at rest data
		err := uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)
		assert.NoError(t, err)

		// Run calculate twice to test unique constraint issue
		for i := 0; i < 2; i++ {
			latestTally, actualNodeData, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
			require.NoError(t, err)
			assert.Len(t, actualNodeData, 0)

			_, err = planet.Satellites[0].DB.ProjectAccounting().SaveTallies(ctx, latestTally, actualBucketData)
			require.NoError(t, err)

			// Confirm the correct bucket storage tally was created
			assert.Equal(t, len(actualBucketData), 1)
			for bucketID, actualTally := range actualBucketData {
				assert.Contains(t, bucketID, expectedBucketName)
				assert.Equal(t, expectedTally, *actualTally)
			}
		}
	})
}

func TestCalculateNodeAtRestData(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		tallySvc := planet.Satellites[0].Accounting.Tally
		uplink := planet.Uplinks[0]

		// Setup: create 50KiB of data for the uplink to upload
		expectedData := testrand.Bytes(50 * memory.KiB)

		// Setup: get the expected size of the data that will be stored in pointer
		uplinkConfig := uplink.GetConfig(planet.Satellites[0])
		expectedTotalBytes, err := encryption.CalcEncryptedSize(int64(len(expectedData)), uplinkConfig.GetEncryptionScheme())
		require.NoError(t, err)

		// Execute test: upload a file, then calculate at rest data
		expectedBucketName := "testbucket"
		err = uplink.Upload(ctx, planet.Satellites[0], expectedBucketName, "test/path", expectedData)

		assert.NoError(t, err)
		_, actualNodeData, _, err := tallySvc.CalculateAtRestData(ctx)
		require.NoError(t, err)

		// Confirm the correct number of shares were stored
		uplinkRS := uplinkConfig.GetRedundancyScheme()
		if !correctRedundencyScheme(len(actualNodeData), uplinkRS) {
			t.Fatalf("expected between: %d and %d, actual: %d", uplinkRS.RepairShares, uplinkRS.TotalShares, len(actualNodeData))
		}

		// Confirm the correct number of bytes were stored on each node
		for _, actualTotalBytes := range actualNodeData {
			assert.Equal(t, int64(actualTotalBytes), expectedTotalBytes)
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
		{"bucket, no objects", "9656af6e-2d9c-42fa-91f2-bfd516a722d7", "", "mockBucketName", "", true, false},
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
			tt := tt // avoid scopelint error, ref: https://github.com/golangci/golangci-lint/issues/281

			t.Run(tt.name, func(t *testing.T) {
				projectID, err := uuid.Parse(tt.project)
				require.NoError(t, err)

				// setup: create a pointer and save it to pointerDB
				pointer, _ := makePointer(planet.StorageNodes, redundancyScheme, int64(2), tt.inline)
				metainfo := satellitePeer.Metainfo.Service
				objectPath := fmt.Sprintf("%s/%s/%s/%s", tt.project, tt.segmentIndex, tt.bucketName, tt.objectName)
				if tt.objectName == "" {
					objectPath = fmt.Sprintf("%s/%s/%s", tt.project, tt.segmentIndex, tt.bucketName)
				}
				err = metainfo.Put(ctx, objectPath, pointer)
				require.NoError(t, err)

				// setup: create expected bucket tally for the pointer just created, but only if
				// the pointer was for an object and not just for a bucket
				if tt.objectName != "" {
					bucketID := fmt.Sprintf("%s/%s", tt.project, tt.bucketName)
					newTally := addBucketTally(expectedBucketTallies[bucketID], tt.inline, tt.last)
					newTally.BucketName = []byte(tt.bucketName)
					newTally.ProjectID = projectID[:]
					expectedBucketTallies[bucketID] = newTally
				}

				// test: calculate at rest data
				tallySvc := satellitePeer.Accounting.Tally
				_, _, actualBucketData, err := tallySvc.CalculateAtRestData(ctx)
				require.NoError(t, err)

				assert.Equal(t, len(expectedBucketTallies), len(actualBucketData))
				for bucket, actualTally := range actualBucketData {
					assert.Equal(t, *expectedBucketTallies[bucket], *actualTally)
				}
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
		existingTally.Segments++
		existingTally.Bytes += int64(2)
		existingTally.MetadataSize += int64(12)
		existingTally.RemoteSegments++
		existingTally.RemoteBytes += int64(2)
		return existingTally
	}

	// if the pointer was inline, create a tally with inline info
	if inline {
		newInlineTally := accounting.BucketTally{
			Segments:       int64(1),
			InlineSegments: int64(1),
			Files:          int64(1),
			InlineFiles:    int64(1),
			Bytes:          int64(2),
			InlineBytes:    int64(2),
			MetadataSize:   int64(12),
		}
		return &newInlineTally
	}

	// if the pointer was remote, create a tally with remote info
	newRemoteTally := accounting.BucketTally{
		Segments:       int64(1),
		RemoteSegments: int64(1),
		Bytes:          int64(2),
		RemoteBytes:    int64(2),
		MetadataSize:   int64(12),
	}

	if last {
		newRemoteTally.Files++
		newRemoteTally.RemoteFiles++
	}

	return &newRemoteTally
}

// makePointer creates a pointer
func makePointer(storageNodes []*storagenode.Peer, rs storj.RedundancyScheme,
	segmentSize int64, inline bool) (*pb.Pointer, error) {

	if inline {
		inlinePointer := &pb.Pointer{
			Type:          pb.Pointer_INLINE,
			InlineSegment: make([]byte, segmentSize),
			SegmentSize:   segmentSize,
			Metadata:      []byte("fakemetadata"),
		}
		return inlinePointer, nil
	}

	pieces := make([]*pb.RemotePiece, 0, len(storageNodes))
	for i, storagenode := range storageNodes {
		pieces = append(pieces, &pb.RemotePiece{
			PieceNum: int32(i),
			NodeId:   storagenode.ID(),
		})
	}

	pointer := &pb.Pointer{
		Type: pb.Pointer_REMOTE,
		Remote: &pb.RemoteSegment{
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

	return pointer, nil
}

func correctRedundencyScheme(shareCount int, uplinkRS storj.RedundancyScheme) bool {

	// The shareCount should be a value between RequiredShares and TotalShares where
	// RequiredShares is the min number of shares required to recover a segment and
	// TotalShares is the number of shares to encode
	if int(uplinkRS.RepairShares) <= shareCount && shareCount <= int(uplinkRS.TotalShares) {
		return true
	}

	return false
}
