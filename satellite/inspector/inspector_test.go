// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package inspector_test

import (
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/inspector"
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
	assert.NoError(t, err)

	bucket := "testbucket"

	err = uplink.Upload(ctx, planet.Satellites[0], bucket, "test/path", testData)
	assert.NoError(t, err)

	log := zaptest.NewLogger(t)

	health, err := inspector.NewEndpoint(log, planet.Satellites[0].Overlay.Service, planet.Satellites[0].Metainfo.Service)
	assert.NoError(t, err)

	// Get path of random segment we just uploaded and check the health
	_ = planet.Satellites[0].Metainfo.Database.Iterate(storage.IterateOptions{Recurse: true, Reverse: false},
		func(it storage.Iterator) error {
			var item storage.ListItem
			for it.Next(&item) {
				if strings.Contains(string(item.Key), fmt.Sprintf("%s/", bucket)) {
					break
				}
			}

			fullPath := storj.SplitPath(item.Key.String())
			projectID := fullPath[0]
			bucket := fullPath[2]
			encryptedPath := strings.Join(fullPath[3:], "/")

			{ // Test Segment Health Request
				req := &pb.SegmentHealthRequest{
					ProjectId:     []byte(projectID),
					EncryptedPath: []byte(encryptedPath),
					Bucket:        []byte(bucket),
					Segment:       -1,
				}

				resp, err := health.SegmentHealth(ctx, req)
				assert.NoError(t, err)

				assert.Equal(t, int32(0), resp.GetSuccessThreshold())
				assert.Equal(t, int32(1), resp.GetMinimumRequired())
				assert.Equal(t, int32(4), resp.GetTotal())
				assert.Equal(t, int32(0), resp.GetRepairThreshold())
				assert.Equal(t, int32(4), resp.GetOnlineNodes())
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
				resp, err := health.ObjectHealth(ctx, objectHealthReq)

				assert.Equal(t, 1, len(resp.GetSegments()))

				segments := resp.GetSegments()
				assert.Equal(t, int32(0), segments[0].GetSuccessThreshold())
				assert.Equal(t, int32(1), segments[0].GetMinimumRequired())
				assert.Equal(t, int32(4), segments[0].GetTotal())
				assert.Equal(t, int32(0), segments[0].GetRepairThreshold())
				assert.Equal(t, int32(4), segments[0].GetOnlineNodes())

				assert.NoError(t, err)
			}

			return nil
		},
	)
}
