// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/orders"
)

func TestOrdersStore_Enqueue_GracePeriodFailure(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	dirName := ctx.Dir("test-orders")
	now := time.Now()

	// make order limit grace period 24 hours
	ordersStore, err := orders.NewFileStore(zaptest.NewLogger(t), dirName, 24*time.Hour)
	require.NoError(t, err)

	// adding order before grace period should result in an error
	newSN := testrand.SerialNumber()
	newInfo := &orders.Info{
		Limit: &pb.OrderLimit{
			SerialNumber:    newSN,
			SatelliteId:     testrand.NodeID(),
			Action:          pb.PieceAction_GET,
			OrderCreation:   now.Add(-48 * time.Hour),
			OrderExpiration: now.Add(time.Hour),
		},
		Order: &pb.Order{
			SerialNumber: newSN,
			Amount:       10,
		},
	}
	err = ordersStore.Enqueue(newInfo)
	require.Error(t, err)
}

func TestOrdersStore_ListUnsentBySatellite(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	dirName := ctx.Dir("test-orders")
	now := time.Now()

	// make order limit grace period 12 hours
	ordersStore, err := orders.NewFileStore(zaptest.NewLogger(t), dirName, 12*time.Hour)
	require.NoError(t, err)

	// for each satellite, make three orders from four hours ago, three from two hours ago, and three from now.
	numSatellites := 3
	createdTimes := []time.Time{
		now.Add(-4 * time.Hour),
		now.Add(-2 * time.Hour),
		now,
	}
	serialsPerSatPerTime := 3

	originalInfos, err := storeNewOrders(ordersStore, numSatellites, serialsPerSatPerTime, createdTimes)
	require.NoError(t, err)

	// 3 times:
	//    list unsent orders - should receive data from all satellites the first two times, and nothing the last time.
	//    archive unsent orders
	expectedArchivedInfos := make(map[storj.SerialNumber]*orders.ArchivedInfo)

	archiveTime1 := now.Add(-2 * time.Hour)
	archiveTime2 := now
	status1 := pb.SettlementWithWindowResponse_ACCEPTED
	status2 := pb.SettlementWithWindowResponse_REJECTED
	for i := 0; i < 3; i++ {
		// This should return all the orders created more than 1 hour before "now".
		unsentMap, err := ordersStore.ListUnsentBySatellite(now.Add(12 * time.Hour))
		require.NoError(t, err)

		// on last iteration, expect nothing returned
		if i == 2 {
			require.Len(t, unsentMap, 0)
			break
		}

		// go through order limits and make sure information is accurate
		require.Len(t, unsentMap, numSatellites)
		for satelliteID, unsentSatList := range unsentMap {
			require.Len(t, unsentSatList.InfoList, serialsPerSatPerTime)

			for _, unsentInfo := range unsentSatList.InfoList {
				// "new" orders should not be returned
				require.True(t, unsentInfo.Limit.OrderCreation.Before(createdTimes[2]))
				sn := unsentInfo.Limit.SerialNumber
				originalInfo := originalInfos[sn]

				verifyInfosEqual(t, unsentInfo, originalInfo)
				// expect that creation hour is consistent with order
				require.True(t, unsentSatList.CreatedAtHour.Equal(unsentInfo.Limit.OrderCreation.Truncate(time.Hour)))

				// add to archive batch
				// create
				archivedAt := archiveTime1
				orderStatus := orders.StatusAccepted
				if i == 1 {
					archivedAt = archiveTime2
					orderStatus = orders.StatusRejected
				}
				newArchivedInfo := &orders.ArchivedInfo{
					Limit:      unsentInfo.Limit,
					Order:      unsentInfo.Order,
					Status:     orderStatus,
					ArchivedAt: archivedAt,
				}
				expectedArchivedInfos[unsentInfo.Limit.SerialNumber] = newArchivedInfo
			}

			// archive unsent file
			archivedAt := archiveTime1
			status := status1
			if i == 1 {
				archivedAt = archiveTime2
				status = status2
			}
			err = ordersStore.Archive(satelliteID, unsentSatList.CreatedAtHour, archivedAt, status)
			require.NoError(t, err)
		}
	}

	// list archived, expect everything from first two created at time buckets
	archived, err := ordersStore.ListArchived()
	require.NoError(t, err)
	require.Len(t, archived, numSatellites*serialsPerSatPerTime*2)
	for _, archivedInfo := range archived {
		sn := archivedInfo.Limit.SerialNumber
		expectedInfo := expectedArchivedInfos[sn]
		verifyArchivedInfosEqual(t, expectedInfo, archivedInfo)

		// one of the batches should be "accepted" and the other should be "rejected"
		if archivedInfo.ArchivedAt.Round(0).Equal(archiveTime2.Round(0)) {
			require.Equal(t, archivedInfo.Status, orders.StatusRejected)
		} else {
			require.Equal(t, archivedInfo.Status, orders.StatusAccepted)
		}
	}

	// clean archive for anything older than 30 minutes
	err = ordersStore.CleanArchive(now.Add(-30 * time.Minute))
	require.NoError(t, err)

	// list archived, expect only recent archived batch (other was cleaned)
	archived, err = ordersStore.ListArchived()
	require.NoError(t, err)
	require.Len(t, archived, numSatellites*serialsPerSatPerTime)
	for _, archivedInfo := range archived {
		sn := archivedInfo.Limit.SerialNumber
		expectedInfo := expectedArchivedInfos[sn]
		verifyArchivedInfosEqual(t, expectedInfo, archivedInfo)
		require.Equal(t, archivedInfo.ArchivedAt.Round(0), archiveTime2.Round(0))
		require.Equal(t, archivedInfo.Status, orders.StatusRejected)
	}

	// clean archive for everything before now, expect list to return nothing
	err = ordersStore.CleanArchive(now.Add(time.Nanosecond))
	require.NoError(t, err)
	archived, err = ordersStore.ListArchived()
	require.NoError(t, err)
	require.Len(t, archived, 0)
}

func TestOrdersStore_ListUnsentBySatellite_Ongoing(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	dirName := ctx.Dir("test-orders")
	now := time.Now()
	satellite := testrand.NodeID()
	tomorrow := now.Add(24 * time.Hour)

	// make order limit grace period 1 hour
	ordersStore, err := orders.NewFileStore(zaptest.NewLogger(t), dirName, time.Hour)
	require.NoError(t, err)

	// empty store means no orders can be listed
	unsent, err := ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 0)

	// store an order that can be listed
	sn := testrand.SerialNumber()
	require.NoError(t, ordersStore.Enqueue(&orders.Info{
		Limit: &pb.OrderLimit{
			SerialNumber:  sn,
			SatelliteId:   satellite,
			Action:        pb.PieceAction_GET,
			OrderCreation: now,
		},
		Order: &pb.Order{
			SerialNumber: sn,
			Amount:       1,
		},
	}))

	// check that we can list it tomorrow
	unsent, err = ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 1)

	// begin an enqueue in the bucket
	commit, err := ordersStore.BeginEnqueue(satellite, now)
	require.NoError(t, err)

	// we should no longer be able to list that window
	unsent, err = ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 0)

	// commit the order
	sn = testrand.SerialNumber()
	require.NoError(t, commit(&orders.Info{
		Limit: &pb.OrderLimit{
			SerialNumber:  sn,
			SatelliteId:   satellite,
			Action:        pb.PieceAction_GET,
			OrderCreation: now,
		},
		Order: &pb.Order{
			SerialNumber: sn,
			Amount:       1,
		},
	}))

	// check that we can list it tomorrow
	unsent, err = ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 1)
}

func TestOrdersStore_CorruptUnsent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	dirName := ctx.Dir("test-orders")
	now := time.Now()
	satellite := testrand.NodeID()
	tomorrow := now.Add(24 * time.Hour)

	// make order limit grace period 1 hour
	ordersStore, err := orders.NewFileStore(zaptest.NewLogger(t), dirName, time.Hour)
	require.NoError(t, err)

	// empty store means no orders can be listed
	unsent, err := ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 0)

	sn := testrand.SerialNumber()
	info := &orders.Info{
		Limit: &pb.OrderLimit{
			SerialNumber:  sn,
			SatelliteId:   satellite,
			Action:        pb.PieceAction_GET,
			OrderCreation: now,
		},
		Order: &pb.Order{
			SerialNumber: sn,
			Amount:       1,
		},
	}
	// store two orders for the same window
	require.NoError(t, ordersStore.Enqueue(info))
	require.NoError(t, ordersStore.Enqueue(info))

	// check that we can see both orders tomorrow
	unsent, err = ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 1)
	require.Len(t, unsent[satellite].InfoList, 2)

	// corrupt unsent orders file by removing the last byte
	err = filepath.Walk(filepath.Join(dirName, "unsent"), func(path string, info os.FileInfo, err error) error {
		require.NoError(t, err)
		if info.IsDir() {
			return nil
		}
		err = os.Truncate(path, info.Size()-1)
		return err
	})
	require.NoError(t, err)

	// only the second order should be corrupted, so we should still see one order
	unsent, err = ordersStore.ListUnsentBySatellite(tomorrow)
	require.NoError(t, err)
	require.Len(t, unsent, 1)
	require.Len(t, unsent[satellite].InfoList, 1)
}

func verifyInfosEqual(t *testing.T, a, b *orders.Info) {
	t.Helper()

	require.NotNil(t, a)
	require.NotNil(t, b)

	require.Equal(t, a.Limit.SerialNumber, b.Limit.SerialNumber)
	require.Equal(t, a.Limit.SatelliteId, b.Limit.SatelliteId)
	require.Equal(t, a.Limit.OrderExpiration.UTC(), b.Limit.OrderExpiration.UTC())
	require.Equal(t, a.Limit.Action, b.Limit.Action)

	require.Equal(t, a.Order.Amount, b.Order.Amount)
	require.Equal(t, a.Order.SerialNumber, b.Order.SerialNumber)
}

func verifyArchivedInfosEqual(t *testing.T, a, b *orders.ArchivedInfo) {
	t.Helper()

	require.NotNil(t, a)
	require.NotNil(t, b)

	require.Equal(t, a.Limit.SerialNumber, b.Limit.SerialNumber)
	require.Equal(t, a.Limit.SatelliteId, b.Limit.SatelliteId)
	require.Equal(t, a.Limit.OrderExpiration.UTC(), b.Limit.OrderExpiration.UTC())
	require.Equal(t, a.Limit.Action, b.Limit.Action)

	require.Equal(t, a.Order.Amount, b.Order.Amount)
	require.Equal(t, a.Order.SerialNumber, b.Order.SerialNumber)

	require.Equal(t, a.Status, b.Status)
	require.Equal(t, a.ArchivedAt.UTC(), b.ArchivedAt.UTC())
}

func storeNewOrders(ordersStore *orders.FileStore, numSatellites, numOrdersPerSatPerTime int, createdAtTimes []time.Time) (map[storj.SerialNumber]*orders.Info, error) {
	actions := []pb.PieceAction{
		pb.PieceAction_GET,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_GET_AUDIT,
	}
	originalInfos := make(map[storj.SerialNumber]*orders.Info)
	for i := 0; i < numSatellites; i++ {
		satellite := testrand.NodeID()

		for _, createdAt := range createdAtTimes {
			for j := 0; j < numOrdersPerSatPerTime; j++ {
				expiration := time.Now().Add(time.Hour)
				amount := testrand.Int63n(1000)
				sn := testrand.SerialNumber()
				action := actions[j%len(actions)]

				newInfo := &orders.Info{
					Limit: &pb.OrderLimit{
						SerialNumber:    sn,
						SatelliteId:     satellite,
						Action:          action,
						OrderCreation:   createdAt,
						OrderExpiration: expiration,
					},
					Order: &pb.Order{
						SerialNumber: sn,
						Amount:       amount,
					},
				}
				originalInfos[sn] = newInfo

				// store the new info in the orders store
				err := ordersStore.Enqueue(newInfo)
				if err != nil {
					return originalInfos, err
				}
			}
		}
	}
	return originalInfos, nil
}
