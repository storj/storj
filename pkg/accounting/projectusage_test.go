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

				if tt.expectedResource == "storage" {
					// if tt.setup then create a bucket_storage_rollup where settled is > alphamaxusage
				}

				if tt.expectedResource == "bandwidth" {
					// if tt.setip2 then create bucket_bandwidth_rollup where rollup.Inline, rollup.Remote > alphamaxusage
				}

				bandwidthTotal, err := acctDB.ProjectBandwidthTotal(ctx, project.ID, from)
				require.NoError(t, err)
				inlineTotal, remoteTotal, err := acctDB.ProjectStorageTotals(ctx, project.ID)
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
