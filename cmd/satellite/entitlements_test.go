// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
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

			err = processUserEmail(ctx, user1.Email, args, true)
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

			err = processUserEmail(ctx, "invalid-email", args, true)
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid email format")
		})

		t.Run("NonexistentUser", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			err = processUserEmail(ctx, "nonexistent@example.com", args, true)
			require.Error(t, err)
			require.True(t, errors.Is(err, sql.ErrNoRows))
		})

		t.Run("NonactiveUser", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			err = processUserEmail(ctx, user3.Email, args, true)
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

			err = processUserEmail(ctx, user2.Email, args, true)
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

			err = processUserEmail(ctx, user4.Email, args, true)
			require.NoError(t, err)

			// Verify that user4's project got its custom default placement (10)
			features, err := entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
			require.NoError(t, err)
			require.EqualValues(t, []storj.PlacementConstraint{10}, features.NewBucketPlacements)
		})
	})
}

func TestSetNewBucketPlacements_AllUsers(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
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

		// Create an inactive user (should be skipped).
		inactiveUser, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Inactive User",
			Email:    "inactive@example.com",
		}, 1)
		require.NoError(t, err)
		inactiveStatus := console.PendingDeletion
		require.NoError(t, sat.DB.Console().Users().Update(ctx, inactiveUser.ID, console.UpdateUserRequest{
			Status: &inactiveStatus,
		}))

		user1Project, err := sat.AddProject(ctx, user1.ID, "user1project")
		require.NoError(t, err)
		user2Project, err := sat.AddProject(ctx, user2.ID, "user2project")
		require.NoError(t, err)
		user2Project2, err := sat.AddProject(ctx, user2.ID, "user2project2")
		require.NoError(t, err)
		user3Project, err := sat.AddProject(ctx, user3.ID, "user3project")
		require.NoError(t, err)

		activeProjects := []uuid.UUID{user1Project.PublicID, user2Project.PublicID, user2Project2.PublicID, user3Project.PublicID}

		// Create project for inactive user (should be skipped).
		inactiveUserProject, err := sat.AddProject(ctx, inactiveUser.ID, "inactiveproject")
		require.NoError(t, err)

		// Set custom default placement for user3's project.
		require.NoError(t, sat.DB.Console().Projects().UpdateDefaultPlacement(ctx, user3Project.ID, storj.PlacementConstraint(10)))

		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())
		args := processingArgs{
			log:         zaptest.NewLogger(t),
			satDB:       sat.DB,
			entService:  entService,
			skipConfirm: true,
		}

		t.Run("ProcessAllActiveUsers", func(t *testing.T) {
			args.newPlacements = []storj.PlacementConstraint{5, 12}

			err = processAllUsers(ctx, args)
			require.NoError(t, err)

			// Verify that entitlements were set for all active users' projects.
			for _, publicID := range activeProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{5, 12}, features.NewBucketPlacements)
			}

			// Verify that inactive user's project was not processed.
			_, err = entService.Projects().GetByPublicID(ctx, inactiveUserProject.PublicID)
			require.True(t, entitlements.ErrNotFound.Has(err))
		})

		t.Run("ProcessAllUsersWithDefaultPlacementLogic", func(t *testing.T) {
			// Reset entitlements first.
			for _, publicID := range activeProjects {
				err = entService.Projects().SetNewBucketPlacementsByPublicID(ctx, publicID, []storj.PlacementConstraint{storj.DefaultPlacement})
				require.NoError(t, err)
			}

			args.newPlacements = nil // Use default logic
			args.allowedPlacements = []storj.PlacementConstraint{3, 4}

			err = processAllUsers(ctx, args)
			require.NoError(t, err)

			// Verify that projects with default placement got allowedPlacements.
			defaultPlacementProjects := []uuid.UUID{user1Project.PublicID, user2Project.PublicID, user2Project2.PublicID}
			for _, publicID := range defaultPlacementProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{3, 4}, features.NewBucketPlacements)
			}

			// Verify that user3's project got its custom default placement (10).
			features, err := entService.Projects().GetByPublicID(ctx, user3Project.PublicID)
			require.NoError(t, err)
			require.EqualValues(t, []storj.PlacementConstraint{10}, features.NewBucketPlacements)
		})

		t.Run("NewPlacementsTakesPrecedenceOverAllowedPlacements", func(t *testing.T) {
			// Reset entitlements first.
			for _, publicID := range activeProjects {
				err = entService.Projects().SetNewBucketPlacementsByPublicID(ctx, publicID, []storj.PlacementConstraint{storj.DefaultPlacement})
				require.NoError(t, err)
			}

			args.newPlacements = []storj.PlacementConstraint{7, 8, 9}
			args.allowedPlacements = []storj.PlacementConstraint{1, 2, 3} // These should be ignored

			err = processAllUsers(ctx, args)
			require.NoError(t, err)

			// Verify that all projects got newPlacements (not allowedPlacements).
			for _, publicID := range activeProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{7, 8, 9}, features.NewBucketPlacements)
			}
		})
	})
}

func TestSetNewBucketPlacements_CSV(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
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

		// Make user3 inactive.
		inactiveStatus := console.PendingDeletion
		require.NoError(t, sat.DB.Console().Users().Update(ctx, user3.ID, console.UpdateUserRequest{
			Status: &inactiveStatus,
		}))

		user1Project, err := sat.AddProject(ctx, user1.ID, "user1project")
		require.NoError(t, err)
		user2Project, err := sat.AddProject(ctx, user2.ID, "user2project")
		require.NoError(t, err)

		activeProjects := []uuid.UUID{user1Project.PublicID, user2Project.PublicID}

		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())
		args := processingArgs{
			log:         zaptest.NewLogger(t),
			satDB:       sat.DB,
			entService:  entService,
			skipConfirm: true,
		}

		// Helper function to create temporary CSV file.
		createTempCSV := func(content string) string {
			tmpFile, err := os.CreateTemp(t.TempDir(), "test_*.csv")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, tmpFile.Close())
			}()

			_, err = tmpFile.WriteString(content)
			require.NoError(t, err)

			return tmpFile.Name()
		}

		t.Run("ValidCSVWithHeader", func(t *testing.T) {
			csvContent := "email\nuser1@example.com\nuser2@example.com"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{5, 12}

			err = processCSVFile(ctx, csvPath, args)
			require.NoError(t, err)

			// Verify entitlements were set.
			for _, publicID := range activeProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{5, 12}, features.NewBucketPlacements)
			}
		})

		t.Run("ValidCSVWithoutHeader", func(t *testing.T) {
			// Reset entitlements first.
			for _, publicID := range activeProjects {
				err = entService.Projects().SetNewBucketPlacementsByPublicID(ctx, publicID, []storj.PlacementConstraint{storj.DefaultPlacement})
				require.NoError(t, err)
			}

			csvContent := "user1@example.com\nuser2@example.com"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{3, 4}

			err = processCSVFile(ctx, csvPath, args)
			require.NoError(t, err)

			// Verify entitlements were set.
			for _, publicID := range activeProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{3, 4}, features.NewBucketPlacements)
			}
		})

		t.Run("CSVWithValidEmailsOnly", func(t *testing.T) {
			// Reset entitlements first.
			for _, publicID := range activeProjects {
				err = entService.Projects().SetNewBucketPlacementsByPublicID(ctx, publicID, []storj.PlacementConstraint{storj.DefaultPlacement})
				require.NoError(t, err)
			}

			csvContent := "email\nuser1@example.com\n\nuser2@example.com"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{7, 8}

			err = processCSVFile(ctx, csvPath, args)
			require.NoError(t, err)

			// Verify entitlements were set.
			for _, publicID := range activeProjects {
				features, err := entService.Projects().GetByPublicID(ctx, publicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{7, 8}, features.NewBucketPlacements)
			}
		})

		t.Run("CSVWithInvalidEmails", func(t *testing.T) {
			csvContent := "email\nuser1@example.com\ninvalid-email\n\nuser2@example.com\nanother-invalid"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{9, 10}

			// Should return error because of invalid email addresses.
			err = processCSVFile(ctx, csvPath, args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "CSV file contains invalid email addresses")
		})

		t.Run("CSVWithNonexistentUser", func(t *testing.T) {
			csvContent := "user1@example.com\nnonexistent@example.com\nuser2@example.com"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{11, 12}

			// Should return error because of nonexistent user.
			err = processCSVFile(ctx, csvPath, args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "errors occurred while processing CSV users")
		})

		t.Run("CSVWithInactiveUser", func(t *testing.T) {
			csvContent := "user1@example.com\nuser3@example.com\nuser2@example.com"
			csvPath := createTempCSV(csvContent)
			defer func() {
				require.NoError(t, os.Remove(csvPath))
			}()

			args.newPlacements = []storj.PlacementConstraint{13, 14}

			// Should return error because user3 is inactive.
			err = processCSVFile(ctx, csvPath, args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "errors occurred while processing CSV users")
		})
	})

	t.Run("CSVFileErrors", func(t *testing.T) {
		args := processingArgs{
			log:         zaptest.NewLogger(t),
			skipConfirm: true,
		}

		t.Run("NonexistentFile", func(t *testing.T) {
			err := processCSVFile(context.TODO(), "nonexistent.csv", args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error opening CSV file")
		})

		t.Run("EmptyFile", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "empty_*.csv")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, errs.Combine(os.Remove(tmpFile.Name()), tmpFile.Close()))
			}()

			err = processCSVFile(context.TODO(), tmpFile.Name(), args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "CSV file is empty")
		})

		t.Run("MalformedCSV", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "malformed_*.csv")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, errs.Combine(os.Remove(tmpFile.Name()), tmpFile.Close()))
			}()

			// Create a CSV with unclosed quotes.
			_, err = tmpFile.WriteString("\"unclosed quote\nuser@example.com")
			require.NoError(t, err)

			err = processCSVFile(context.TODO(), tmpFile.Name(), args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error reading CSV file")
		})
	})
}

func TestSetNewBucketPlacements_Validation(t *testing.T) {
	t.Run("BothEmailAndCSVFlags", func(t *testing.T) {
		setNewBucketPlacementsEmail = "test@example.com"
		setNewBucketPlacementsCSV = "test.csv"
		setNewBucketPlacementsJSON = ""

		err := cmdSetNewBucketPlacements(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot specify both --email and --csv flags")
	})

	t.Run("InvalidJSONPlacements", func(t *testing.T) {
		setNewBucketPlacementsEmail = "test@example.com"
		setNewBucketPlacementsCSV = ""
		setNewBucketPlacementsJSON = "invalid-json"

		err := cmdSetNewBucketPlacements(nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid JSON format for placements")
	})
}
