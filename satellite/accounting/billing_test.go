// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
)

func TestBilling_FilesAfterDeletion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Orders.FlushBatchSize = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "testbucket"
			filePath   = "test/path"
		)

		var (
			satelliteSys = planet.Satellites[0]
			uplink       = planet.Uplinks[0]
			projectID    = uplink.ProjectID[satelliteSys.ID()]
			since        = time.Now()
		)

		satelliteSys.Accounting.Tally.Loop.Pause()

		// Prepare some data for the Uplink to upload
		uploadData := testrand.Bytes(5 * memory.KiB)

		err := uplink.Upload(ctx, satelliteSys, bucketName, filePath, uploadData)
		require.NoError(t, err)

		// We need to call tally twice, it calculates the estimated time
		// using the difference in the generation time of the two tallies
		satelliteSys.Accounting.Tally.Loop.TriggerWait()

		// Get usage for uploaded file before we delete it
		usageBefore := getProjectTotal(ctx, t, planet, 0, projectID, since)

		// ObjectCount and Storage should be > 0
		require.NotZero(t, usageBefore.ObjectCount)
		require.NotZero(t, usageBefore.Storage)
		require.Zero(t, usageBefore.Egress)

		err = uplink.DeleteObject(ctx, satelliteSys, bucketName, filePath)
		require.NoError(t, err)

		err = uplink.DeleteBucket(ctx, satelliteSys, bucketName)
		require.NoError(t, err)

		// Get usage after file was deleted
		usageAfter := getProjectTotal(ctx, t, planet, 0, projectID, since)

		// Verify data is correct. We don’t bill for the data after deleting objects, usage should be equal
		require.Equal(t, usageBefore.ObjectCount, usageAfter.ObjectCount, "Object count should be equal")
		require.Equal(t, usageBefore.Storage, usageAfter.Storage, "Storage should be equal")
		require.Zero(t, usageAfter.Egress, "Egress should be 0")
	})
}

func TestBilling_TrafficAfterFileDeletion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "testbucket"
			filePath   = "test/path"
		)

		var (
			satelliteSys = planet.Satellites[0]
			uplink       = planet.Uplinks[0]
			projectID    = uplink.ProjectID[satelliteSys.ID()]
		)

		data := testrand.Bytes(5 * memory.KiB)
		err := uplink.Upload(ctx, satelliteSys, bucketName, filePath, data)
		require.NoError(t, err)

		_, err = uplink.Download(ctx, satelliteSys, bucketName, filePath)
		require.NoError(t, err)

		err = uplink.DeleteObject(ctx, satelliteSys, bucketName, filePath)
		require.NoError(t, err)

		err = uplink.DeleteBucket(ctx, satelliteSys, bucketName)
		require.NoError(t, err)

		// Check that download traffic gets billed even if the file and bucket was deleted
		usage := getProjectTotal(ctx, t, planet, 0, projectID, time.Now().Add(-30*time.Millisecond))
		require.NotZero(t, usage.Egress, "Egress should not be empty")
	})
}

func TestBilling_DownloadAndNoUploadTraffic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "a-bucket"
			objectKey  = "object-filename"
		)

		satelliteSys := planet.Satellites[0]
		// Make sure that we don't have interference with billed repair traffic
		// in case of a bug. There is a specific test to verify that the repair
		// traffic isn't billed.
		satelliteSys.Audit.Chore.Loop.Stop()
		satelliteSys.Repair.Repairer.Loop.Stop()

		var (
			uplnk     = planet.Uplinks[0]
			projectID = uplnk.ProjectID[satelliteSys.ID()]
		)

		since := time.Now().Add(-10 * time.Hour)
		usage := getProjectTotal(ctx, t, planet, 0, projectID, since)
		require.Zero(t, usage.Egress, "billed usage")

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.Upload(ctx, satelliteSys, bucketName, objectKey, data)
			require.NoError(t, err)
		}

		usage = getProjectTotal(ctx, t, planet, 0, projectID, since)
		require.Zero(t, usage.Egress, "billed usage")

		_, err := uplnk.Download(ctx, satelliteSys, bucketName, objectKey)
		require.NoError(t, err)

		usage = getProjectTotal(ctx, t, planet, 0, projectID, since)
		require.NotZero(t, usage.Egress, "billed usage")
	})
}

// getProjectTotal returns used egress, storage, objectCount for the
// projectID in the satellite referenced by satelliteIdx index.
func getProjectTotal(
	ctx context.Context, t *testing.T, planet *testplanet.Planet, satelliteIdx int,
	projectID uuid.UUID, since time.Time,
) *accounting.ProjectUsage {
	t.Helper()

	// Wait for the SNs endpoints to finish their work
	require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

	// Calculate the usage used for upload
	for _, sn := range planet.StorageNodes {
		sn.Storage2.Orders.Sender.TriggerWait()
	}

	sat := planet.Satellites[satelliteIdx]
	{
		rollout := sat.Core.Accounting.ReportedRollupChore
		require.NoError(t, rollout.RunOnce(ctx, since))
	}

	sat.Accounting.Tally.Loop.TriggerWait()

	usage, err := sat.DB.ProjectAccounting().GetProjectTotal(ctx, projectID, since, time.Now())
	require.NoError(t, err)

	return usage
}
