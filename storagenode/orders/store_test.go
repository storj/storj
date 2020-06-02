// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package orders_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/orders"
)

func TestOrdersStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	dirName := ctx.Dir("test-orders")

	ordersStore := orders.NewFileStore(dirName)

	numSatellites := 3
	serialsPerSat := 5
	originalInfos, err := storeNewOrders(ordersStore, numSatellites, serialsPerSat)
	require.NoError(t, err)

	unsentMap, err := ordersStore.ListUnsentBySatellite()
	require.NoError(t, err)

	// go through order limits and make sure information is accurate
	require.Len(t, unsentMap, numSatellites)
	for _, unsentSatList := range unsentMap {
		require.Len(t, unsentSatList, serialsPerSat)

		for _, unsentInfo := range unsentSatList {
			sn := unsentInfo.Limit.SerialNumber
			originalInfo := originalInfos[sn]

			verifyInfosEqual(t, unsentInfo, originalInfo)
		}
	}

	// add some more orders and list again
	// we should see everything we added before, plus the new ones
	newInfos, err := storeNewOrders(ordersStore, numSatellites, serialsPerSat)
	require.NoError(t, err)
	for sn, info := range newInfos {
		originalInfos[sn] = info
	}
	// because we have stored two times, we have twice as  many serials...
	require.Len(t, originalInfos, 2*numSatellites*serialsPerSat)

	unsentMap, err = ordersStore.ListUnsentBySatellite()
	require.NoError(t, err)
	// ...and twice as many satellites.
	require.Len(t, unsentMap, 2*numSatellites)
	for _, unsentSatList := range unsentMap {
		require.Len(t, unsentSatList, serialsPerSat)

		for _, unsentInfo := range unsentSatList {
			sn := unsentInfo.Limit.SerialNumber
			originalInfo := originalInfos[sn]

			verifyInfosEqual(t, unsentInfo, originalInfo)
		}
	}

	// now, add another order, delete ready to send files, and list
	// we should only see the new order
	originalInfos, err = storeNewOrders(ordersStore, 1, 1)
	require.NoError(t, err)

	err = ordersStore.DeleteReadyToSendFiles()
	require.NoError(t, err)

	unsentMap, err = ordersStore.ListUnsentBySatellite()
	require.NoError(t, err)
	require.Len(t, unsentMap, 1)
	for _, unsentSatList := range unsentMap {
		require.Len(t, unsentSatList, 1)
		for _, unsentInfo := range unsentSatList {
			sn := unsentInfo.Limit.SerialNumber
			originalInfo := originalInfos[sn]

			verifyInfosEqual(t, unsentInfo, originalInfo)
		}
	}

	// TODO test order archival
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

func storeNewOrders(ordersStore *orders.FileStore, numSatellites, numOrdersPerSatellite int) (map[storj.SerialNumber]*orders.Info, error) {
	actions := []pb.PieceAction{
		pb.PieceAction_GET,
		pb.PieceAction_PUT_REPAIR,
		pb.PieceAction_GET_AUDIT,
	}
	originalInfos := make(map[storj.SerialNumber]*orders.Info)
	for i := 0; i < numSatellites; i++ {
		satellite := testrand.NodeID()

		// for each satellite, half of the orders will expire in an hour
		// and half will expire in three hours.
		for j := 0; j < numOrdersPerSatellite; j++ {
			expiration := time.Now().Add(time.Hour)
			if j < 2 {
				expiration = time.Now().Add(3 * time.Hour)
			}
			amount := testrand.Int63n(1000)
			sn := testrand.SerialNumber()
			action := actions[j%len(actions)]

			newInfo := &orders.Info{
				Limit: &pb.OrderLimit{
					SerialNumber:    sn,
					SatelliteId:     satellite,
					Action:          action,
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
	return originalInfos, nil
}
