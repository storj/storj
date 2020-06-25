// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/date"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/heldamount"
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
			SatelliteCount:   2,
			StorageNodeCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			satellite2 := planet.Satellites[1]
			sno := planet.StorageNodes[0]
			console := sno.Console
			bandwidthdb := sno.DB.Bandwidth()
			pricingdb := sno.DB.Pricing()
			storageusagedb := sno.DB.StorageUsage()
			reputationdb := sno.DB.Reputation()
			baseURL := fmt.Sprintf("http://%s/api/sno", console.Listener.Addr())

			now := time.Now().UTC().Add(-2 * time.Hour)

			randAmount1 := int64(120000000000)
			randAmount2 := int64(450000000000)

			for _, action := range actions {
				err := bandwidthdb.Add(ctx, satellite.ID(), action, randAmount1, now)
				require.NoError(t, err)

				err = bandwidthdb.Add(ctx, satellite2.ID(), action, randAmount2, now.Add(2*time.Hour))
				require.NoError(t, err)
			}
			var satellites []storj.NodeID

			satellites = append(satellites, satellite.ID(), satellite2.ID())
			stamps, _ := makeStorageUsageStamps(satellites)

			err := storageusagedb.Store(ctx, stamps)
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

			err = pricingdb.Store(ctx, pricing.Pricing{
				SatelliteID:     satellite2.ID(),
				EgressBandwidth: egressPrice,
				RepairBandwidth: repairPrice,
				AuditBandwidth:  auditPrice,
				DiskSpace:       diskPrice,
			})
			require.NoError(t, err)

			err = reputationdb.Store(ctx, reputation.Stats{
				SatelliteID: satellite.ID(),
				JoinedAt:    time.Now().UTC(),
			})
			require.NoError(t, err)
			err = reputationdb.Store(ctx, reputation.Stats{
				SatelliteID: satellite2.ID(),
				JoinedAt:    time.Now().UTC(),
			})
			require.NoError(t, err)

			t.Run("test EstimatedPayout", func(t *testing.T) {
				// should return estimated payout for both satellites in current month and empty for previous
				url := fmt.Sprintf("%s/estimatedPayout", baseURL)
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				expectedAuditRepairSatellite1 := 4 * (float64(randAmount1*auditPrice) / math.Pow10(12))
				expectedAuditRepairSatellite2 := 4 * float64(randAmount2*repairPrice) / math.Pow10(12)
				expectedUsageSatellite1 := 2 * float64(randAmount1*egressPrice) / math.Pow10(12)
				expectedUsageSatellite2 := 2 * float64(randAmount2*egressPrice) / math.Pow10(12)
				expectedDisk := int64(float64(30000000000000*diskPrice/730)/math.Pow10(12)) * int64(time.Now().UTC().Day())

				day := int64(time.Now().Day())

				month := time.Now().UTC()
				_, to := date.MonthBoundary(month)

				sum1 := expectedAuditRepairSatellite1 + expectedUsageSatellite1 + float64(expectedDisk)
				sum1AfterHeld := math.Round(sum1 / 4)
				estimated1 := int64(sum1AfterHeld) * int64(to.Day()) / day
				sum2 := expectedAuditRepairSatellite2 + expectedUsageSatellite2 + float64(expectedDisk)
				sum2AfterHeld := math.Round(sum2 / 4)
				estimated2 := int64(sum2AfterHeld) * int64(to.Day()) / day

				expected, err := json.Marshal(heldamount.EstimatedPayout{
					CurrentMonthEstimatedAmount: estimated1 + estimated2,
					CurrentMonthHeld:            int64(sum1 + sum2 - sum1AfterHeld - sum2AfterHeld),
					PreviousMonthPayout: heldamount.PayoutMonthly{
						EgressBandwidth:   0,
						EgressPayout:      0,
						EgressRepairAudit: 0,
						RepairAuditPayout: 0,
						DiskSpace:         0,
						DiskSpaceAmount:   0,
						HeldPercentRate:   0,
					},
				})
				require.NoError(t, err)
				require.Equal(t, string(expected)+"\n", string(body))

				// should return estimated payout for first satellite in current month and empty for previous
				url = fmt.Sprintf("%s/estimatedPayout?id=%s", baseURL, satellite.ID().String())
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				expected2, err := json.Marshal(heldamount.EstimatedPayout{
					CurrentMonthEstimatedAmount: estimated1,
					CurrentMonthHeld:            int64(sum1 - sum1AfterHeld),
					PreviousMonthPayout: heldamount.PayoutMonthly{
						EgressBandwidth:   0,
						EgressPayout:      0,
						EgressRepairAudit: 0,
						RepairAuditPayout: 0,
						DiskSpace:         0,
						DiskSpaceAmount:   0,
						HeldPercentRate:   75,
					},
				})
				require.NoError(t, err)
				require.Equal(t, string(expected2)+"\n", string(body2))
			})
		},
	)
}

// makeStorageUsageStamps creates storage usage stamps and expected summaries for provided satellites.
// Creates one entry per day for 30 days with last date as beginning of provided endDate.
func makeStorageUsageStamps(satellites []storj.NodeID) ([]storageusage.Stamp, map[storj.NodeID]float64) {
	var stamps []storageusage.Stamp
	summary := make(map[storj.NodeID]float64)

	now := time.Now().UTC().Day()

	for _, satellite := range satellites {
		for i := 0; i < now; i++ {
			stamp := storageusage.Stamp{
				SatelliteID:   satellite,
				AtRestTotal:   30000000000000,
				IntervalStart: time.Now().UTC().Add(time.Hour * -24 * time.Duration(i)),
			}

			summary[satellite] += stamp.AtRestTotal
			stamps = append(stamps, stamp)
		}
	}

	return stamps, summary
}
