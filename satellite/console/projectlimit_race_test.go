// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestCreateProjectLimitConcurrentRequests(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// OpenRegistrationEnabled must be off so that the registration
				// token created by AddUser applies the requested project limit.
				config.Console.OpenRegistrationEnabled = false
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service

		// An account limited to a single project must never end up with more
		// than one, even when several creation requests arrive at the same
		// time. See https://github.com/storj/storj/issues/7814.
		const concurrentRequests = 8
		for attempt := 0; attempt < 3; attempt++ {
			user, err := sat.AddUser(ctx, console.CreateUser{
				FullName: "Concurrent Projects",
				Email:    fmt.Sprintf("concurrent-%d@mail.test", attempt),
			}, 1)
			require.NoError(t, err)

			userCtx, err := sat.UserContext(ctx, user.ID)
			require.NoError(t, err)

			start := make(chan struct{})
			var (
				wg      sync.WaitGroup
				mu      sync.Mutex
				created int
				failed  int
			)
			for i := 0; i < concurrentRequests; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					_, err := service.CreateProject(userCtx, console.UpsertProjectInfo{
						Name: fmt.Sprintf("race-%d-%d", attempt, i),
					})
					mu.Lock()
					defer mu.Unlock()
					if err == nil {
						created++
					} else {
						failed++
					}
				}(i)
			}
			close(start)
			wg.Wait()

			require.Equal(t, 1, created, "expected exactly one project creation to succeed")
			require.Equal(t, concurrentRequests-1, failed)

			projects, err := service.GetUsersProjects(userCtx)
			require.NoError(t, err)
			require.Len(t, projects, 1, "project limit bypassed: user owns more projects than allowed")
		}
	})
}
