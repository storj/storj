// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

func TestInspectorStats(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 6, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	uplink := planet.Uplinks[0]
	testData := make([]byte, 1*memory.MiB)
	_, err = rand.Read(testData)
	require.NoError(t, err)

	bucket := "testbucket"

	err = uplink.Upload(ctx, planet.Satellites[0], bucket, "test/path", testData)
	require.NoError(t, err)

	healthEndpoint := planet.Satellites[0].Inspector.Endpoint

	// Get path of random segment we just uploaded and check the health
	_ = planet.Satellites[0].Metainfo.Database.Iterate(storage.IterateOptions{Recurse: true},
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
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

				require.Equal(t, int32(0), resp.GetHealth().GetSuccessThreshold())
				require.Equal(t, int32(1), resp.GetHealth().GetMinimumRequired())
				require.Equal(t, int32(4), resp.GetHealth().GetTotal())
				require.Equal(t, int32(0), resp.GetHealth().GetRepairThreshold())
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

				require.Len(t, resp.GetSegments(), 1)

				segments := resp.GetSegments()
				require.Equal(t, int32(0), segments[0].GetSuccessThreshold())
				require.Equal(t, int32(1), segments[0].GetMinimumRequired())
				require.Equal(t, int32(4), segments[0].GetTotal())
				require.Equal(t, int32(0), segments[0].GetRepairThreshold())

				require.NoError(t, err)
			}

			return nil
		},
	)
}
