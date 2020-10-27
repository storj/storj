// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

func TestAPI(t *testing.T) {
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
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		project, err := sat.DB.Console().Projects().Get(ctx, planet.Uplinks[0].Projects[0].ID)
		require.NoError(t, err)

		link := "http://" + address.String() + "/api/project/" + project.ID.String()
		linkLimit := link + "/limit"

		t.Run("GetProject", func(t *testing.T) {
			require.NoError(t, err)
			expected := fmt.Sprintf(
				`{"id":"%s","name":"%s","description":"%s","partnerId":"%s","ownerId":"%s","rateLimit":null,"maxBuckets":null,"createdAt":"%s"}`,
				project.ID.String(),
				project.Name,
				project.Description,
				project.PartnerID.String(),
				project.OwnerID.String(),
				project.CreatedAt.Format(time.RFC3339Nano),
			)
			assertGet(t, link, expected, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("GetProjectLimits", func(t *testing.T) {
			assertGet(t, linkLimit, `{"usage":{"amount":"0 B","bytes":0},"bandwidth":{"amount":"0 B","bytes":0},"rate":{"rps":0},"maxBuckets":0}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("UpdateUsage", func(t *testing.T) {
			data := url.Values{"usage": []string{"1TiB"}}
			req, err := http.NewRequest(http.MethodPost, linkLimit, strings.NewReader(data.Encode()))
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, linkLimit, `{"usage":{"amount":"1.0 TiB","bytes":1099511627776},"bandwidth":{"amount":"0 B","bytes":0},"rate":{"rps":0},"maxBuckets":0}`, planet.Satellites[0].Config.Console.AuthToken)

			req, err = http.NewRequest(http.MethodPut, linkLimit+"?usage=1GB", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"0 B","bytes":0},"rate":{"rps":0},"maxBuckets":0}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("UpdateBandwidth", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPut, linkLimit+"?bandwidth=1MB", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":0},"maxBuckets":0}`, planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("UpdateRate", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPut, linkLimit+"?rate=100", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":100},"maxBuckets":0}`, planet.Satellites[0].Config.Console.AuthToken)
		})
		t.Run("UpdateBuckets", func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPut, linkLimit+"?buckets=2000", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(t, linkLimit, `{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"rate":{"rps":100},"maxBuckets":2000}`, planet.Satellites[0].Config.Console.AuthToken)
		})
	})
}

func TestAddProject(t *testing.T) {
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
		userID := planet.Uplinks[0].Projects[0].Owner

		body := strings.NewReader(fmt.Sprintf(`{"ownerId":"%s","projectName":"Test Project"}`, userID.ID.String()))
		req, err := http.NewRequest(http.MethodPost, "http://"+address.String()+"/api/project", body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		responseBody, err := ioutil.ReadAll(response.Body)
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

func TestRenameProject(t *testing.T) {
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
		userID := planet.Uplinks[0].Projects[0].Owner
		oldName, newName := "renameTest", "Test Project"

		project, err := planet.Satellites[0].AddProject(ctx, userID.ID, oldName)
		require.NoError(t, err)
		require.Equal(t, oldName, project.Name)

		body := strings.NewReader(fmt.Sprintf(`{"projectName":"%s","description":"This project got renamed"}`, newName))
		req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://"+address.String()+"/api/project/%s", project.ID.String()), body)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.NoError(t, response.Body.Close())

		project, err = planet.Satellites[0].DB.Console().Projects().Get(ctx, project.ID)
		require.NoError(t, err)
		require.Equal(t, newName, project.Name)
	})
}

func TestDeleteProject(t *testing.T) {
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
		projectID := planet.Uplinks[0].Projects[0].ID

		// Ensure there are no buckets left
		buckets, err := planet.Satellites[0].DB.Buckets().ListBuckets(ctx, projectID, storj.BucketListOptions{Limit: 1, Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
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
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/project/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apikeys.APIKeys[0].ID)
		require.NoError(t, err)

		req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/project/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)

		project, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectID)
		require.Error(t, err)
		require.Nil(t, project)
	})
}

func TestCheckUsageWithoutUsage(t *testing.T) {
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

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/project/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"result\":\"no project usage exist\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
	})
}

func TestCheckUsageWithUsage(t *testing.T) {
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
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/project/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for current month exists\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)
	})
}

func TestCheckUsageLastMonthUnappliedInvoice(t *testing.T) {
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
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		planet.Satellites[0].API.Payments.Service.SetNow(func() time.Time {
			return oneMonthAhead
		})
		err = planet.Satellites[0].API.Payments.Service.PrepareInvoiceProjectRecords(ctx, now)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://"+address.String()+"/api/project/%s/usage", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"unapplied project invoice record exist\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)
	})
}

func TestDeleteProjectWithUsageCurrentMonth(t *testing.T) {
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
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/project/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for current month exists\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)
	})
}

func TestDeleteProjectWithUsagePreviousMonth(t *testing.T) {
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
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      accTime,
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:         "test",
			ProjectID:          projectID,
			IntervalStart:      accTime.AddDate(0, 0, 1),
			ObjectCount:        1,
			InlineSegmentCount: 1,
			RemoteSegmentCount: 1,
			InlineBytes:        10,
			RemoteBytes:        640000,
			MetadataSize:       2,
		}
		err = planet.Satellites[0].DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://"+address.String()+"/api/project/%s", projectID), nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := ioutil.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"usage for last month exist, but is not billed yet\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)
	})
}

func assertGet(t *testing.T, link string, expected string, authToken string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, link, nil)
	require.NoError(t, err)

	req.Header.Set("Authorization", authToken)

	response, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	data, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())

	require.Equal(t, http.StatusOK, response.StatusCode, string(data))
	require.Equal(t, expected, string(data))
}
