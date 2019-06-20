// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

func TestInspectorStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplink := planet.Uplinks[0]
		testData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(testData)
		require.NoError(t, err)

		bucket := "testbucket"

		err = uplink.Upload(ctx, planet.Satellites[0], bucket, paths.NewUnencrypted("test/path"), testData)
		require.NoError(t, err)

		healthEndpoint := planet.Satellites[0].Inspector.Endpoint

		// Get path of random segment we just uploaded and check the health
		_ = planet.Satellites[0].Metainfo.Database.Iterate(ctx, storage.IterateOptions{Recurse: true},
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(ctx, &item) {
					path, err := metainfo.ParsePath([]byte(item.Key))
					require.NoError(t, err)
					if b, ok := path.Bucket(); ok && b == bucket {
						break
					}
				}

				fullPath, err := metainfo.ParsePath([]byte(item.Key))
				require.NoError(t, err)

				projectID := fullPath.ProjectID()
				bucket, ok := fullPath.Bucket()
				require.True(t, ok)
				encryptedPath := fullPath.EncryptedPath()

				{ // Test Segment Health Request
					req := &pb.SegmentHealthRequest{
						ProjectId:     []byte(projectID.String()),
						EncryptedPath: []byte(encryptedPath.Raw()),
						Bucket:        []byte(bucket),
						SegmentIndex:  -1,
					}

					resp, err := healthEndpoint.SegmentHealth(ctx, req)
					require.NoError(t, err)

					redundancy, err := eestream.NewRedundancyStrategyFromProto(resp.GetRedundancy())
					require.NoError(t, err)

					require.Equal(t, 4, redundancy.TotalCount())
					require.True(t, bytes.Equal([]byte("l"), resp.GetHealth().GetSegment()))
				}

				{ // Test Object Health Request
					objectHealthReq := &pb.ObjectHealthRequest{
						ProjectId:         []byte(projectID.String()),
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
					require.True(t, bytes.Equal([]byte("l"), segments[0].GetSegment()))
				}

				return nil
			},
		)
	})
}
