// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
)

func TestBillingTrafficAfterFileDeletion(t *testing.T) {
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
		bandwidth := getTotalProjectBandwidth(ctx, t, planet, 0, projectID, time.Now().Add(-3*time.Hour))
		require.NotZero(t, bandwidth, "Egress should not be empty")
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
		bandwidth := getTotalProjectBandwidth(ctx, t, planet, 0, projectID, since)
		require.Zero(t, bandwidth, "billed bandwidth")

		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.Upload(ctx, satelliteSys, bucketName, objectKey, data)
			require.NoError(t, err)
		}

		bandwidth = getTotalProjectBandwidth(ctx, t, planet, 0, projectID, since)
		require.Zero(t, bandwidth, "billed bandwidth")

		_, err := uplnk.Download(ctx, satelliteSys, bucketName, objectKey)
		require.NoError(t, err)

		bandwidth = getTotalProjectBandwidth(ctx, t, planet, 0, projectID, since)
		require.NotZero(t, bandwidth, "billed bandwidth")
	})
}

// getTotalProjectBandwidth returns the total used egress bandwidth for the
// projectID in the satellite referenced by satelliteIdx index.
func getTotalProjectBandwidth(
	ctx context.Context, t *testing.T, planet *testplanet.Planet, satelliteIdx int,
	projectID uuid.UUID, since time.Time,
) int64 {
	t.Helper()

	// Wait for the SNs endpoints to finish thir work
	require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

	// Calculate the bandwidth used for upload
	for _, sn := range planet.StorageNodes {
		sn.Storage2.Orders.Sender.TriggerWait()
	}

	sat := planet.Satellites[satelliteIdx]
	{
		rollout := sat.Core.Accounting.ReportedRollupChore
		require.NoError(t, rollout.RunOnce(ctx, since))
	}

	sat.Accounting.Tally.Loop.TriggerWait()

	bandwidth, err := sat.DB.ProjectAccounting().GetProjectTotal(ctx, projectID, since, time.Now())
	require.NoError(t, err)

	return bandwidth.Egress
}
