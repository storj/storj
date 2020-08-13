// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments"
)

func TestAddCoupon(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		body := strings.NewReader(fmt.Sprintf(`{"userId": "%s", "duration": 2, "amount": 3000, "description": "testcoupon-alice"}`, user.ID))
		req, err := http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/coupon", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output uuid.UUID

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		coupon, err := planet.Satellites[0].DB.StripeCoinPayments().Coupons().Get(ctx, output)
		require.NoError(t, err)
		require.Equal(t, user.ID, coupon.UserID)
		require.Equal(t, 2, coupon.Duration)
		require.Equal(t, "testcoupon-alice", coupon.Description)
		require.Equal(t, int64(3000), coupon.Amount)
	})
}

func TestCouponInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		var output payments.Coupon
		var id uuid.UUID

		body := strings.NewReader(fmt.Sprintf(`{"userId": "%s", "duration": 2, "amount": 3000, "description": "testcoupon-alice"}`, user.ID))
		req, err := http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/coupon", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		err = json.Unmarshal(responseBody, &id)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/coupon/%s", id.String()), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err = ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)
		require.Equal(t, id, output.ID)
		require.Equal(t, 2, output.Duration)
		require.Equal(t, int64(3000), output.Amount)
		require.Equal(t, "testcoupon-alice", output.Description)
	})
}

func TestCouponDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		user, err := planet.Satellites[0].DB.Console().Users().GetByEmail(ctx, planet.Uplinks[0].Projects[0].Owner.Email)
		require.NoError(t, err)

		var id uuid.UUID

		body := strings.NewReader(fmt.Sprintf(`{"userId": "%s", "duration": 2, "amount": 3000, "description": "testcoupon-alice"}`, user.ID))
		req, err := http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/coupon", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		err = json.Unmarshal(responseBody, &id)
		require.NoError(t, err)

		coupons, err := planet.Satellites[0].DB.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
		require.NoError(t, err)
		// each created user have always one coupon already
		require.Len(t, coupons, 2)

		req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/coupon/%s", id), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)

		coupons, err = planet.Satellites[0].DB.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
		require.NoError(t, err)
		require.Len(t, coupons, 1)
	})
}
