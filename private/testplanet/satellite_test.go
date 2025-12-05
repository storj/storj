// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestSatellite_AddProject(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.OpenRegistrationEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		user, err := planet.Satellites[0].AddUser(ctx, console.CreateUser{
			FullName: "test user",
			Email:    "test-email@test",
			Password: "password",
		}, 4)
		require.NoError(t, err)

		limit, err := planet.Satellites[0].DB.Console().Users().GetProjectLimit(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, 4, limit)

		for i := 0; i < 4; i++ {
			_, err = planet.Satellites[0].AddProject(ctx, user.ID, "test project "+strconv.Itoa(i))
			require.NoError(t, err)
		}

		projects, err := planet.Satellites[0].DB.Console().Projects().GetByUserID(ctx, user.ID)
		require.NoError(t, err)

		require.Equal(t, 4, len(projects))

		userCtx, err := planet.Satellites[0].UserContext(ctx, user.ID)
		require.NoError(t, err)

		// test if adding over limit will end with error
		_, err = planet.Satellites[0].API.Console.Service.CreateProject(userCtx, console.UpsertProjectInfo{
			Name: "test project over limit",
		})
		require.Error(t, err)
	})
}
