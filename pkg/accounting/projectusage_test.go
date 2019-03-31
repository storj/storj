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
		acctDB := saDB.Accounting()
		projectsDB := saDB.Console().Projects()
		from := time.Now().AddDate(0, 0, -accounting.AvgDaysInMonth) // past 30 days

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				pID, err := uuid.New()
				require.NoError(t, err)
				project, err := projectsDB.Insert(ctx, &console.Project{ID: *pID})
				require.NoError(t, err)

				// Setup to test exceeding storage project limit
				if tt.expectedResource == "storage" {
					// seed DB with over 25GB of storage
					tally := accounting.BucketStorageTally{
						BucketName:    "testbucket",
						ProjectID:     project.ID,
						IntervalStart: time.Now(),
						RemoteBytes:   26000000000, // 26GB
					}
					acctDB.CreateBucketStorageTally(ctx, tally)
				}

				// Setup to test exceeding bandwidth project limit
				if tt.expectedResource == "bandwidth" {
					// seed DB with over 25GBh of bandwidth usage
					rollup := accounting.BucketBandwidthRollup{
						BucketName:    "testbucket",
						ProjectID:     project.ID,
						Settled:       28e13, // 28TB
						IntervalStart: time.Now(),
						Action:        uint(pb.BandwidthAction_GET),
					}
					acctDB.CreateBucketBandwidthRollup(ctx, rollup)
				}

				// Execute test: get Project Storage and Bandwidth totals, then check if that exceeds the max usage limit
				inlineTotal, remoteTotal, err := acctDB.ProjectStorageTotals(ctx, project.ID)
				require.NoError(t, err)
				bandwidthTotal, err := acctDB.ProjectBandwidthTotal(ctx, project.ID, from)
				require.NoError(t, err)
				maxAlphaUsage := memory.Size(25 * memory.GB)
				actualExceeded, actualResource := accounting.ExceedsAlphaUsage(bandwidthTotal, inlineTotal, remoteTotal, maxAlphaUsage)

				if tt.expectedExceeded != actualExceeded {
					t.Fatalf("expect exceeded: %v, actual exceeded: %v", tt.expectedExceeded, actualExceeded)
				}
				if tt.expectedResource != actualResource {
					t.Fatalf("expect resrouce: %s, actual resource: %s", tt.expectedResource, actualResource)
				}
			})
		}
	})
}
