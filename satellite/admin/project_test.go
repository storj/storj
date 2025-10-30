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
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/attribution"
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
				`{"id":"%s","publicId":"%s","name":"%s","description":"%s","userAgent":null,"ownerId":"%s","maxBuckets":null,"createdAt":"%s","memberCount":0,"status":1,"storageLimit":"25.00 GB","bandwidthLimit":"25.00 GB","userSpecifiedStorageLimit":null,"userSpecifiedBandwidthLimit":null,"segmentLimit":10000,"rateLimit":null,"burstLimit":null,"defaultPlacement":0,"defaultVersioning":1,"isClassic":false}`,
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
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodGet,
				"",
				http.StatusNotFound,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(body), "does not exist")
		})
	})
}

func TestProjectGetByAnyID(t *testing.T) {
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

		testGetProject := func(pid string) {
			link := "http://" + address.String() + "/api/projects/" + pid
			expected := fmt.Sprintf(
				`{"id":"%s","publicId":"%s","name":"%s","description":"%s","userAgent":null,"ownerId":"%s","maxBuckets":null,"createdAt":"%s","memberCount":0,"status":1,"storageLimit":"25.00 GB","bandwidthLimit":"25.00 GB","userSpecifiedStorageLimit":null,"userSpecifiedBandwidthLimit":null,"segmentLimit":10000,"rateLimit":null,"burstLimit":null,"defaultPlacement":0,"defaultVersioning":1,"isClassic":false}`,
				project.ID.String(),
				project.PublicID.String(),
				project.Name,
				project.Description,
				project.OwnerID.String(),
				project.CreatedAt.Format(time.RFC3339Nano),
			)
			assertGet(ctx, t, link, expected, planet.Satellites[0].Config.Console.AuthToken)
		}

		// should work with either public or private ID
		t.Run("Get by public ID", func(t *testing.T) {
			testGetProject(project.PublicID.String())
		})
		t.Run("Get by private ID", func(t *testing.T) {
			testGetProject(project.ID.String())
		})
		// should work even if provided UUID does not contain dashes
		t.Run("Get by public ID no dashes", func(t *testing.T) {
			publicIDNoDashes := strings.ReplaceAll(project.PublicID.String(), "-", "")
			testGetProject(publicIDNoDashes)
		})
		t.Run("Get by private ID no dashes", func(t *testing.T) {
			privateIDNoDashes := strings.ReplaceAll(project.ID.String(), "-", "")
			testGetProject(privateIDNoDashes)
		})
	})
}

func TestProjectLimitGet(t *testing.T) {
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
			assertGet(
				ctx,
				t,
				linkLimit,
				`{"usage":{"amount":"25.00 GB","bytes":25000000000},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
				planet.Satellites[0].Config.Console.AuthToken,
			)
		})

		t.Run("Get Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := "http://" + address.String() + "/api/projects/" + id.String() + "/limit"
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodGet,
				"",
				http.StatusNotFound,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(body), "does not exist")
		})
	})
}

func TestProjectLimitUpdate(t *testing.T) {
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

		t.Run("Update Not Found", func(t *testing.T) {
			id, err := uuid.New()
			require.NoError(t, err)

			link := "http://" + address.String() + "/api/projects/" + id.String() + "/limit?usage=100000000"
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodPut,
				"",
				http.StatusNotFound,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(body), "does not exist")
		})

		t.Run("Update Nothing", func(t *testing.T) {
			expectedBody := assertReq(
				ctx,
				t,
				linkLimit,
				http.MethodGet,
				"",
				http.StatusOK,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)

			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(ctx, t, linkLimit, string(expectedBody), planet.Satellites[0].Config.Console.AuthToken)
		})

		t.Run("Update Bandwidth", func(t *testing.T) {
			t.Run("without units", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?usage=35000000000", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"35.00 GB","bytes":35000000000},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("with units", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?usage=1GB", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"25.00 GB","bytes":25000000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})

		t.Run("Update Bandwidth", func(t *testing.T) {
			t.Run("without unit", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?bandwidth=99000000", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"99.00 MB","bytes":99000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("with unit", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?bandwidth=1MB", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})

		t.Run("Update Rate", func(t *testing.T) {
			t.Run("set not zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rate=100", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":100,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set default", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rate=-1", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":null,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rate=0", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":0,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})

		t.Run("Update Burst", func(t *testing.T) {
			t.Run("set not 0", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?burst=50", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":0,"burst":50,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set default", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?burst=-99", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":0,"burst":null,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set 0", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?burst=0", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})

		t.Run("Update Buckets", func(t *testing.T) {
			t.Run("set not zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?buckets=2000", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":2000,"maxSegments":10000,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set not zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?buckets=-38", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":null,"maxSegments":10000,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?buckets=0", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":0,"maxSegments":10000,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})

		t.Run("Update Segment Limit", func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?segments=500", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(
				ctx,
				t,
				linkLimit,
				`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":0,"maxSegments":500,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
				planet.Satellites[0].Config.Console.AuthToken,
			)
		})

		t.Run("Update Op-Specific Rates", func(t *testing.T) {
			t.Run("set not zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rateHead=1&burstHead=2&rateGet=3&burstGet=4&ratePut=5&burstPut=6&rateList=7&burstList=8&rateDelete=9&burstDelete=10", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":0,"maxSegments":500,"rate":0,"burst":0,"rateHead":1,"burstHead":2,"rateGet":3,"burstGet":4,"ratePut":5,"burstPut":6,"rateList":7,"burstList":8,"rateDelete":9,"burstDelete":10}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set default", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rateHead=-1&burstHead=-2&rateGet=-3&burstGet=-4&ratePut=-5&burstPut=-6&rateList=-7&burstList=-8&rateDelete=-9&burstDelete=-10", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":0,"maxSegments":500,"rate":0,"burst":0,"rateHead":null,"burstHead":null,"rateGet":null,"burstGet":null,"ratePut":null,"burstPut":null,"rateList":null,"burstList":null,"rateDelete":null,"burstDelete":null}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})

			t.Run("set zero", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit+"?rateHead=0&burstHead=0&rateGet=0&burstGet=0&ratePut=0&burstPut=0&rateList=0&burstList=0&rateDelete=0&burstDelete=0", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

				response, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				require.Equal(t, http.StatusOK, response.StatusCode)
				require.NoError(t, response.Body.Close())

				assertGet(
					ctx,
					t,
					linkLimit,
					`{"usage":{"amount":"1.00 GB","bytes":1000000000},"bandwidth":{"amount":"1.00 MB","bytes":1000000},"maxBuckets":0,"maxSegments":500,"rate":0,"burst":0,"rateHead":0,"burstHead":0,"rateGet":0,"burstGet":0,"ratePut":0,"burstPut":0,"rateList":0,"burstList":0,"rateDelete":0,"burstDelete":0}`,
					planet.Satellites[0].Config.Console.AuthToken,
				)
			})
		})
		t.Run("Update via body", func(t *testing.T) {
			data := url.Values{
				"usage":       []string{"1TiB"},
				"bandwidth":   []string{"2000000000"},
				"buckets":     []string{"-1"},
				"segments":    []string{"999"},
				"rate":        []string{"1"},
				"burst":       []string{"2"},
				"rateHead":    []string{"3"},
				"burstHead":   []string{"4"},
				"rateGet":     []string{"5"},
				"burstGet":    []string{"6"},
				"ratePut":     []string{"7"},
				"burstPut":    []string{"8"},
				"rateList":    []string{"9"},
				"burstList":   []string{"10"},
				"rateDelete":  []string{"11"},
				"burstDelete": []string{"12"},
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodPut, linkLimit, strings.NewReader(data.Encode()))
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			assertGet(
				ctx,
				t,
				linkLimit,
				`{"usage":{"amount":"1.0 TiB","bytes":1099511627776},"bandwidth":{"amount":"2.00 GB","bytes":2000000000},"maxBuckets":null,"maxSegments":999,"rate":1,"burst":2,"rateHead":3,"burstHead":4,"rateGet":5,"burstGet":6,"ratePut":7,"burstPut":8,"rateList":9,"burstList":10,"rateDelete":11,"burstDelete":12}`,
				planet.Satellites[0].Config.Console.AuthToken,
			)
		})
	})
}

func TestProjectAdd(t *testing.T) {
	t.Run("Basic project creation", func(t *testing.T) {
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
			userID := planet.Uplinks[0].Projects[0].Owner

			body := strings.NewReader(fmt.Sprintf(`{"ownerId":"%s","projectName":"Test Project"}`, userID.ID.String()))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/projects", body)
			require.NoError(t, err)
			req.Header.Set("Authorization", sat.Config.Console.AuthToken)

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

			project, err := sat.DB.Console().Projects().Get(ctx, output.ProjectID)
			require.NoError(t, err)
			require.Equal(t, "Test Project", project.Name)
			require.Equal(t, userID.ID, project.OwnerID)

			page, err := sat.DB.Console().ProjectMembers().GetPagedWithInvitationsByProjectID(ctx, output.ProjectID, console.ProjectMembersCursor{Limit: 10, Page: 1})
			require.NoError(t, err)
			require.Len(t, page.ProjectMembers, 1)
			require.Equal(t, userID.ID, page.ProjectMembers[0].MemberID)
			require.Equal(t, output.ProjectID, page.ProjectMembers[0].ProjectID)
			require.Equal(t, console.RoleAdmin, page.ProjectMembers[0].Role)
		})
	})

	t.Run("Project creation with entitlements enabled", func(t *testing.T) {
		var allowedPlacements = []storj.PlacementConstraint{storj.DefaultPlacement, storj.PlacementConstraint(10)}
		testplanet.Run(t, testplanet.Config{
			SatelliteCount:   1,
			StorageNodeCount: 0,
			UplinkCount:      1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
					config.Admin.Address = "127.0.0.1:0"
					config.Entitlements.Enabled = true
					config.Console.Placement.AllowedPlacementIdsForNewProjects = allowedPlacements
				},
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			address := sat.Admin.Admin.Listener.Addr()
			userID := planet.Uplinks[0].Projects[0].Owner

			body := strings.NewReader(fmt.Sprintf(`{"ownerId":"%s","projectName":"Test Project With Entitlements"}`, userID.ID.String()))
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address.String()+"/api/projects", body)
			require.NoError(t, err)
			req.Header.Set("Authorization", sat.Config.Console.AuthToken)

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

			project, err := sat.DB.Console().Projects().Get(ctx, output.ProjectID)
			require.NoError(t, err)
			require.Equal(t, "Test Project With Entitlements", project.Name)
			require.Equal(t, userID.ID, project.OwnerID)
			require.Contains(t, sat.Config.Console.Placement.AllowedPlacementIdsForNewProjects, project.DefaultPlacement)

			page, err := sat.DB.Console().ProjectMembers().GetPagedWithInvitationsByProjectID(ctx, output.ProjectID, console.ProjectMembersCursor{Limit: 10, Page: 1})
			require.NoError(t, err)
			require.Len(t, page.ProjectMembers, 1)
			require.Equal(t, userID.ID, page.ProjectMembers[0].MemberID)
			require.Equal(t, console.RoleAdmin, page.ProjectMembers[0].Role)

			features, err := sat.API.Entitlements.Service.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			require.NotNil(t, features)
			require.Equal(t, allowedPlacements, features.NewBucketPlacements)
		})
	})
}

func TestUpdateProjectsUserAgent(t *testing.T) {
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
		db := planet.Satellites[0].DB
		address := planet.Satellites[0].Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]
		newUserAgent := "awesome user agent value"

		t.Run("OK", func(t *testing.T) {
			bucketName := "testName"
			bucketName1 := "testName1"

			bucketID, err := uuid.New()
			require.NoError(t, err)
			bucketID1, err := uuid.New()
			require.NoError(t, err)

			_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        bucketID,
				Name:      bucketName,
				ProjectID: project.ID,
				UserAgent: nil,
			})
			require.NoError(t, err)

			_, err = db.Buckets().CreateBucket(ctx, buckets.Bucket{
				ID:        bucketID1,
				Name:      bucketName1,
				ProjectID: project.ID,
				UserAgent: nil,
			})
			require.NoError(t, err)

			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project.ID,
				BucketName: []byte(bucketName),
				UserAgent:  nil,
			})
			require.NoError(t, err)

			_, err = db.Attribution().Insert(ctx, &attribution.Info{
				ProjectID:  project.ID,
				BucketName: []byte(bucketName1),
				UserAgent:  nil,
			})
			require.NoError(t, err)

			body := strings.NewReader(fmt.Sprintf(`{"userAgent":"%s"}`, newUserAgent))
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPatch,
				fmt.Sprintf("http://"+address.String()+"/api/projects/%s/useragent", project.ID.String()),
				body,
			)
			require.NoError(t, err)
			req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			newUserAgentBytes := []byte(newUserAgent)

			updatedProject, err := db.Console().Projects().Get(ctx, project.ID)
			require.NoError(t, err)
			require.Equal(t, newUserAgentBytes, updatedProject.UserAgent)

			projectBuckets, err := planet.Satellites[0].API.Buckets.Service.ListBuckets(
				ctx,
				updatedProject.ID,
				buckets.ListOptions{Direction: buckets.DirectionForward},
				macaroon.AllowedBuckets{All: true},
			)
			require.NoError(t, err)
			require.Equal(t, 2, len(projectBuckets.Items))

			for _, bucket := range projectBuckets.Items {
				require.Equal(t, newUserAgentBytes, bucket.UserAgent)
			}

			bucketAttribution, err := db.Attribution().Get(ctx, project.ID, []byte(bucketName))
			require.Equal(t, newUserAgentBytes, bucketAttribution.UserAgent)
			require.NoError(t, err)

			bucketAttribution1, err := db.Attribution().Get(ctx, project.ID, []byte(bucketName1))
			require.Equal(t, newUserAgentBytes, bucketAttribution1.UserAgent)
			require.NoError(t, err)
		})

		t.Run("Same UserAgent", func(t *testing.T) {
			newProject, err := db.Console().Projects().Insert(ctx, &console.Project{
				Name:      "test",
				UserAgent: []byte(newUserAgent),
			})
			require.NoError(t, err)

			link := fmt.Sprintf("http://"+address.String()+"/api/projects/%s/useragent", newProject.ID)
			body := fmt.Sprintf(`{"userAgent":"%s"}`, newUserAgent)
			responseBody := assertReq(
				ctx,
				t,
				link,
				http.MethodPatch,
				body,
				http.StatusBadRequest,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(responseBody), "new UserAgent is equal to existing projects UserAgent")
		})
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
			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodPut,
				fmt.Sprintf("http://"+address.String()+"/api/projects/%s", project.ID.String()),
				body,
			)
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
			body := assertReq(
				ctx,
				t,
				link,
				http.MethodPut,
				putBody,
				http.StatusNotFound,
				"",
				planet.Satellites[0].Config.Console.AuthToken,
			)
			require.Contains(t, string(body), "does not exist")
		})
	})
}

func TestProjectUpdateComputeAccessToken(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 0,
		UplinkCount:      1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Entitlements.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		entService := sat.Admin.Entitlements.Service
		address := sat.Admin.Admin.Listener.Addr()
		project := planet.Uplinks[0].Projects[0]
		newAccessToken := "awesome token value"

		t.Run("OK", func(t *testing.T) {
			url := fmt.Sprintf("http://"+address.String()+"/api/projects/%s/compute-access-token", project.ID)
			body := strings.NewReader(fmt.Sprintf(`{"accessToken":"%s"}`, newAccessToken))
			req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
			require.NoError(t, err)
			req.Header.Set("Authorization", sat.Config.Console.AuthToken)

			response, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			feats, err := entService.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			require.NotNil(t, feats.ComputeAccessToken)
			require.EqualValues(t, []byte(newAccessToken), feats.ComputeAccessToken)

			// update to nil.
			body = strings.NewReader(`{"accessToken":null}`)
			req, err = http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
			require.NoError(t, err)
			req.Header.Set("Authorization", sat.Config.Console.AuthToken)

			response, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())

			feats, err = entService.Projects().GetByPublicID(ctx, project.PublicID)
			require.NoError(t, err)
			require.Nil(t, feats.ComputeAccessToken)
		})

		t.Run("Same AccessToken", func(t *testing.T) {
			err := entService.Projects().SetComputeAccessTokenByPublicID(ctx, project.PublicID, []byte(newAccessToken))
			require.NoError(t, err)

			link := fmt.Sprintf("http://"+address.String()+"/api/projects/%s/compute-access-token", project.ID)
			body := fmt.Sprintf(`{"accessToken":"%s"}`, newAccessToken)
			responseBody := assertReq(ctx, t, link, http.MethodPatch, body, http.StatusBadRequest, "", planet.Satellites[0].Config.Console.AuthToken)
			require.Contains(t, string(responseBody), "new token is equal to existing ComputeAccessToken")
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
		buckets, err := planet.Satellites[0].API.Buckets.Service.ListBuckets(
			ctx,
			projectID,
			buckets.ListOptions{Limit: 1, Direction: buckets.DirectionForward},
			macaroon.AllowedBuckets{All: true},
		)
		require.NoError(t, err)
		require.Len(t, buckets.Items, 0)

		apikeys, err := planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		}, "")
		require.NoError(t, err)
		require.Len(t, apikeys.APIKeys, 1)

		// the deletion with an existing API key should be successful
		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "", response.Header.Get("Content-Type"))

		// Ensure API keys are deleted.
		apikeys, err = planet.Satellites[0].DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:  1,
			Limit: 1,
		}, "")
		require.NoError(t, err)
		require.Len(t, apikeys.APIKeys, 0)

		project, err := planet.Satellites[0].DB.Console().Projects().Get(ctx, projectID)
		require.NoError(t, err)
		require.Equal(t, console.ProjectDisabled, *project.Status)
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
		}, "")
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID),
			nil,
		)
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
		}, "")
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
		kind, err := planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, kind)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID),
			nil,
		)
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
		err = planet.Satellites[0].DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3, &now)
		require.NoError(t, err)

		// Ensure User is paid tier.
		kind, err = planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, kind)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", projectID),
			nil,
		)
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
		project := planet.Uplinks[0].Projects[0]
		db := planet.Satellites[0].DB

		kind := console.PaidUser
		err := db.Console().Users().Update(ctx, project.Owner.ID, console.UpdateUserRequest{Kind: &kind})
		require.NoError(t, err)

		apiKeys, err := db.Console().APIKeys().GetPagedByProjectID(ctx, project.ID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		}, "")
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = db.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		now := time.Date(2030, time.Month(9), 1, 0, 0, 0, 0, time.UTC)

		oneMonthAhead := now.AddDate(0, 1, 0)
		planet.Satellites[0].Admin.Admin.Server.SetNow(func() time.Time {
			return oneMonthAhead
		})

		// use fixed intervals to avoid issues at the beginning of the month
		tally := accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         project.ID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = db.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)
		tally = accounting.BucketStorageTally{
			BucketName:        "test",
			ProjectID:         project.ID,
			IntervalStart:     time.Date(now.Year(), now.Month(), 1, 0, 1, 0, 1, time.UTC),
			ObjectCount:       1,
			TotalSegmentCount: 2,
			TotalBytes:        640000,
			MetadataSize:      2,
		}
		err = db.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		planet.Satellites[0].API.Payments.StripeService.SetNow(func() time.Time {
			return oneMonthAhead
		})
		err = planet.Satellites[0].API.Payments.StripeService.PrepareInvoiceProjectRecords(ctx, now)
		require.NoError(t, err)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s/usage", project.ID),
			nil,
		)
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
		}, "")
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
		err = planet.Satellites[0].DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3, &now)
		require.NoError(t, err)

		// Ensure User is paid tier.
		kind, err := planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, kind)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
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
		err = planet.Satellites[0].DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, false, memory.TB, memory.TB, 100000, 3, nil)
		require.NoError(t, err)

		// Ensure User is free tier.
		kind, err = planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, kind)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
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
		}, "")
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = planet.Satellites[0].DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
		require.NoError(t, err)

		// TODO: Improve updating of DB entries
		now := time.Now().UTC()
		planet.Satellites[0].Admin.Admin.Server.SetNow(func() time.Time {
			return now
		})

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
		err = planet.Satellites[0].DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3, &now)
		require.NoError(t, err)

		// Ensure User is paid tier.
		kind, err := planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, kind)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", planet.Satellites[0].Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(
			t,
			"{\"error\":\"usage for last month exist, but is not billed yet\",\"detail\":\"\"}",
			string(responseBody),
		)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Make User free tier again.
		err = planet.Satellites[0].DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, false, memory.TB, memory.TB, 100000, 3, nil)
		require.NoError(t, err)

		// Ensure User is free tier.
		kind, err = planet.Satellites[0].DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.FreeUser, kind)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
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
		sat := planet.Satellites[0]

		// Pause all accounting loops to avoid unnecessary inserts to the tally/rollup tables
		// which would conflict with the test.
		sat.Accounting.Tally.Loop.Pause()
		sat.Accounting.Rollup.Loop.Pause()
		sat.Accounting.RollupArchive.Loop.Pause()
		sat.Accounting.ProjectBWCleanup.Loop.Pause()

		address := sat.Admin.Admin.Listener.Addr()
		projectID := planet.Uplinks[0].Projects[0].ID

		apiKeys, err := sat.DB.Console().APIKeys().GetPagedByProjectID(ctx, projectID, console.APIKeyCursor{
			Page:   1,
			Limit:  2,
			Search: "",
		}, "")
		require.NoError(t, err)
		require.Len(t, apiKeys.APIKeys, 1)

		err = sat.DB.Console().APIKeys().Delete(ctx, apiKeys.APIKeys[0].ID)
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
		err = sat.DB.ProjectAccounting().CreateStorageTally(ctx, tally)
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
		err = sat.DB.ProjectAccounting().CreateStorageTally(ctx, tally)
		require.NoError(t, err)

		// Make User paid tier.
		err = sat.DB.Console().
			Users().
			UpdatePaidTier(ctx, planet.Uplinks[0].Projects[0].Owner.ID, true, memory.PB, memory.PB, 1000000, 3, &now)
		require.NoError(t, err)

		// Ensure User is paid tier.
		kind, err := sat.DB.Console().Users().GetUserKind(ctx, planet.Uplinks[0].Projects[0].Owner.ID)
		require.NoError(t, err)
		require.Equal(t, console.PaidUser, kind)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

		response, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(
			t,
			"{\"error\":\"usage for last month exist, but is not billed yet\",\"detail\":\"\"}",
			string(responseBody),
		)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Create Invoice Record for last month.
		firstOfMonth := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
		err = sat.DB.StripeCoinPayments().ProjectRecords().Create(ctx,
			[]stripe.CreateProjectRecord{{
				ProjectID: projectID,
				Storage:   100,
				Egress:    100,
				Segments:  100,
			}}, firstOfMonth, firstOfMonth.AddDate(0, 1, 0))
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

		// Project should fail to delete since the project record has not been used/billed yet.
		response, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		responseBody, err = io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, "{\"error\":\"unapplied project invoice record exist\",\"detail\":\"\"}", string(responseBody))
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusConflict, response.StatusCode)

		// Retrieve its ID and consume(mark as billed) it.
		dbRecord, err := sat.DB.StripeCoinPayments().
			ProjectRecords().
			Get(ctx, projectID, firstOfMonth, firstOfMonth.AddDate(0, 1, 0))
		require.NoError(t, err)
		require.Equal(t, projectID, dbRecord.ProjectID)
		err = sat.DB.StripeCoinPayments().ProjectRecords().Consume(ctx, dbRecord.ID)
		require.NoError(t, err)

		req, err = http.NewRequestWithContext(
			ctx,
			http.MethodDelete,
			fmt.Sprintf("http://"+address.String()+"/api/projects/%s", projectID),
			nil,
		)
		require.NoError(t, err)
		req.Header.Set("Authorization", sat.Config.Console.AuthToken)

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
