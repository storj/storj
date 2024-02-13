// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package version_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/version"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/versioncontrol"
)

func TestCursorEmptyChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var mockAllowedVer version.SemVer
		mockAllowedVer.Patch = 2

		service := planet.StorageNodes[0].Version.Service
		require.NotNil(t, service)

		service.SetAcceptedVersion(mockAllowedVer)

		chore := planet.StorageNodes[0].Version.Chore
		require.NotNil(t, chore)

		chore.Loop.Pause()

		firstTimeStamp := chore.TestCheckVersion()
		require.Equal(t, false, firstTimeStamp.IsOutdated)
	})
}

func TestRolloutChore(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			VersionControl: func(config *versioncontrol.Config) {
				config.Binary.Storagenode.Rollout.Cursor = 100
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		var mockAllowedVer version.SemVer
		mockAllowedVer.Patch = 2
		service := planet.StorageNodes[0].Version.Service
		require.NotNil(t, service)

		service.SetAcceptedVersion(mockAllowedVer)

		chore := planet.StorageNodes[0].Version.Chore
		require.NotNil(t, chore)

		chore.Loop.Pause()
		chore.Loop.TriggerWait()

		firstTimeStamp := chore.TestCheckVersion()
		require.Equal(t, true, firstTimeStamp.IsOutdated)
		require.Equal(t, notifications.TimesNotifiedZero, firstTimeStamp.TimesNotified)

		firstPeriod := time.Now().UTC().Add(97 * time.Hour)
		chore.TestSetNow(firstPeriod.UTC)
		chore.Loop.TriggerWait()

		secondTimeStamp := chore.TestCheckVersion()
		require.Equal(t, notifications.TimesNotifiedFirst, secondTimeStamp.TimesNotified)

		secondPeriod := time.Now().UTC().Add(145 * time.Hour)
		chore.TestSetNow(secondPeriod.UTC)
		chore.Loop.TriggerWait()

		thirdTimeStamp := chore.TestCheckVersion()
		require.Equal(t, notifications.TimesNotifiedSecond, thirdTimeStamp.TimesNotified)

		thirdPeriod := time.Now().UTC().Add(336 * time.Hour)
		chore.TestSetNow(thirdPeriod.UTC)
		chore.Loop.TriggerWait()

		lastTimeStamp := chore.TestCheckVersion()
		require.Equal(t, notifications.TimesNotifiedLast, lastTimeStamp.TimesNotified)
	})
}
