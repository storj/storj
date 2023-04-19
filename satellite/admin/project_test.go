// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/stripe"
)

func TestProjectGet(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		t.Run("OK", func(t *testing.T) {
			link := "http://" + address.String() + "/api/projects/" + project.ID.String()
			expected := fmt.Sprintf(
				`{"id":"%s","publicId":"%s","name":"%s","description":"%s","userAgent":null,"ownerId":"%s","rateLimit":null,"burstLimit":null,"maxBuckets":null,"createdAt":"%s","memberCount":0,"storageLimit":"25.00 GB","bandwidthLimit":"25.00 GB","userSpecifiedStorageLimit":null,"userSpecifiedBandwidthLimit":null,"segmentLimit":10000}`,
				project.ID.String(),
				project.PublicID.String(),
				project.Name,
				project.Description,
				project.OwnerID.String(),
				project.CreatedAt.Format(time.RFC3339Nano),
			)
			assertGet(ctx, t, link, expected, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := "http://" + address.String() + "/api/projects/" + id.String() + "/limit"
			body := assertReq(ctx, t, link, http.MethodGet, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(body), "does not exist")
		})
	})
}

func TestProjectLimit(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		linkLimit := "http://" + address.String() + "/api/projects/" + project.ID.String() + "/limit"

		t.Run("Get OK", func(t *testing.T) {
			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"25.00 GB","bytes":25000000000},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"rate":{"rps":0},"maxBuckets":0,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Get Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := "http://" + address.String() + "/api/projects/" + id.String() + "/limit"
			body := assertReq(ctx, t, link, http.MethodGet, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(body), "does not exist")
		})

		t.Run("Update Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := "http://" + address.String() + "/api/projects/" + id.String() + "/limit?usage=100000000"
			body := assertReq(ctx, t, link, http.MethodPut, "", http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(body), "does not exist")
		})

		t.Run("Update Nothing", func(t *testing.T) {
			expectedBody := assertReq(ctx, t, linkLimit, http.MethodGet, "", http.StatusOK, "", planet.Satellites[0].Config.Console.AuthToken)

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, linkLimit, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, string(expectedBody), planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Usage", func(t *testing.T) {
			data := url.Values{"usage": []string{"1TiB"}}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, linkLimit, strings.NewReader(data.Encode()))
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.0 TiB","bytes":1099511627776},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"rate":{"rps":0},"maxBuckets":0,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)

			req, err = http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?usage=1GB", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"rate":{"rps":0},"maxBuckets":0,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Bandwidth", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?bandwidth=1MB", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":0},"maxBuckets":0,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Rate", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rate=100", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":100},"maxBuckets":0,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Buckets", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?buckets=2000", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":100},"maxBuckets":2000,"maxSegments":10000}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Segment Limit", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?segments=500", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":100},"maxBuckets":2000,"maxSegments":500}`, planet.Satellites[0].Config.Console.AuthToken)
		})
	})
}

func TestProjectAdd(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		userID := planet.Uplinks[0].Projects[0].Owner

		body := strings.NewReader(fmt.Sprintf(`{"ownerId":"%s","projectName":"Test Project"}`, userID.ID.String()))
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/projects", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())

		var output struct {
			ProjectID uuid.UUID `json:"projectId"`
		}

		err = json.Unmarshal(responseBody, &output)
		require.NoError(t, err)

		project, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, output.ProjectID)
		require.NoError(t, err)
		require.Equal(t, "Test Project", project.Name)
	})
}

func TestProjectRename(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		userID := planet.Uplinks[0].Projects[0].Owner
		oldName, newName := "renameTest", "Test Project"

		project, err := planet.Satellites[0].AddProject(ctx, userID.ID, oldName)
		require.NoError(t, err)
		require.Equal(t, oldName, project.Name)

		t.Run("OK", func(t *testing.T) {
			body := strings.NewReader(fmt.Sprintf(`{"projectName":"%s","description":"This project got renamed"}`, newName))
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", project.ID.String()), body)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.Equal(t, "", response.Header.Get("Content-Type"))
			require.NoError(t, response.Body.Close())

			project, err = planet.Satellites[0].DB.Console().Projects().Get(ctx, project.ID)
			require.NoError(t, err)
			require.Equal(t, newName, project.Name)
		})

		t.Run("Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := fmt.Sprintf("http://"+address.String()+"/api/projects/%s", id.String())
			putBody := fmt.Sprintf(`{"projectName":"%s","description":"This project got renamed"}`, newName)
			body := assertReq(ctx, t, link, http.MethodPut, putBody, http.StatusNotFound, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(body), "does not exist")
		})
	})
}

func TestProjectDelete(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		// Ensure there are no buckets left
		buckets, err := planet.Satellites[0].API.Buckets.Service.ListBuckets(ctx, projectID, buckets.ListOptions{Limit: 1, Direction: buckets.DirectionForward}, macaroon.AllowedBuckets{All: true})
		require.NoError(t, err)
		require.Len(t, buckets.Items, 0)

		apikeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apikeys.APIKeys, 1)

		// the deletion with an existing API key should fail
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apikeys.APIKeys[0].ID)
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "", response.Header.Get("Content-Type"))

		project, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectID)
		require.Error(t, err)
		require.Nil(t, project)
	})
}

func TestProjectCheckUsage_withoutUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"result\":\"no project usage exist\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}

func TestProjectCheckUsage_withUsage(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		now := time.Now().UTC()
		// use fixed intervals to avoid issues at the beginning of the month
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		// Ensure User is free tier.
		paid, err := planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.False(t, paid)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"result\":\"no project usage exist\"}", string(responseBody))
		require.NoError(t, response.Body.Close())

		// Make User paid tier.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3)
		require.NoError(t, err)

		// Ensure User is paid tier.
		paid, err = planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.True(t, paid)

		req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for current month exists\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}

func TestProjectCheckUsage_lastMonthUnappliedInvoice(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		now := time.Date(2030, time.Month(9), 1, 0, 0, 0, 0, time.UTC)

		oneMonthAhead := now.AddDate(0, 1, 0)
		planet.Satellites[0].Admin.Admin.Server.SetNow(func() time.Time {
			return oneMonthAhead
		})

		// use fixed intervals to avoid issues at the beginning of the month
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		planet.Satellites[0].API.Payments.StripeService.SetNow(func() time.Time {
			return oneMonthAhead
		})
		err = planet.Satellites[0].API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, now)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)

		require.Equal(t, "application/json", response.Header.Get("Content-Type"))
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"unapplied project invoice record exist\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}

// TestProjectDelete_withUsageCurrentMonth first tries to delete an actively used project of a paid tier user, which
// should fail and afterwards converts the user to free tier and tries the deletion again. That deletion should succeed.
func TestProjectDelete_withUsageCurrentMonth(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		now := time.Now().UTC()
		// use fixed intervals to avoid issues at the beginning of the month
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		// Make User paid tier.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3)
		require.NoError(t, err)

		// Ensure User is paid tier.
		paid, err := planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.True(t, paid)

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, response.StatusCode)
		require.Equal(t, "application/json", response.Header.Get("Content-Type"))

		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for current month exists\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())

		// Make User free tier again.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, false, memory.TB, memory.TB, 100000, 3)
		require.NoError(t, err)

		// Ensure User is free tier.
		paid, err = planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.False(t, paid)

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}

// TestProjectDelete_withUsageCurrentMonthUncharged first tries to delete a last month used project of a paid tier user, which
// should fail and afterwards converts the user to free tier and tries the deletion again. That deletion should succeed.
// This test ensures we bill paid tier users for past months usage and do not forget to do so.
func TestProjectDelete_withUsagePreviousMonthUncharged(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		// TODO: Improve updating of DB entries
		now := time.Now().UTC()
		// set fixed day to avoid failures at the end of the month
		accTime := time.Date(now.Year(), now.Month()-1, 15, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), time.UTC)
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     accTime,
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     accTime.AddDate(0, 0, 1),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		// Make User paid tier.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3)
		require.NoError(t, err)

		// Ensure User is paid tier.
		paid, err := planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.True(t, paid)

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for last month exist, but is not billed yet\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Make User free tier again.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, false, memory.TB, memory.TB, 100000, 3)
		require.NoError(t, err)

		// Ensure User is free tier.
		paid, err = planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.False(t, paid)

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		// Project should be fine to delete now.
		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}

// TestProjectDelete_withUsageCurrentMonthCharged tries to delete a last month used project of a paid tier user, which
// should fail without an accommodating invoice record. This tests creates the corresponding entry and should afterwards
// the deletion of the project.
func TestProjectDelete_withUsagePreviousMonthCharged(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		})
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		// TODO: Improve updating of DB entries
		now := time.Now().UTC()
		// set fixed day to avoid failures at the end of the month
		accTime := time.Date(now.Year(), now.Month()-1, 15, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), time.UTC)
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     accTime,
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         projectID,
			IntervalStart:     accTime.AddDate(0, 0, 1),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		// Make User paid tier.
		err = planet.Satellites[0].DB.Console().Users().UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3)
		require.NoError(t, err)

		// Ensure User is paid tier.
		paid, err := planet.Satellites[0].DB.Console().Users().GetUserPaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.True(t, paid)

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for last month exist, but is not billed yet\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Create Invoice Record for last month.
		firstOfMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		err = planet.Satellites[0].DB.StripeCoinPayments().ProjectRecords().Create(ctx,
			[]stripe.CreateProjectRecord{{
				ProjectID: projectID,
				Storage:   100,
				Egress:    100,
				Segments:  100}}, firstOfMonth, firstOfMonth.AddDate(0, 1, 0))
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		// Project should fail to delete since the project record has not been used/billed yet.
		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"unapplied project invoice record exist\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Retrieve its ID and consume(mark as billed) it.
		dbRecord, err := planet.Satellites[0].DB.StripeCoinPayments().ProjectRecords().Get(ctx, projectID, firstOfMonth, firstOfMonth.AddDate(0, 1, 0))
		require.NoError(t, err)
		require.Equal(t, projectID, dbRecord.ProjectID)
		err = planet.Satellites[0].DB.StripeCoinPayments().ProjectRecords().Consume(ctx, dbRecord.ID)
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		// Project should be fine to delete now.
		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)

		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "", string(responseBody))
		require.NoError(t, response.Body.Close())
	})
}
