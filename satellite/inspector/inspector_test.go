// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/base58"
	"storj.io/common/encryption"
	"storj.io/common/memory"
	"storj.io/common/paths"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/internalpb"
	"storj.io/storj/satellite/metabase"
	"storj.io/uplink/private/eestream"
)

func TestInspectorStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]
		upl := planet.Uplinks[0]
		testData := testrand.Bytes(1 * memory.MiB)

		bucket := "testbucket"
		projectID := upl.Projects[0].ID

		err := upl.Upload(ctx, planet.Satellites[0], bucket, "test/path", testData)
		require.NoError(t, err)

		healthEndpoint := planet.Satellites[0].Inspector.Endpoint

		// Get path of random segment we just uploaded and check the health
		access := upl.Access[satellite.ID()]
		serializedAccess, err := access.Serialize()
		require.NoError(t, err)

		store, err := encryptionAccess(serializedAccess)
		require.NoError(t, err)

		encryptedPath, err := encryption.EncryptPathWithStoreCipher(bucket, paths.NewUnencrypted("test/path"), store)
		require.NoError(t, err)

		objectLocation := metabase.ObjectLocation{
			ProjectID:  projectID,
			BucketName: "testbucket",
			ObjectKey:  metabase.ObjectKey(encryptedPath.Raw()),
		}

		segment, err := satellite.Metabase.DB.GetLatestObjectLastSegment(ctx, metabase.GetLatestObjectLastSegment{
			ObjectLocation: objectLocation,
		})
		require.NoError(t, err)

		{ // Test Segment Health Request
			req := &internalpb.SegmentHealthRequest{
				ProjectId:     projectID[:],
				EncryptedPath: []byte(encryptedPath.Raw()),
				Bucket:        []byte(bucket),
				SegmentIndex:  int64(segment.Position.Encode()),
			}

			resp, err := healthEndpoint.SegmentHealth(ctx, req)
			require.NoError(t, err)

			redundancy, err := eestream.NewRedundancyStrategyFromProto(resp.GetRedundancy())
			require.NoError(t, err)

			require.Equal(t, 4, redundancy.TotalCount())
			encodedPosition := binary.LittleEndian.Uint64(resp.GetHealth().GetSegment())
			position := metabase.SegmentPositionFromEncoded(encodedPosition)
			require.Equal(t, segment.Position, position)
		}

		{ // Test Object Health Request
			objectHealthReq := &internalpb.ObjectHealthRequest{
				ProjectId:         projectID[:],
				EncryptedPath:     []byte(encryptedPath.Raw()),
				Bucket:            []byte(bucket),
				StartAfterSegment: 0,
				EndBeforeSegment:  0,
				Limit:             0,
			}
			resp, err := healthEndpoint.ObjectHealth(ctx, objectHealthReq)
			require.NoError(t, err)

			segments := resp.GetSegments()
			require.Len(t, segments, 1)

			redundancy, err := eestream.NewRedundancyStrategyFromProto(resp.GetRedundancy())
			require.NoError(t, err)

			require.Equal(t, 4, redundancy.TotalCount())
			encodedPosition := binary.LittleEndian.Uint64(segments[0].GetSegment())
			position := metabase.SegmentPositionFromEncoded(encodedPosition)
			require.Equal(t, segment.Position, position)
		}

	})
}

func encryptionAccess(access string) (*encryption.Store, error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return nil, errors.New("invalid access grant format")
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return nil, err
	}

	key, err := storj.NewKey(p.EncryptionAccess.DefaultKey)
	if err != nil {
		return nil, err
	}

	store := encryption.NewStore()
	store.SetDefaultKey(key)
	store.SetDefaultPathCipher(storj.EncAESGCM)

	return store, nil
}
