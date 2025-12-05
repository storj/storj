// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/repair/queue"
	"storj.io/storj/satellite/repair/repairer"
	"storj.io/storj/satellite/repair/repairqueuetest"
)

func TestStatChore(t *testing.T) {
	repairqueuetest.Run(t, func(ctx *testcontext.Context, t *testing.T, repairQueue queue.RepairQueue) {
		_, err := repairQueue.InsertBatch(ctx, []*queue.InjuredSegment{
			{
				StreamID:  testrand.UUID(),
				Placement: storj.PlacementConstraint(1),
			},
			{
				StreamID:  testrand.UUID(),
				Placement: storj.PlacementConstraint(2),
			},
			{
				StreamID:  testrand.UUID(),
				Placement: storj.PlacementConstraint(2),
			},
		})
		require.NoError(t, err)

		registry := monkit.NewRegistry()
		chore := repairer.NewQueueStat(zaptest.NewLogger(t), registry, []storj.PlacementConstraint{0, 1, 2}, repairQueue, 100*time.Hour)

		collectMonkitStat := func() map[string]float64 {
			monkitValues := map[string]float64{}
			registry.Stats(func(key monkit.SeriesKey, field string, val float64) {
				if key.Measurement != "repair_queue" {
					return
				}

				tags := key.Tags.All()

				var tagKeys []string
				for t := range tags {
					tagKeys = append(tagKeys, t)
				}
				sort.Strings(tagKeys)

				var tagKeyValues []string
				for _, k := range tagKeys {
					tagKeyValues = append(tagKeyValues, fmt.Sprintf("%s=%s", k, tags[k]))
				}

				monkitValues[strings.Join(tagKeyValues, ",")+" "+field] = val
			})
			return monkitValues
		}

		stat := collectMonkitStat()
		require.Zero(t, stat["attempted=false,placement=1,scope=storj.io/storj/satellite/repair/repairer count"])

		chore.RunOnce(ctx)
		stat = collectMonkitStat()

		require.Equal(t, float64(0), stat["attempted=false,placement=0,scope=storj.io/storj/satellite/repair/repairer count"])
		require.Equal(t, float64(1), stat["attempted=false,placement=1,scope=storj.io/storj/satellite/repair/repairer count"])
		require.Equal(t, float64(2), stat["attempted=false,placement=2,scope=storj.io/storj/satellite/repair/repairer count"])
	})
}
