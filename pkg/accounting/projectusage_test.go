// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package accounting_test

import (
	"testing"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/accounting"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
)

func TestProjectUsage(t *testing.T) {
	cases := []struct {
		name             string
		expectedExceeded bool
		expectedResource string
	}{
		{name: "doesn't exceed storage or bandwidth project limit", expectedExceeded: false, expectedResource: ""},
		{name: "exceeds storage project limit", expectedExceeded: true, expectedResource: "storage"},
		{name: "exceeds bandwidth project limit", expectedExceeded: true, expectedResource: "bandwidth"},
	}

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		saDB := planet.Satellites[0].DB
		orderDB := saDB.Orders()
		acctDB := saDB.Accounting()
		projectsDB := saDB.Console().Projects()

		// Setup: This date represents the past 30 days so that we can check
		// if the alpha max usage has been exceeded in the past month
		from := time.Now().AddDate(0, 0, -accounting.AverageDaysInMonth)

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {

				// Setup: create a new project to use the projectID
				pID, err := uuid.New()
				require.NoError(t, err)
				project, err := projectsDB.Insert(ctx, &console.Project{ID: *pID})
				require.NoError(t, err)
				bucketID := createBucketID(project.ID, []byte("testbucket"))

				// Setup: create a BucketStorageTally record to test exceeding storage project limit
				if tt.expectedResource == "storage" {
					tally := accounting.BucketStorageTally{
						BucketName:    "testbucket",
						ProjectID:     project.ID,
						IntervalStart: time.Now(),
						RemoteBytes:   26 * memory.GB.Int64(),
					}
					err := acctDB.CreateBucketStorageTally(ctx, tally)
					require.NoError(t, err)
				}

				// Setup: create a BucketBandwidthRollup record to test exceeding bandwidth project limit
				if tt.expectedResource == "bandwidth" {
					amount := 26 * memory.GB.Int64()
					action := pb.PieceAction_GET
					err := orderDB.UpdateBucketBandwidthSettle(ctx, bucketID, action, amount)
					require.NoError(t, err)
				}

				// Execute test: get storage and bandwidth totals for a project, then check if that exceeds the max usage limit
				inlineTotal, remoteTotal, err := acctDB.ProjectStorageTotals(ctx, project.ID)
				require.NoError(t, err)
				bandwidthTotal, err := acctDB.ProjectBandwidthTotal(ctx, bucketID, from)
				require.NoError(t, err)
				maxAlphaUsage := 25 * memory.GB
				actualExceeded, actualResource := accounting.ExceedsAlphaUsage(bandwidthTotal, inlineTotal, remoteTotal, maxAlphaUsage)

				require.Equal(t, tt.expectedExceeded, actualExceeded)
				require.Equal(t, tt.expectedResource, actualResource)
			})
		}
	})
}

func createBucketID(projectID uuid.UUID, bucket []byte) []byte {
	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, string(bucket))
	return []byte(storj.JoinPaths(entries...))
}
