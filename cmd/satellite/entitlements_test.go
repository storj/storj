// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
)

func TestSetNewBucketPlacements_UserEmail(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		user1, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User 1",
			Email:    "user1@example.com",
		}, 1)
		require.NoError(t, err)
		user2, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User 2",
			Email:    "user2@example.com",
		}, 1)
		require.NoError(t, err)
		user3, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User 3",
			Email:    "user3@example.com",
		}, 1)
		require.NoError(t, err)
		user4, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User 4",
			Email:    "user4@example.com",
		}, 1)
		require.NoError(t, err)

		user1Project, err := sat.AddProject(ctx, user1.ID, "user1project")
		require.NoError(t, err)
		user2Project, err := sat.AddProject(ctx, user2.ID, "user2project")
		require.NoError(t, err)
		proj2, err := sat.AddProject(ctx, user2.ID, "user2 second project")
		require.NoError(t, err)
		proj3, err := sat.AddProject(ctx, user2.ID, "user2 third project")
		require.NoError(t, err)

		// Make user3 non-active (should be skipped).
		inactiveStatus := console.PendingDeletion
		require.NoError(t, sat.DB.Console().Users().Update(ctx, user3.ID, console.UpdateUserRequest{
			Status: &inactiveStatus,
		}))

		// Create a project for user4 with custom default placement.
		user4Project, err := sat.AddProject(ctx, user4.ID, "user4project")
		require.NoError(t, err)
		require.NoError(t, sat.DB.Console().Projects().UpdateDefaultPlacement(ctx, user4Project.ID, storj.PlacementConstraint(10)))

		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())

		t.Run("ValidUser", func(t *testing.T) {
			args := processingArgs{
				log:               zaptest.NewLogger(t),
				satDB:             sat.DB,
				entService:        entService,
				newPlacements:     []storj.PlacementConstraint{5, 12},
				allowedPlacements: nil,
			}

			err = processUserEmail(ctx, user1.Email, args)
			require.NoError(t, err)

			// Verify that entitlements were set for user1's project
			features, err := entService.Projects().GetByPublicID(ctx, user1Project.PublicID)
			require.NoError(t, err)
			require.EqualValues(t, []storj.PlacementConstraint{5, 12}, features.NewBucketPlacements)
		})

		t.Run("InvalidEmail", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			err = processUserEmail(ctx, "invalid-email", args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid email format")
		})

		t.Run("NonexistentUser", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			err = processUserEmail(ctx, "nonexistent@example.com", args)
			require.Error(t, err)
			require.True(t, errors.Is(err, sql.ErrNoRows))
		})

		t.Run("NonactiveUser", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			err = processUserEmail(ctx, user3.Email, args)
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), fmt.Sprintf("user with email %s is not active", user3.Email)))
		})

		t.Run("MultipleProjects", func(t *testing.T) {
			args := processingArgs{
				log:               zaptest.NewLogger(t),
				satDB:             sat.DB,
				entService:        entService,
				newPlacements:     []storj.PlacementConstraint{1, 2},
				allowedPlacements: nil,
			}

			err = processUserEmail(ctx, user2.Email, args)
			require.NoError(t, err)

			// Verify entitlements were set for all of user2's projects
			projectPublicIDs := []uuid.UUID{user2Project.PublicID, proj2.PublicID, proj3.PublicID}
			for _, publicID := range projectPublicIDs {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{1, 2}, features.NewBucketPlacements)
			}
		})

		t.Run("DefaultPlacementLogic", func(t *testing.T) {
			args := processingArgs{
				log:               zaptest.NewLogger(t),
				satDB:             sat.DB,
				entService:        entService,
				newPlacements:     nil, // Use default logic
				allowedPlacements: []storj.PlacementConstraint{3, 4},
			}

			err = processUserEmail(ctx, user4.Email, args)
			require.NoError(t, err)

			// Verify that user4's project got its custom default placement (10)
			features, err := entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
			require.NoError(t, err)
			require.EqualValues(t, []storj.PlacementConstraint{10}, features.NewBucketPlacements)
		})
	})
}

func TestSetNewBucketPlacements_Validation(t *testing.T) {
	t.Run("BothEmailAndCSVFlags", func(t *testing.T) {
		setNewBucketPlacementsEmail = "test@example.com"
		setNewBucketPlacementsCSV = "test.csv"
		setNewBucketPlacementsJSON = ""
		setNewBucketPlacementsSkipConfirm = true

		err := cmdSetNewBucketPlacements(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot specify both --email and --csv flags")
	})

	t.Run("InvalidJSONPlacements", func(t *testing.T) {
		setNewBucketPlacementsEmail = "test@example.com"
		setNewBucketPlacementsCSV = ""
		setNewBucketPlacementsJSON = "invalid-json"
		setNewBucketPlacementsSkipConfirm = true

		err := cmdSetNewBucketPlacements(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid JSON format for placements")
	})

	t.Run("NonexistentCSVFile", func(t *testing.T) {
		args := processingArgs{
			log: zaptest.NewLogger(t),
		}

		err := processCSVFile(context.TODO(), "nonexistent.csv", args)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error opening CSV file")
	})
}
