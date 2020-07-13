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
	t.Skip("Flaky")

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

				egressBandwidth1 := randAmount1
				egressBandwidthPayout1 := int64(float64(randAmount1*egressPrice) / math.Pow10(12))
				egressRepairAudit1 := randAmount1 * 2
				egressRepairAuditPayout1 := int64(float64(randAmount1*auditPrice+randAmount1*repairPrice) / math.Pow10(12))
				diskSpace1 := 30000000000000 * time.Now().UTC().Day()
				egressBandwidth2 := randAmount2
				egressBandwidthPayout2 := int64(float64(randAmount2*egressPrice) / math.Pow10(12))
				egressRepairAudit2 := randAmount2 * 2
				egressRepairAuditPayout2 := int64(float64(randAmount2*auditPrice+randAmount2*repairPrice) / math.Pow10(12))
				diskSpace2 := 30000000000000 * time.Now().UTC().Day()
				diskSpacePayout2 := int64(30000000000000/720/math.Pow10(12)*float64(diskPrice)) * int64(time.Now().UTC().Day())

				expected, err := json.Marshal(heldamount.EstimatedPayout{
					CurrentMonth: heldamount.PayoutMonthly{
						EgressBandwidth:         2 * (egressBandwidth1 + egressBandwidth2),
						EgressBandwidthPayout:   (egressBandwidthPayout1 + egressBandwidthPayout2) / 2,
						EgressRepairAudit:       2 * (egressRepairAudit1 + egressRepairAudit2),
						EgressRepairAuditPayout: (egressRepairAuditPayout1 + egressRepairAuditPayout2) / 2,
						DiskSpace:               float64(diskSpace1 + diskSpace2),
						DiskSpacePayout:         (diskSpacePayout2 - (diskSpacePayout2 * 75 / 100)) * 2,
						HeldRate:                0,
						Held:                    (2*(egressBandwidthPayout1+egressRepairAuditPayout1)+diskSpacePayout2)*75/100 + (2*(egressBandwidthPayout2+egressRepairAuditPayout2)+diskSpacePayout2)*75/100,
						Payout:                  ((egressBandwidthPayout1 + egressBandwidthPayout2) / 2) + ((egressRepairAuditPayout1 + egressRepairAuditPayout2) / 2) + (diskSpacePayout2-(diskSpacePayout2*75/100))*2,
					},
					PreviousMonth: heldamount.PayoutMonthly{
						EgressBandwidth:         0,
						EgressBandwidthPayout:   0,
						EgressRepairAudit:       0,
						EgressRepairAuditPayout: 0,
						DiskSpace:               0,
						DiskSpacePayout:         0,
						HeldRate:                0,
						Held:                    0,
						Payout:                  0,
					},
				})
				require.NoError(t, err)
				require.Equal(t, string(expected)+"\n", string(body))
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
