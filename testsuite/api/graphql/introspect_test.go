//  Copyright (C) 2021 Storj Labs, Inc.
//  See LICENSE for copying information.

package endpoints_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
)

func EndpointsTest(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		user, err := planet.Satellites[0].AddUser(ctx, console.CreateUser{
			FullName: "test user",
			Email:    "test-email@test",
			Password: "password",
		}, 4)
		require.NoError(t, err)

		_, err = planet.Satellites[0].DB.Console().Projects().GetByUserID(ctx, user.ID)
		require.NoError(t, err)
	})
}
