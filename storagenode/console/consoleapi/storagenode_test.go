// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storageusage"
)

var (
	actions = []pb.PieceAction{
		pb.PieceAction_INVALID,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,

		pb.PieceAction_PUT,
		pb.PieceAction_GET,
		pb.PieceAction_GET_AUDIT,
		pb.PieceAction_GET_REPAIR,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_DELETE,
	}
)

func TestStorageNodeApi(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
					config.Payments.NodeEgressBandwidthPrice = 2000
					config.Payments.NodeAuditBandwidthPrice = 1000
					config.Payments.NodeRepairBandwidthPrice = 1000
					config.Payments.NodeDiskSpacePrice = 150
				},
			},
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			sno := planet.StorageNodes[0]
			console := sno.Console
			bandwidthdb := sno.DB.Bandwidth()
			pricingdb := sno.DB.Pricing()
			storageusagedb := sno.DB.StorageUsage()
			reputationdb := sno.DB.Reputation()
			baseURL := fmt.Sprintf("http://%s/api/sno", console.Listener.Addr())

			// pause node stats reputation cache because later tests assert a specific join date.
			sno.NodeStats.Cache.Reputation.Pause()
			startingPoint := time.Now().UTC().Add(-2 * time.Hour)

			for _, action := range actions {
				err := bandwidthdb.Add(ctx, satellite.ID(), action, 2300000000000, startingPoint)
				require.NoError(t, err)
			}
			var satellites []storj.NodeID

			satellites = append(satellites, satellite.ID())
			stamps, _ := makeStorageUsageStamps(satellites)

			err := storageusagedb.Store(ctx, stamps)
			require.NoError(t, err)

			err = reputationdb.Store(ctx, reputation.Stats{
				SatelliteID: satellite.ID(),
				JoinedAt:    startingPoint.AddDate(0, -2, 0),
			})
			require.NoError(t, err)

			egressPrice, repairPrice, auditPrice, diskPrice := int64(2000), int64(1000), int64(1000), int64(150)

			err = pricingdb.Store(ctx, pricing.Pricing{
				SatelliteID:     satellite.ID(),
				EgressBandwidth: egressPrice,
				RepairBandwidth: repairPrice,
				AuditBandwidth:  auditPrice,
				DiskSpace:       diskPrice,
			})
			require.NoError(t, err)

			t.Run("EstimatedPayout", func(t *testing.T) {
				// should return estimated payout for both satellites in current month and empty for previous
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/estimated-payout", baseURL), nil)
				require.NoError(t, err)

				// setting now here to cache closest to api all timestamp, so service call
				// would not have difference in passed "now" that can distort result
				now := time.Now()
				res, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)
				require.NotNil(t, body)

				bodyPayout := &estimatedpayouts.EstimatedPayout{}
				require.NoError(t, json.Unmarshal(body, bodyPayout))

				estimation, err := sno.Console.Service.GetAllSatellitesEstimatedPayout(ctx, now)
				require.NoError(t, err)

				expectedPayout := &estimatedpayouts.EstimatedPayout{
					CurrentMonth:             estimation.CurrentMonth,
					PreviousMonth:            estimation.PreviousMonth,
					CurrentMonthExpectations: estimation.CurrentMonthExpectations,
				}
				require.EqualValues(t, expectedPayout, bodyPayout)
			})
		},
	)
}

// makeStorageUsageStamps creates storage usage stamps and expected summaries for provided satellites.
// Creates one entry per day for 30 days with last date as beginning of provided endDate.
func makeStorageUsageStamps(satellites []storj.NodeID) ([]storageusage.Stamp, map[storj.NodeID]float64) {
	var stamps []storageusage.Stamp
	summary := make(map[storj.NodeID]float64)

	now := time.Now().UTC()
	currentDay := now.Day()

	for _, satellite := range satellites {
		for i := 0; i < currentDay; i++ {
			intervalEndTime := now.Add(time.Hour * -24 * time.Duration(i))
			stamp := storageusage.Stamp{
				SatelliteID:     satellite,
				AtRestTotal:     30000000000000,
				IntervalStart:   time.Date(intervalEndTime.Year(), intervalEndTime.Month(), intervalEndTime.Day(), 0, 0, 0, 0, intervalEndTime.Location()),
				IntervalEndTime: intervalEndTime,
			}

			summary[satellite] += stamp.AtRestTotal
			stamps = append(stamps, stamp)
		}
	}

	return stamps, summary
}
