// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func Test_AllBucketNames(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = true
				config.Console.RateLimit.Burst = 10
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		project := planet.Uplinks[0].Projects[0]
		service := sat.API.Console.Service

		bucket1 := storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket1",
			ProjectID: project.ID,
		}

		bucket2 := storj.Bucket{
			ID:        testrand.UUID(),
			Name:      "testBucket2",
			ProjectID: project.ID,
		}

		_, err := sat.DB.Buckets().CreateBucket(ctx, bucket1)
		require.NoError(t, err)

		_, err = sat.DB.Buckets().CreateBucket(ctx, bucket2)
		require.NoError(t, err)

		user := console.CreateUser{
			FullName:  "Jack",
			ShortName: "",
			Email:     "bucketest@test.test",
			Password:  "123a123",
		}
		refUserID := ""

		regToken, err := service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		createdUser, err := service.CreateUser(ctx, user, regToken.Secret, refUserID)
		require.NoError(t, err)

		activationToken, err := service.GenerateActivationToken(ctx, createdUser.ID, createdUser.Email)
		require.NoError(t, err)

		err = service.ActivateAccount(ctx, activationToken)
		require.NoError(t, err)

		token, err := service.Token(ctx, user.Email, user.Password)
		require.NoError(t, err)

		client := http.Client{}

		req, err := http.NewRequest("GET", "http://"+planet.Satellites[0].API.Console.Listener.Addr().String()+"/api/v0/buckets/bucket-names?projectID="+project.ID.String(), nil)
		require.NoError(t, err)

		expire := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{
			Name:    "_tokenKey",
			Path:    "/",
			Value:   token,
			Expires: expire,
		}

		req.AddCookie(&cookie)

		result, err := client.Do(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, result.StatusCode)

		body, err := ioutil.ReadAll(result.Body)
		require.NoError(t, err)

		var output []string

		err = json.Unmarshal(body, &output)
		require.NoError(t, err)

		require.Equal(t, bucket1.Name, output[0])
		require.Equal(t, bucket2.Name, output[1])

		defer func() {
			err = result.Body.Close()
			require.NoError(t, err)
		}()
	})
}
