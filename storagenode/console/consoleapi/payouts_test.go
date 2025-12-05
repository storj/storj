// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/payouts"
	"storj.io/storj/storagenode/reputation"
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
			payoutsDB := sno.DB.Payout()
			reputationDB := sno.DB.Reputation()
			satellitesDB := sno.DB.Satellites()
			baseURL := fmt.Sprintf("http://%s/api/heldamount", console.Listener.Addr())

			// pause nodestats reputation cache because later tests assert a specific joinedat.
			sno.Reputation.Chore.Loop.Pause()

			period := "2020-03"
			paystub := payouts.PayStub{
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
			err := payoutsDB.StorePayStub(ctx, paystub)
			require.NoError(t, err)

			t.Run("SatellitePayStubMonthly", func(t *testing.T) {
				// should return paystub inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, period, satellite.ID().String())
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				paystub.UsageAtRest /= 720

				expected, err := json.Marshal(paystub)
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				// should return 404 cause no payouts for the period.
				url = fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, "2020-01", satellite.ID().String())
				res2, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := io.ReadAll(res2.Body)
				require.NoError(t, err)

				expected = []byte("null\n")
				require.Equal(t, expected, body2)

				// should return 400 cause of wrong satellite id.
				url = fmt.Sprintf("%s/paystubs/%s?id=%s", baseURL, "2020-01", "123")
				res3, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res3)
				require.Equal(t, http.StatusBadRequest, res3.StatusCode)

				defer func() {
					err = res3.Body.Close()
					require.NoError(t, err)
				}()
			})

			paystub2 := payouts.PayStub{
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
			err = payoutsDB.StorePayStub(ctx, paystub2)
			require.NoError(t, err)

			t.Run("AllPayStubsMonthly", func(t *testing.T) {
				// should return 2 paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s", baseURL, period)
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				paystub2.UsageAtRest /= 720

				expected, err := json.Marshal([]payouts.PayStub{paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				// should return 2 paystubs inserted earlier
				url = fmt.Sprintf("%s/paystubs/%s", baseURL, "2020-01")
				res2, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := io.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, "null\n", string(body2))
			})

			period2 := "2020-02"
			paystub3 := payouts.PayStub{
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
			err = payoutsDB.StorePayStub(ctx, paystub3)
			require.NoError(t, err)

			t.Run("SatellitePayStubPeriod", func(t *testing.T) {
				// should return all paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, satellite.ID().String())
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				paystub3.UsageAtRest /= 720

				expected, err := json.Marshal([]payouts.PayStub{paystub3, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period, period, satellite.ID().String())
				res2, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				expected, err = json.Marshal([]payouts.PayStub{paystub})
				require.NoError(t, err)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := io.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body2))

				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, paystub2.SatelliteID.String())
				res3, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res3)
				require.Equal(t, http.StatusOK, res3.StatusCode)

				expected, err = json.Marshal([]payouts.PayStub{paystub2})
				require.NoError(t, err)

				defer func() {
					err = res3.Body.Close()
					require.NoError(t, err)
				}()
				body3, err := io.ReadAll(res3.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body3))

				// should return 400 because of bad satellite id.
				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period2, period, "1")
				res4, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res4)
				require.Equal(t, http.StatusBadRequest, res4.StatusCode)

				defer func() {
					err = res4.Body.Close()
					require.NoError(t, err)
				}()

				// should return 400 because of bad period.
				url = fmt.Sprintf("%s/paystubs/%s/%s?id=%s", baseURL, period, period2, satellite.ID().String())
				res5, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res5)
				require.Equal(t, http.StatusBadRequest, res5.StatusCode)

				defer func() {
					err = res5.Body.Close()
					require.NoError(t, err)
				}()

				body5, err := io.ReadAll(res5.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"error\":\"consoleapi payouts: wrong period format: period has wrong format\"}\n", string(body5))
			})

			t.Run("AllPayStubsPeriod", func(t *testing.T) {
				// should return all paystubs inserted earlier
				url := fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period2, period)
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				expected, err := json.Marshal([]payouts.PayStub{paystub3, paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				url = fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period, period)
				res2, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res2)
				require.Equal(t, http.StatusOK, res2.StatusCode)

				expected, err = json.Marshal([]payouts.PayStub{paystub2, paystub})
				require.NoError(t, err)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := io.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body2))

				// should return 400 because of bad period.
				url = fmt.Sprintf("%s/paystubs/%s/%s", baseURL, period, period2)
				res5, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res5)
				require.Equal(t, http.StatusBadRequest, res5.StatusCode)

				defer func() {
					err = res5.Body.Close()
					require.NoError(t, err)
				}()

				body5, err := io.ReadAll(res5.Body)
				require.NoError(t, err)

				require.Equal(t, "{\"error\":\"consoleapi payouts: wrong period format: period has wrong format\"}\n", string(body5))
			})

			t.Run("HeldbackHistory", func(t *testing.T) {
				date := time.Now().UTC().AddDate(0, -2, 0).Round(time.Minute)
				err = reputationDB.Store(t.Context(), reputation.Stats{
					SatelliteID: satellite.ID(),
					JoinedAt:    date,
				})
				require.NoError(t, err)

				err = satellitesDB.SetAddress(ctx, satellite.ID(), satellite.Addr())
				require.NoError(t, err)

				// should return all heldback history inserted earlier
				url := baseURL + "/held-history"
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				held := payouts.SatelliteHeldHistory{
					SatelliteID:         satellite.ID(),
					SatelliteName:       satellite.Addr(),
					HoldForFirstPeriod:  28,
					HoldForSecondPeriod: 0,
					HoldForThirdPeriod:  0,
					TotalHeld:           28,
					TotalDisposed:       32,
					JoinedAt:            date.Round(time.Minute),
				}

				var periods []payouts.SatelliteHeldHistory
				periods = append(periods, held)

				expected, err := json.Marshal(periods)
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.Equal(t, string(expected)+"\n", string(body))
			})

			t.Run("Periods", func(t *testing.T) {
				url := baseURL + "/periods"
				res, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				var periods []string
				periods = append(periods, "2020-03", "2020-02")

				expected, err := json.Marshal(periods)
				require.NoError(t, err)

				defer func() {
					err = res.Body.Close()
					require.NoError(t, err)
				}()
				body, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected)+"\n", string(body))

				//
				url = fmt.Sprintf("%s/periods?id=%s", baseURL, paystub2.SatelliteID.String())
				res2, err := httpGet(ctx, url)
				require.NoError(t, err)
				require.NotNil(t, res)
				require.Equal(t, http.StatusOK, res.StatusCode)

				var periods2 []string
				periods2 = append(periods2, "2020-03")

				expected2, err := json.Marshal(periods2)
				require.NoError(t, err)

				defer func() {
					err = res2.Body.Close()
					require.NoError(t, err)
				}()
				body2, err := io.ReadAll(res2.Body)
				require.NoError(t, err)

				require.Equal(t, string(expected2)+"\n", string(body2))
			})
		},
	)
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

func httpPost(ctx context.Context, url string, contentType string, b io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return http.DefaultClient.Do(req)
}
