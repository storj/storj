// Copyright (C) 2019 Storj Labs, Inc.
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

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/heldamount"
)

func TestHeldAmountApi(t *testing.T) {
	testplanet.Run(t,
		testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 1,
		},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			satellite := planet.Satellites[0]
			sno := planet.StorageNodes[0]
			console := sno.Console
			heldAmountDB := sno.DB.HeldAmount()
			baseURL := fmt.Sprintf("http://%s/api/heldamount", console.Listener.Addr())

			period := "2020-03"
			paystub := heldamount.PayStub{
				SatelliteID:    satellite.ID(),
				Period:         period,
				Created:        time.Now().UTC(),
				Codes:          "qwe",
				UsageAtRest:    1,
				UsageGet:       2,
				UsagePut:       3,
				UsageGetRepair: 4,
				UsagePutRepair: 5,
				UsageGetAudit:  6,
				CompAtRest:     7,
				CompGet:        8,
				CompPut:        9,
				CompGetRepair:  10,
				CompPutRepair:  11,
				CompGetAudit:   12,
				SurgePercent:   13,
				Held:           14,
				Owed:           15,
				Disposed:       16,
				Paid:           17,
			}
			err := heldAmountDB.StorePayStub(ctx, paystub)
			require.NoError(t, err)

			t.Run("test SatellitePayStubMonthly", func(t *testing.T) {
				// should return paystub inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, period, satellite.ID().String())
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]heldamount.PayStub{paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				// should return 404 cause no payout for the period.
				url = fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, "2020-01", satellite.ID().String())
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusNotFound, res2.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				expected = []byte("{\"error\":\"heldAmount console web error: heldamount service error: no payStub for period error: sql: no rows in result set\"}\n")
				require.Equal(t, expected, body2)

				// should return 400 cause of wrong satellite id.
				url = fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, "2020-01", "123")
				res3, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res3)
				require.Equal(t, http.StatusBadRequest, res3.StatusCode)

				defer func() {
					err = res3.Body.Close()
					require.NoError(t, err)
				}()
			})

			paystub2 := heldamount.PayStub{
				SatelliteID:    storj.NodeID{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 0},
				Period:         period,
				Created:        time.Now().UTC(),
				Codes:          "qwe",
				UsageAtRest:    1,
				UsageGet:       2,
				UsagePut:       3,
				UsageGetRepair: 4,
				UsagePutRepair: 5,
				UsageGetAudit:  6,
				CompAtRest:     7,
				CompGet:        8,
				CompPut:        9,
				CompGetRepair:  10,
				CompPutRepair:  11,
				CompGetAudit:   12,
				SurgePercent:   13,
				Held:           14,
				Owed:           15,
				Disposed:       16,
				Paid:           17,
			}
			err = heldAmountDB.StorePayStub(ctx, paystub2)
			require.NoError(t, err)

			t.Run("test AllPayStubsMonthly", func(t *testing.T) {
				// should return 2 paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s", baseURL, period)
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]heldamount.PayStub{paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				// should return 2 paystubs inserted earlier
				url = fmt.Sprintf("%s/paystubs/s%s", baseURL, "2020-01")
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, "null\n", string(body2))
			})

			period2 := "2020-02"
			paystub3 := heldamount.PayStub{
				SatelliteID:    satellite.ID(),
				Period:         period2,
				Created:        time.Now().UTC(),
				Codes:          "qwe",
				UsageAtRest:    1,
				UsageGet:       2,
				UsagePut:       3,
				UsageGetRepair: 4,
				UsagePutRepair: 5,
				UsageGetAudit:  6,
				CompAtRest:     7,
				CompGet:        8,
				CompPut:        9,
				CompGetRepair:  10,
				CompPutRepair:  11,
				CompGetAudit:   12,
				SurgePercent:   13,
				Held:           14,
				Owed:           15,
				Disposed:       16,
				Paid:           17,
			}
			err = heldAmountDB.StorePayStub(ctx, paystub3)
			require.NoError(t, err)

			t.Run("test SatellitePayStubPeriod", func(t *testing.T) {
				// should return all paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, satellite.ID().String())
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]heldamount.PayStub{paystub3, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period, period, satellite.ID().String())
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				expected, err = json.Marshal([]heldamount.PayStub{paystub})
				require.NoError(t, err)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body2))

				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, paystub2.SatelliteID.String())
				res3, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res3)
				require.Equal(t, http.StatusOK, res3.StatusCode)

				expected, err = json.Marshal([]heldamount.PayStub{paystub2})
				require.NoError(t, err)

				defer func() {
					err = res3.Body.Close()
					require.NoError(t, err)
				}()
				body3, err := ioutil.ReadAll(res3.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body3))

				// should return 400 because of bad satellite id.
				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, "1")
				res4, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res4)
				require.Equal(t, http.StatusBadRequest, res4.StatusCode)

				defer func() {
					err = res4.Body.Close()
					require.NoError(t, err)
				}()

				// should return 400 because of bad period.
				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period, period2, satellite.ID().String())
				res5, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res5)
				require.Equal(t, http.StatusBadRequest, res5.StatusCode)

				defer func() {
					err = res5.Body.Close()
					require.NoError(t, err)
				}()

				body5, err := ioutil.ReadAll(res5.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"error\":\"heldAmount console web error: wrong period format: period has wrong format\"}\n", string(body5))
			})

			t.Run("test AllPayStubsPeriod", func(t *testing.T) {
				// should return all paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period2, period)
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]heldamount.PayStub{paystub3, paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				url = fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period, period)
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				expected, err = json.Marshal([]heldamount.PayStub{paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body2))

				// should return 400 because of bad period.
				url = fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period, period2)
				res5, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res5)
				require.Equal(t, http.StatusBadRequest, res5.StatusCode)

				defer func() {
					err = res5.Body.Close()
					require.NoError(t, err)
				}()

				body5, err := ioutil.ReadAll(res5.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"error\":\"heldAmount console web error: wrong period format: period has wrong format\"}\n", string(body5))
			})

			t.Run("test HeldbackHistory", func(t *testing.T) {
				// should return all heldback history inserted earlier
				url := fmt.Sprintf("%s/heldback/%s", baseURL, satellite.ID().String())
				res, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				period75 := heldamount.HeldbackPeriod{
					PercentageRate: 75,
					Held:           paystub2.Held + paystub3.Held,
				}

				var periods []heldamount.HeldbackPeriod
				periods = append(periods, period75)

				expected, err := json.Marshal(periods)
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := ioutil.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				// should return 400 because of bad period.
				url = fmt.Sprintf("%s/heldback/%s", baseURL, satellite.ID().String()+"11")
				res2, err := http.Get(url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusBadRequest, res2.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()

				body2, err := ioutil.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"error\":\"heldAmount console web error: node ID error: checksum error\"}\n", string(body2))
			})
		},
	)
}
