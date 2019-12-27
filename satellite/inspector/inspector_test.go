// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storage"
	"storj.io/storj/uplink/eestream"
)

func TestInspectorStats(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		uplink := planet.Uplinks[0]
		testData := testrand.Bytes(1 * memory.MiB)

		bucket := "testbucket"

		err := uplink.Upload(ctx, planet.Satellites[0], bucket, "test/path", testData)
		require.NoError(t, err)

		healthEndpoint := planet.Satellites[0].Inspector.Endpoint

		// Get path of random segment we just uploaded and check the health
		_ = planet.Satellites[0].Metainfo.Database.Iterate(ctx, storage.IterateOptions{Recurse: true},
			func(ctx context.Context, it storage.Iterator) error {
				var item storage.ListItem
				for it.Next(ctx, &item) {
					if bytes.Contains(item.Key, []byte(fmt.Sprintf("%s/", bucket))) {
						break
					}
				}

				fullPath := storj.SplitPath(item.Key.String())
				require.Falsef(t, len(fullPath) < 4, "Could not retrieve a full path from pointerdb")

				projectID := fullPath[0]
				bucket := fullPath[2]
				encryptedPath := strings.Join(fullPath[3:], "/")

				{ // Test Segment Health Request
					req := &pb.SegmentHealthRequest{
						ProjectId:     []byte(projectID),
						EncryptedPath: []byte(encryptedPath),
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
						ProjectId:         []byte(projectID),
						EncryptedPath:     []byte(encryptedPath),
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
