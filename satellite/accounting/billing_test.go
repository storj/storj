// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/overlay"
)

func TestBilling_InlineFiles(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Orders.FlushBatchSize = 1
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "testbucket"
			firstPath  = "path"
			secondPath = "another_path"
		)

		var (
			satelliteSys = planet.Satellites[0]
			uplink       = planet.Uplinks[0]
			projectID    = uplink.ProjectID[satelliteSys.ID()]
			since        = time.Now()
		)

		satelliteSys.Accounting.Tally.Loop.Pause()

		// Prepare two inline segments for the Uplink to upload
		firstSegment := testrand.Bytes(2 * memory.KiB)
		secondSegment := testrand.Bytes(3 * memory.KiB)

		err := uplink.Upload(ctx, satelliteSys, bucketName, firstPath, firstSegment)
		require.NoError(t, err)
		err = uplink.Upload(ctx, satelliteSys, bucketName, secondPath, secondSegment)
		require.NoError(t, err)

		_, err = uplink.Download(ctx, satelliteSys, bucketName, firstPath)
		require.NoError(t, err)

		// We need to call tally twice, it calculates the estimated time
		// using the difference in the generation time of the two tallies
		satelliteSys.Accounting.Tally.Loop.TriggerWait()

		usage := getProjectTotal(ctx, t, planet, 0, projectID, since)

		// Usage should be > 0
		require.NotZero(t, usage.ObjectCount)
		require.NotZero(t, usage.Storage)
		require.NotZero(t, usage.Egress)
	})
}

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

func TestBilling_AuditRepairTraffic(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 4),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		const (
			bucketName = "a-bucket"
			objectKey  = "object-filename"
		)

		satelliteSys := planet.Satellites[0]
		satelliteSys.Audit.Worker.Loop.Pause()
		satelliteSys.Repair.Checker.Loop.Pause()
		satelliteSys.Repair.Repairer.Loop.Pause()

		for _, sn := range planet.StorageNodes {
			sn.Storage2.Orders.Sender.Pause()
		}

		uplnk := planet.Uplinks[0]
		{
			data := testrand.Bytes(10 * memory.KiB)
			err := uplnk.Upload(ctx, satelliteSys, bucketName, objectKey, data)
			require.NoError(t, err)
		}

		// make sure we have at least one tally in db, so when we call
		// getProjectStorage it returns something
		satelliteSys.Accounting.Tally.Loop.TriggerWait()

		_, err := uplnk.Download(ctx, satelliteSys, bucketName, objectKey)
		require.NoError(t, err)

		var (
			projectID = uplnk.ProjectID[satelliteSys.ID()]
			since     = time.Now()
		)
		projectTotal := getProjectTotal(ctx, t, planet, 0, projectID, since)
		require.NotZero(t, projectTotal.Egress)

		// get the only metainfo record (our upload)
		key, err := planet.Satellites[0].Metainfo.Database.List(ctx, nil, 10)
		require.NoError(t, err)
		require.Len(t, key, 1)
		ptr, err := satelliteSys.Metainfo.Service.Get(ctx, key[0].String())
		require.NoError(t, err)

		// Cause repair traffic
		stoppedNodeID := ptr.GetRemote().GetRemotePieces()[0].NodeId
		stopNodeByID(ctx, t, planet, stoppedNodeID)

		runningNodes := make([]*testplanet.StorageNode, 0)
		for _, node := range planet.StorageNodes {
			if node.ID() != stoppedNodeID {
				runningNodes = append(runningNodes, node)
			}
		}

		// trigger repair
		_, err = satelliteSys.Repairer.SegmentRepairer.Repair(ctx, key[0].String())
		require.NoError(t, err)

		// get the only metainfo record (our upload)
		key, err = planet.Satellites[0].Metainfo.Database.List(ctx, nil, 1)
		require.NoError(t, err)
		require.Len(t, key, 1)

		ptr2, err := satelliteSys.Metainfo.Service.Get(ctx, key[0].String())
		require.NoError(t, err)

		remotePieces := ptr2.GetRemote().GetRemotePieces()
		require.NotEqual(t, ptr, ptr2)
		for _, piece := range remotePieces {
			require.NotEqual(t, stoppedNodeID, piece.NodeId, "there shouldn't be pieces in stopped nodes")
		}

		projectTotalAfterRepair := getProjectTotalFromStorageNodes(ctx, t, planet, 0, projectID, since, runningNodes)
		require.Equal(t, projectTotal.Egress, projectTotalAfterRepair.Egress, "bandwidth totals")
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

	return getProjectTotalFromStorageNodes(ctx, t, planet, satelliteIdx, projectID, since, planet.StorageNodes)
}

// getProjectTotal returns used egress, storage, objectCount for the
// projectID in the satellite referenced by satelliteIdx index.
func getProjectTotalFromStorageNodes(
	ctx context.Context, t *testing.T, planet *testplanet.Planet, satelliteIdx int,
	projectID uuid.UUID, since time.Time, storageNodes []*testplanet.StorageNode,
) *accounting.ProjectUsage {
	t.Helper()

	// Wait for the SNs endpoints to finish their work
	require.NoError(t, planet.WaitForStorageNodeEndpoints(ctx))

	// Calculate the usage used for upload
	for _, sn := range storageNodes {
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

func stopNodeByID(ctx context.Context, t *testing.T, planet *testplanet.Planet, nodeID storj.NodeID) {
	t.Helper()

	for _, node := range planet.StorageNodes {
		if node.ID() == nodeID {

			err := planet.StopPeer(node)
			require.NoError(t, err)
			for _, satellite := range planet.Satellites {
				err := satellite.Overlay.Service.UpdateCheckIn(ctx, overlay.NodeCheckInInfo{
					NodeID:  node.ID(),
					Address: &pb.NodeAddress{Address: node.Addr()},
					IsUp:    true,
					Version: &pb.NodeVersion{
						Version:    "v0.0.0",
						CommitHash: "",
						Timestamp:  time.Time{},
						Release:    false,
					},
				}, time.Now().Add(-4*time.Hour))
				require.NoError(t, err)
			}

			break
		}
	}
}
