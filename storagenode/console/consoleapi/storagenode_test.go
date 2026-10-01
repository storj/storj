// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/date"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/storagenode/payouts/estimatedpayouts"
	"storj.io/storj/storagenode/pricing"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
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
					config.Compensation.Rates.GetTB = compensation.RequireRateFromString("20")
					config.Compensation.Rates.GetAuditTB = compensation.RequireRateFromString("10")
					config.Compensation.Rates.GetRepairTB = compensation.RequireRateFromString("10")
					config.Compensation.Rates.AtRestGBHours = compensation.RequireRateFromString(".00000208")
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
			sno.Reputation.Chore.Loop.Pause()
			startingPoint := time.Now().UTC().Add(-2 * time.Hour)
			// keep bandwidth usage in the current month, also in the first hours of the month.
			if beginOfMonth := date.UTCBeginOfMonth(time.Now()); startingPoint.Before(beginOfMonth) {
				startingPoint = beginOfMonth
			}

			for _, action := range actions {
				err := bandwidthdb.Add(ctx, satellite.ID(), action, 2300000000000, startingPoint)
				require.NoError(t, err)
			}
			var satellites []storj.NodeID

			satellites = append(satellites, satellite.ID())
			stamps := storagenodedbtest.MakeStorageUsageStamps(satellites, 30, time.Now().UTC())

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
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/estimated-payout", nil)
				require.NoError(t, err)

				// the handler takes its own "now", which is between nowBefore and nowAfter.
				nowBefore := time.Now()
				res, err := http.DefaultClient.Do(req)
				nowAfter := time.Now()
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.NotNil(t, body)

				bodyPayout := &estimatedpayouts.EstimatedPayout{}
				require.NoError(t, json.Unmarshal(body, bodyPayout))

				before, err := sno.Console.Service.GetAllSatellitesEstimatedPayout(ctx, nowBefore)
				require.NoError(t, err)
				after, err := sno.Console.Service.GetAllSatellitesEstimatedPayout(ctx, nowAfter)
				require.NoError(t, err)

				// Current-month disk space grows and the expectation shrinks with time,
				// so the response has to be between the values for nowBefore and nowAfter.
				require.GreaterOrEqual(t, bodyPayout.CurrentMonth.DiskSpace, before.CurrentMonth.DiskSpace)
				require.LessOrEqual(t, bodyPayout.CurrentMonth.DiskSpace, after.CurrentMonth.DiskSpace)
				require.LessOrEqual(t, bodyPayout.CurrentMonthExpectations, before.CurrentMonthExpectations)
				require.GreaterOrEqual(t, bodyPayout.CurrentMonthExpectations, after.CurrentMonthExpectations)

				expectedPayout := &estimatedpayouts.EstimatedPayout{
					CurrentMonth:             before.CurrentMonth,
					PreviousMonth:            before.PreviousMonth,
					CurrentMonthExpectations: bodyPayout.CurrentMonthExpectations,
				}
				expectedPayout.CurrentMonth.DiskSpace = bodyPayout.CurrentMonth.DiskSpace
				require.EqualValues(t, expectedPayout, bodyPayout)
			})
		},
	)
}
