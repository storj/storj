// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

func TestSetEntitlement_UserEmail(t *testing.T) {
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
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}
			t.Run("SetNewBucketPlacements", func(t *testing.T) {
				args.action = actionSetNewBucketPlacements
				args.newPlacements = []storj.PlacementConstraint{5, 12}

				err = processUserEmail(ctx, user1.Email, args, true)
				require.NoError(t, err)

				// Verify that entitlements were set for user1's project
				features, err := entService.Projects().GetByPublicID(ctx, user1Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{5, 12}, features.NewBucketPlacements)
			})

			t.Run("SetPlacementProductMap", func(t *testing.T) {
				args.action = actionSetPlacementProductMap
				args.placementProductMap = entitlements.PlacementProductMappings{
					5:  1,
					12: 1,
				}

				err = processUserEmail(ctx, user1.Email, args, true)
				require.NoError(t, err)

				// Verify that entitlements were set for user1's project
				features, err := entService.Projects().GetByPublicID(ctx, user1Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, args.placementProductMap, features.PlacementProductMappings)
			})
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
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			t.Run("SetNewBucketPlacements", func(t *testing.T) {
				args.action = actionSetNewBucketPlacements
				args.newPlacements = []storj.PlacementConstraint{1, 2}

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

			t.Run("SetPlacementProductMap", func(t *testing.T) {
				args.action = actionSetPlacementProductMap
				args.placementProductMap = entitlements.PlacementProductMappings{
					1: 1,
					2: 1,
				}

				err = processUserEmail(ctx, user2.Email, args, true)
				require.NoError(t, err)

				// Verify entitlements were set for all of user2's projects
				projectPublicIDs := []uuid.UUID{user2Project.PublicID, proj2.PublicID, proj3.PublicID}
				for _, publicID := range projectPublicIDs {
					features, err := entService.Projects().GetByPublicID(ctx, publicID)
					require.NoError(t, err)
					require.EqualValues(t, args.placementProductMap, features.PlacementProductMappings)
				}
			})
		})

		t.Run("DefaultsLogic", func(t *testing.T) {
			args := processingArgs{
				log:        zaptest.NewLogger(t),
				satDB:      sat.DB,
				entService: entService,
			}

			t.Run("SetNewBucketPlacements", func(t *testing.T) {
				args.action = actionSetNewBucketPlacements
				args.newPlacements = nil // Use default logic
				args.allowedPlacements = []storj.PlacementConstraint{3, 4}

				err = processUserEmail(ctx, user4.Email, args, true)
				require.NoError(t, err)

				// Verify that user4's project got its custom default placement (10)
				features, err := entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, []storj.PlacementConstraint{10}, features.NewBucketPlacements)
			})

			t.Run("SetPlacementProductMap", func(t *testing.T) {
				args.action = actionSetPlacementProductMap
				args.placementProductMap = nil // Use default logic
				args.defaultPlacementProductMap = payments.PlacementProductIdMap{
					1: 3, 2: 4,
				}

				err = processUserEmail(ctx, user4.Email, args, true)
				require.NoError(t, err)

				// Verify that user4's project got global placement mapping
				features, err := entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, entitlements.PlacementProductMappings{
					1: 3, 2: 4,
				}, features.PlacementProductMappings)

				// remove entitlements for user4's project
				err = entService.Projects().DeleteByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)

				// update user4's project to have "unknown-agent" user agent
				require.NoError(t, sat.DB.Console().Projects().UpdateUserAgent(ctx, user4Project.ID, []byte("unknown-agent")))

				err = processUserEmail(ctx, user4.Email, args, true)
				require.NoError(t, err)

				// verify that user4's project gets the same global placement mapping
				// regardless of UserAgent
				features, err = entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, entitlements.PlacementProductMappings{
					1: 3, 2: 4,
				}, features.PlacementProductMappings)

				err = entService.Projects().DeleteByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)
				require.NoError(t, sat.DB.Console().Projects().UpdateUserAgent(ctx, user4Project.ID, []byte("test-agent")))

				err = processUserEmail(ctx, user4.Email, args, true)
				require.NoError(t, err)

				// verify that user4's project gets the same global placement mapping
				// even with "test-agent" UserAgent
				features, err = entService.Projects().GetByPublicID(ctx, user4Project.PublicID)
				require.NoError(t, err)
				require.EqualValues(t, entitlements.PlacementProductMappings{
					1: 3, 2: 4,
				}, features.PlacementProductMappings)
			})
		})
	})
}

func TestSetEntitlement_AllUsers(t *testing.T) {
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
		err = sat.DB.Console().Projects().UpdateUserAgent(ctx, user1Project.ID, []byte("test-agent"))
		require.NoError(t, err)
		user2Project, err := sat.AddProject(ctx, user2.ID, "user2project")
		require.NoError(t, err)
		user2Project2, err := sat.AddProject(ctx, user2.ID, "user2project2")
		require.NoError(t, err)
		user3Project, err := sat.AddProject(ctx, user3.ID, "user3project")
		require.NoError(t, err)
		err = sat.DB.Console().Projects().UpdateUserAgent(ctx, user3Project.ID, []byte("test-agent"))
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
			t.Run("SetNewBucketPlacements", func(t *testing.T) {
				args.action = actionSetNewBucketPlacements
				args.newPlacements = []storj.PlacementConstraint{5, 12}

				err = processAllUsers(ctx, args)
				require.NoError(t, err)

				// Verify that entitlements were set for all active users' projects.
				for _, publicID := range activeProjects {
					features, err := entService.Projects().GetByPublicID(ctx, publicID)
					require.NoError(t, err)
					require.EqualValues(t, []storj.PlacementConstraint{5, 12}, features.NewBucketPlacements)
				}
			})

			t.Run("SetPlacementProductMap", func(t *testing.T) {
				args.action = actionSetPlacementProductMap
				args.placementProductMap = entitlements.PlacementProductMappings{
					5:  1,
					12: 1,
				}

				err = processAllUsers(ctx, args)
				require.NoError(t, err)

				// Verify that entitlements were set for all active users' projects.
				for _, publicID := range activeProjects {
					features, err := entService.Projects().GetByPublicID(ctx, publicID)
					require.NoError(t, err)
					require.EqualValues(t, args.placementProductMap, features.PlacementProductMappings)
				}
			})

			// Verify that inactive user's project was not processed.
			_, err = entService.Projects().GetByPublicID(ctx, inactiveUserProject.PublicID)
			require.True(t, entitlements.ErrNotFound.Has(err))
		})

		t.Run("ProcessAllUsersWithDefaultsLogic", func(t *testing.T) {
			// Reset entitlements first.
			for _, publicID := range activeProjects {
				err = entService.Projects().DeleteByPublicID(ctx, publicID)
				require.NoError(t, err)
			}

			t.Run("SetNewBucketPlacements", func(t *testing.T) {
				args.action = actionSetNewBucketPlacements
				args.newPlacements = nil // use default logic
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

			t.Run("SetPlacementProductMap", func(t *testing.T) {
				args.action = actionSetPlacementProductMap
				args.placementProductMap = nil // use default logic
				args.defaultPlacementProductMap = payments.PlacementProductIdMap{
					1: 3, 2: 4,
				}

				err = processAllUsers(ctx, args)
				require.NoError(t, err)

				// Verify that all projects get the same global placement mapping
				// regardless of UserAgent
				allProjects := []uuid.UUID{user1Project.PublicID, user2Project.PublicID, user2Project2.PublicID, user3Project.PublicID}
				for _, publicID := range allProjects {
					features, err := entService.Projects().GetByPublicID(ctx, publicID)
					require.NoError(t, err)
					require.EqualValues(t, entitlements.PlacementProductMappings{
						1: 3, 2: 4,
					}, features.PlacementProductMappings)
				}
			})
		})

		t.Run("NewPlacementsTakesPrecedenceOverAllowedPlacements", func(t *testing.T) {
			args.action = actionSetNewBucketPlacements
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

func TestSetEntitlement_CSV(t *testing.T) {
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
			err := processCSVFile(t.Context(), "nonexistent.csv", args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error opening CSV file")
		})

		t.Run("EmptyFile", func(t *testing.T) {
			tmpFile, err := os.CreateTemp(t.TempDir(), "empty_*.csv")
			require.NoError(t, err)
			defer func() {
				require.NoError(t, errs.Combine(os.Remove(tmpFile.Name()), tmpFile.Close()))
			}()

			err = processCSVFile(t.Context(), tmpFile.Name(), args)
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

			err = processCSVFile(t.Context(), tmpFile.Name(), args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "error reading CSV file")
		})
	})
}

func TestMigratePricing_ParserHelpers(t *testing.T) {
	t.Run("parsePlacementList", func(t *testing.T) {
		got, err := parsePlacementList("0,12,30")
		require.NoError(t, err)
		require.Equal(t, []storj.PlacementConstraint{0, 12, 30}, got)

		got, err = parsePlacementList("")
		require.NoError(t, err)
		require.Nil(t, got)

		_, err = parsePlacementList("0,abc")
		require.Error(t, err)
	})

	t.Run("parsePlacementPairMap", func(t *testing.T) {
		got, err := parsePlacementPairMap("30:0,31:12,32:0")
		require.NoError(t, err)
		require.Equal(t, map[storj.PlacementConstraint]storj.PlacementConstraint{
			30: 0, 31: 12, 32: 0,
		}, got)

		got, err = parsePlacementPairMap("")
		require.NoError(t, err)
		require.Empty(t, got)

		_, err = parsePlacementPairMap("30")
		require.Error(t, err)

		_, err = parsePlacementPairMap("abc:0")
		require.Error(t, err)
	})

	t.Run("parsePlacementProductMap", func(t *testing.T) {
		got, err := parsePlacementProductMap("0:20,12:21")
		require.NoError(t, err)
		require.Equal(t, entitlements.PlacementProductMappings{
			0: 20, 12: 21,
		}, got)

		got, err = parsePlacementProductMap("")
		require.NoError(t, err)
		require.Empty(t, got)

		_, err = parsePlacementProductMap("0")
		require.Error(t, err)

		_, err = parsePlacementProductMap("abc:20")
		require.Error(t, err)
	})
}

func TestMigratePricing_Validation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location","global")` +
						`;12:annotation("location","advanced")` +
						`;30:annotation("location","global-legacy")` +
						`;31:annotation("location","regional-legacy")` +
						`;32:annotation("location","archive-legacy")`,
				}
				price := paymentsconfig.ProjectUsagePrice{StorageTB: "4", EgressTB: "7", Segment: "0.0000088"}
				var productOverrides paymentsconfig.ProductPriceOverrides
				productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					20: {ProjectUsagePrice: price},
					21: {ProjectUsagePrice: price},
				})
				config.Payments.Products = productOverrides
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		runCfg.Placement = sat.Config.Placement
		runCfg.Payments = sat.Config.Payments

		t.Run("InvalidPhase", func(t *testing.T) {
			err := validateMigratePricingFlags("", "0,12", "0:20", "0,12", true)
			require.Error(t, err)
			require.Contains(t, err.Error(), "--phase must be 'ui' or 'billing'")

			err = validateMigratePricingFlags("wrong phase", "0,12", "0:20", "0,12", true)
			require.Error(t, err)
			require.Contains(t, err.Error(), "--phase must be 'ui' or 'billing'")
		})

		t.Run("KnownPlacementsRequired", func(t *testing.T) {
			err := validateMigratePricingFlags("ui", "0,12", "", "", false)
			require.Error(t, err)
			require.Contains(t, err.Error(), "--known-placement-ids is required")

			err = validateMigratePricingFlags("billing", "", "0:20", "", true)
			require.Error(t, err)
			require.Contains(t, err.Error(), "--known-placement-ids is required")
		})

		t.Run("PhaseUI", func(t *testing.T) {
			t.Run("TargetNBPRequired", func(t *testing.T) {
				err := validateMigratePricingFlags("ui", "", "", "0,12", false)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--target-new-bucket-placements is required for the ui phase")
			})

			t.Run("ValidMinimal", func(t *testing.T) {
				// ui phase does not require --new-placement-product-map or --fallback-product-id.
				err := validateMigratePricingFlags("ui", "0,12", "", "0,12", false)
				require.NoError(t, err)
			})

			t.Run("ValidWithAllOptionalFlags", func(t *testing.T) {
				err := validateMigratePricingFlags("ui", "0,12", "0:20,12:21", "0,12,30", true)
				require.NoError(t, err)
			})
		})

		t.Run("PhaseBilling", func(t *testing.T) {
			t.Run("NewPPMRequired", func(t *testing.T) {
				err := validateMigratePricingFlags("billing", "0,12", "", "0,12", true)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--new-placement-product-map is required for phase billing")
			})

			t.Run("FallbackProductRequired", func(t *testing.T) {
				err := validateMigratePricingFlags("billing", "0,12", "0:20", "0,12", false)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--fallback-product-id is required for phase billing")
			})

			t.Run("ValidMinimal", func(t *testing.T) {
				// billing phase does not require --target-new-bucket-placements.
				err := validateMigratePricingFlags("billing", "", "0:20", "0,12", true)
				require.NoError(t, err)
			})

			t.Run("ValidWithAllFlags", func(t *testing.T) {
				err := validateMigratePricingFlags("billing", "0,12", "0:20,12:21", "0,12,30", true)
				require.NoError(t, err)
			})
		})

		t.Run("EmptyNewPPMRejected", func(t *testing.T) {
			args := migratePricingArgs{
				newPlacementProductMappings: entitlements.PlacementProductMappings{},
				knownSet:                    placementSet([]storj.PlacementConstraint{0, 12}),
				phase:                       "billing",
			}
			err := runMigratePricing(t.Context(), zap.NewNop(), nil, nil, args)
			require.Error(t, err)
			require.Contains(t, err.Error(), "empty map")
		})

		t.Run("FlagValues", func(t *testing.T) {
			t.Run("AllValid", func(t *testing.T) {
				err := validateMigratePricingFlagValues(
					"billing",
					[]storj.PlacementConstraint{0, 12},
					map[storj.PlacementConstraint]storj.PlacementConstraint{30: 0, 31: 12, 32: 0},
					entitlements.PlacementProductMappings{0: 20, 12: 21},
					[]storj.PlacementConstraint{0, 12, 30, 31, 32},
					20,
				)
				require.NoError(t, err)
			})
			t.Run("UnknownTargetNBP", func(t *testing.T) {
				err := validateMigratePricingFlagValues("ui",
					[]storj.PlacementConstraint{0, 99}, nil, nil,
					[]storj.PlacementConstraint{0, 12}, 0)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--target-new-bucket-placements")
			})
			t.Run("UnknownKnownPlacement", func(t *testing.T) {
				err := validateMigratePricingFlagValues("ui",
					[]storj.PlacementConstraint{0}, nil, nil,
					[]storj.PlacementConstraint{0, 99}, 0)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--known-placement-ids")
			})
			t.Run("UnknownSunsetOld", func(t *testing.T) {
				err := validateMigratePricingFlagValues("ui",
					[]storj.PlacementConstraint{0},
					map[storj.PlacementConstraint]storj.PlacementConstraint{99: 0},
					nil, []storj.PlacementConstraint{0}, 0)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--sunset-default-placements")
			})
			t.Run("UnknownSunsetNew", func(t *testing.T) {
				err := validateMigratePricingFlagValues("ui",
					[]storj.PlacementConstraint{0},
					map[storj.PlacementConstraint]storj.PlacementConstraint{30: 99},
					nil, []storj.PlacementConstraint{0, 30}, 0)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--sunset-default-placements")
			})
			t.Run("UnknownNewPPMPlacement", func(t *testing.T) {
				err := validateMigratePricingFlagValues("billing",
					nil, nil, entitlements.PlacementProductMappings{99: 20},
					[]storj.PlacementConstraint{0, 12}, 20)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--new-placement-product-map")
			})
			t.Run("UnknownNewPPMProduct", func(t *testing.T) {
				err := validateMigratePricingFlagValues("billing",
					nil, nil, entitlements.PlacementProductMappings{0: 99},
					[]storj.PlacementConstraint{0, 12}, 20)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--new-placement-product-map")
			})
			t.Run("UnknownFallbackProduct", func(t *testing.T) {
				err := validateMigratePricingFlagValues("billing",
					nil, nil, entitlements.PlacementProductMappings{0: 20},
					[]storj.PlacementConstraint{0, 12}, 99)
				require.Error(t, err)
				require.Contains(t, err.Error(), "--fallback-product-id")
			})
			t.Run("FallbackProductNotCheckedForUIPhase", func(t *testing.T) {
				err := validateMigratePricingFlagValues("ui",
					[]storj.PlacementConstraint{0}, nil, nil,
					[]storj.PlacementConstraint{0}, 999)
				require.NoError(t, err)
			})
		})
	})
}

func TestMigratePricing_Phase1(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())
		log := zaptest.NewLogger(t)

		const (
			placementA       storj.PlacementConstraint = 0
			placementB       storj.PlacementConstraint = 12
			sunsetA          storj.PlacementConstraint = 30
			sunsetB          storj.PlacementConstraint = 31
			sunsetC          storj.PlacementConstraint = 32
			unknownPlacement storj.PlacementConstraint = 99
			productA         int32                     = 20
			productB         int32                     = 21
		)

		// Known placements and the sunset map mirror the real US1 invocation
		// (--known-placement-ids 0,12,30,31,32 --sunset-default-placements 30:0,31:12,32:0):
		// the target placements plus the sunset placements. Anything else is custom.
		knownSet := placementSet([]storj.PlacementConstraint{placementA, placementB, sunsetA, sunsetB, sunsetC})
		sunsetMap := map[storj.PlacementConstraint]storj.PlacementConstraint{sunsetA: placementA, sunsetB: placementB, sunsetC: placementA}
		targetNBP := []storj.PlacementConstraint{placementA, placementB}

		baseArgs := migratePricingArgs{
			targetNewBucketPlacements:   targetNBP,
			sunsetMap:                   sunsetMap,
			newPlacementProductMappings: entitlements.PlacementProductMappings{placementA: productA, placementB: productB},
			knownSet:                    knownSet,
			phase:                       "ui",
		}

		user, err := sat.AddUser(ctx, console.CreateUser{FullName: "Test User", Email: "test@example.com"}, 5)
		require.NoError(t, err)

		// Standard project with no existing entitlement row → should be created with targetNewBucketPlacements.
		t.Run("NewEntitlementRowCreated", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-no-ent")
			require.NoError(t, err)

			// Confirm no entitlement exists yet.
			_, err = entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.True(t, entitlements.ErrNotFound.Has(err))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, targetNBP, feats.NewBucketPlacements)
			require.Equal(t, 1, counts.updated)
		})

		// Standard project whose NewBucketPlacements already equals target and has no sunset placements → noChange.
		t.Run("AlreadyUpToDate", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-up-to-date")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			require.Equal(t, 0, counts.updated)
			require.Equal(t, 1, counts.noChange)
		})

		// NewBucketPlacements [12] is a non-sunset subset of the target {0,12} — a
		// surviving tier (Advanced). It contains no sunset placement, so it is left
		// unchanged rather than widened to the full target [0,12].
		t.Run("NonSunsetSubsetLeftUnchanged", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-subset")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementB}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, []storj.PlacementConstraint{placementB}, feats.NewBucketPlacements)
			require.Equal(t, 0, counts.updated)
			require.Equal(t, 1, counts.noChange)
		})

		// NewBucketPlacements with a single sunset placement should migrate to the single
		// corresponding replacement, not the full target set.
		// e.g. [31] (US-regional) → [12] (Advanced), not [0,12].
		t.Run("SingleSunsetPlacementMappedToSingleValue", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-single-sunset")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{sunsetB}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			// [31] should become [12] (its sunset replacement), not the full target [0,12].
			require.Equal(t, []storj.PlacementConstraint{placementB}, feats.NewBucketPlacements)
			require.Equal(t, 1, counts.updated)
		})

		// A single sunset placement that maps to Standard migrates to [0], not [0,12].
		// e.g. [32] (Archive) → [0] (Standard).
		t.Run("SingleSunsetPlacementMappedToStandard", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-single-sunset-archive")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{sunsetC}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			// [32] should become [0] (its sunset replacement), not the full target [0,12].
			require.Equal(t, []storj.PlacementConstraint{placementA}, feats.NewBucketPlacements)
			require.Equal(t, 1, counts.updated)
		})

		// Combined sunset placements collapse to the deduplicated target set.
		// e.g. [30,31,32] → [0,12] (30→0, 31→12, 32→0, deduped).
		t.Run("CombinedSunsetPlacementsCollapseToTarget", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-combined-sunset")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{sunsetA, sunsetB, sunsetC}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, []storj.PlacementConstraint{placementA, placementB}, feats.NewBucketPlacements)
			require.Equal(t, 1, counts.updated)
		})

		// Standard project with a sunset DefaultPlacement → DefaultPlacement migrated.
		t.Run("DefaultPlacementMigrated", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-sunset-dp")
			require.NoError(t, err)

			require.NoError(t, sat.DB.Console().Projects().UpdateDefaultPlacement(ctx, proj.ID, sunsetA))
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))

			updatedProj, err := sat.DB.Console().Projects().Get(ctx, proj.ID)
			require.NoError(t, err)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *updatedProj, baseArgs, &counts))

			// DefaultPlacement should now be placementA (sunsetMap[sunsetA] = placementA).
			afterProj, err := sat.DB.Console().Projects().Get(ctx, proj.ID)
			require.NoError(t, err)
			require.Equal(t, placementA, afterProj.DefaultPlacement)
			require.Equal(t, 1, counts.updated)
		})

		// Custom project (NewBucketPlacements contains placement outside knownSet) → skipped in Phase 1.
		t.Run("CustomProjectSkipped", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-custom")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{unknownPlacement}))

			updatedProj, err := sat.DB.Console().Projects().Get(ctx, proj.ID)
			require.NoError(t, err)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *updatedProj, baseArgs, &counts))

			// NewBucketPlacements should be unchanged.
			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, []storj.PlacementConstraint{unknownPlacement}, feats.NewBucketPlacements)
			require.Equal(t, 1, counts.skipped)
		})

		t.Run("DryRunNoWrites", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-dryrun-p1")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{sunsetA}))
			require.NoError(t, sat.DB.Console().Projects().UpdateDefaultPlacement(ctx, proj.ID, sunsetA))

			updatedProj, err := sat.DB.Console().Projects().Get(ctx, proj.ID)
			require.NoError(t, err)

			dryArgs := baseArgs
			dryArgs.dryRun = true
			dryArgs.knownSet = placementSet([]storj.PlacementConstraint{placementA, placementB, sunsetA})

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase1(ctx, log, sat.DB, entService, *updatedProj, dryArgs, &counts))

			// NewBucketPlacements and DefaultPlacement must be unchanged after dry-run.
			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, []storj.PlacementConstraint{sunsetA}, feats.NewBucketPlacements)

			afterProj, err := sat.DB.Console().Projects().Get(ctx, proj.ID)
			require.NoError(t, err)
			require.Equal(t, sunsetA, afterProj.DefaultPlacement)
		})

		t.Run("InactiveProjectsSkipped", func(t *testing.T) {
			activeProj, err := sat.AddProject(ctx, user.ID, "proj-inactive-active")
			require.NoError(t, err)

			disabledProj, err := sat.AddProject(ctx, user.ID, "proj-inactive-disabled")
			require.NoError(t, err)
			require.NoError(t, sat.DB.Console().Projects().UpdateStatus(ctx, disabledProj.ID, console.ProjectDisabled))

			pendingProj, err := sat.AddProject(ctx, user.ID, "proj-inactive-pending")
			require.NoError(t, err)
			require.NoError(t, sat.DB.Console().Projects().UpdateStatus(ctx, pendingProj.ID, console.ProjectPendingDeletion))

			require.NoError(t, runMigratePricing(ctx, log, sat.DB, entService, baseArgs))

			feats, err := entService.Projects().GetByPublicID(ctx, activeProj.PublicID)
			require.NoError(t, err)
			require.Equal(t, targetNBP, feats.NewBucketPlacements)

			_, err = entService.Projects().GetByPublicID(ctx, disabledProj.PublicID)
			require.True(t, entitlements.ErrNotFound.Has(err), "disabled project should have no entitlement row")

			_, err = entService.Projects().GetByPublicID(ctx, pendingProj.PublicID)
			require.True(t, entitlements.ErrNotFound.Has(err), "pending-deletion project should have no entitlement row")
		})
	})
}

func TestMigratePricing_Phase2(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())
		log := zaptest.NewLogger(t)

		const (
			placementA       storj.PlacementConstraint = 0
			placementB       storj.PlacementConstraint = 12
			customPlacementA storj.PlacementConstraint = 55
			customPlacementB storj.PlacementConstraint = 77
			productA         int32                     = 20
			productB         int32                     = 21
			fallbackProduct  int32                     = 99
		)

		knownSet := placementSet([]storj.PlacementConstraint{placementA, placementB})
		newPPM := entitlements.PlacementProductMappings{placementA: productA, placementB: productB}

		baseArgs := migratePricingArgs{
			targetNewBucketPlacements:   []storj.PlacementConstraint{placementA, placementB},
			newPlacementProductMappings: newPPM,
			knownSet:                    knownSet,
			fallbackProduct:             fallbackProduct,
			phase:                       "billing",
		}

		user, err := sat.AddUser(ctx, console.CreateUser{FullName: "Test User", Email: "test2@example.com"}, 5)
		require.NoError(t, err)

		// Standard project: PlacementProductMappings replaced entirely with newPPM.
		t.Run("StandardProjectPPMReplaced", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-std-PlacementProductMappings")
			require.NoError(t, err)

			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: 1}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, newPPM, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.updated)
			require.Equal(t, 0, counts.custom)
		})

		t.Run("CustomProjectPPMMerged", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-custom-PlacementProductMappings")
			require.NoError(t, err)

			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, customPlacementA}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: 1}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			// Expect newPPM entries for known placements, plus fallbackProduct for customPlacementA.
			require.Equal(t, entitlements.PlacementProductMappings{placementA: productA, placementB: productB, customPlacementA: fallbackProduct}, feats.PlacementProductMappings)
			require.Equal(t, 0, counts.updated)
			require.Equal(t, 1, counts.custom)
		})

		// Custom project: fallback product ID not used if unknown placement already in PlacementProductMappings.
		t.Run("CustomProjectFallbackNotDuplicated", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-custom-no-dup")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, customPlacementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{customPlacementB: 50}))

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			// customPlacementB keeps its existing value (50), not overwritten by fallbackProduct.
			require.Equal(t, int32(50), feats.PlacementProductMappings[customPlacementB])
			require.Equal(t, 1, counts.custom)
		})

		// Dry-run: no writes happen.
		t.Run("DryRunNoWrites", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "proj-dryrun-p2")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: 1}))

			dryArgs := baseArgs
			dryArgs.dryRun = true

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, dryArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			// PlacementProductMappings unchanged.
			require.Equal(t, entitlements.PlacementProductMappings{placementA: 1}, feats.PlacementProductMappings)
		})
	})
}

func TestMigratePricing_Phase2_LegacyPricingCarveOut(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())
		log := zaptest.NewLogger(t)

		const (
			placementA            storj.PlacementConstraint = 0
			placementB            storj.PlacementConstraint = 12
			customPlacement       storj.PlacementConstraint = 55
			newProductA           int32                     = 20
			newProductB           int32                     = 21
			legacyProductA        int32                     = 10
			legacyProductB        int32                     = 11
			legacyFallbackProduct int32                     = 12
			fallbackProduct       int32                     = 99
			legacyUserAgent                                 = "ix-storj-1"
			otherUserAgent                                  = "someone-else"
		)

		knownSet := placementSet([]storj.PlacementConstraint{placementA, placementB})

		// The cohort carve-out pins these placements to legacy products.
		legacyPPM := entitlements.PlacementProductMappings{placementA: legacyProductA, placementB: legacyProductB}

		baseArgs := migratePricingArgs{
			targetNewBucketPlacements:      []storj.PlacementConstraint{placementA, placementB},
			newPlacementProductMappings:    entitlements.PlacementProductMappings{placementA: newProductA, placementB: newProductB},
			knownSet:                       knownSet,
			fallbackProduct:                fallbackProduct,
			phase:                          "billing",
			legacyUserAgents:               map[string]struct{}{legacyUserAgent: {}},
			legacyPlacementProductMappings: legacyPPM,
			legacyFallbackProduct:          legacyFallbackProduct,
		}

		user, err := sat.AddUser(ctx, console.CreateUser{FullName: "Legacy User", Email: "legacy@example.com"}, 5)
		require.NoError(t, err)

		// Standard cohort project: existing PPM is overlaid with the legacy map, never the new one.
		t.Run("StandardCohortPinned", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "legacy-std")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: newProductA}))

			proj.UserAgent = []byte(legacyUserAgent)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, legacyPPM, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.frozen)
			require.Equal(t, 0, counts.updated)
			require.Equal(t, 0, counts.custom)
		})

		// Custom cohort project: custom placement mapping is preserved, cohort placements pinned legacy.
		t.Run("CustomCohortPreservesCustomMapping", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "legacy-custom")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, customPlacement}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: newProductA, customPlacement: 50}))

			proj.UserAgent = []byte(legacyUserAgent)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, entitlements.PlacementProductMappings{placementA: legacyProductA, placementB: legacyProductB, customPlacement: 50}, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.frozen)
			require.Equal(t, 0, counts.custom)
		})

		// Custom cohort project with an unmapped unknown placement: it gets the legacy fallback
		// product (never the migrated fallback), so it can't fall through to new pricing.
		t.Run("CustomCohortUnknownPlacementGetsLegacyFallback", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "legacy-custom-fallback")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, customPlacement}))
			// customPlacement has NO existing mapping.
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: newProductA}))

			proj.UserAgent = []byte(legacyUserAgent)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, entitlements.PlacementProductMappings{placementA: legacyProductA, placementB: legacyProductB, customPlacement: legacyFallbackProduct}, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.frozen)
		})

		// Cohort project with no entitlement row: legacy map is written from scratch.
		t.Run("NoEntitlementRowCohortPinned", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "legacy-norow")
			require.NoError(t, err)
			proj.UserAgent = []byte(legacyUserAgent)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, legacyPPM, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.frozen)
		})

		// Non-cohort project: carve-out does not apply; standard migration runs.
		t.Run("NonCohortMigratedNormally", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "non-cohort")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: 1}))

			proj.UserAgent = []byte(otherUserAgent)

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, baseArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, entitlements.PlacementProductMappings{placementA: newProductA, placementB: newProductB}, feats.PlacementProductMappings)
			require.Equal(t, 0, counts.frozen)
			require.Equal(t, 1, counts.updated)
		})

		// Dry-run cohort: nothing is written.
		t.Run("DryRunCohortNoWrites", func(t *testing.T) {
			proj, err := sat.AddProject(ctx, user.ID, "legacy-dryrun")
			require.NoError(t, err)
			require.NoError(t, entService.Projects().SetNewBucketPlacementsByPublicID(ctx, proj.PublicID, []storj.PlacementConstraint{placementA, placementB}))
			require.NoError(t, entService.Projects().SetPlacementProductMappingsByPublicID(ctx, proj.PublicID, entitlements.PlacementProductMappings{placementA: newProductA}))

			proj.UserAgent = []byte(legacyUserAgent)

			dryArgs := baseArgs
			dryArgs.dryRun = true

			var counts migratePricingCounts
			require.NoError(t, migratePricingPhase2(ctx, log, entService, *proj, dryArgs, &counts))

			feats, err := entService.Projects().GetByPublicID(ctx, proj.PublicID)
			require.NoError(t, err)
			require.Equal(t, entitlements.PlacementProductMappings{placementA: newProductA}, feats.PlacementProductMappings)
			require.Equal(t, 1, counts.frozen)
		})
	})
}

func TestSetEntitlement_Validation(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		UplinkCount: 0, SatelliteCount: 1, StorageNodeCount: 0,
		NonParallel: true,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Placement = nodeselection.ConfigurablePlacementRule{
					PlacementRules: `0:annotation("location", "global")`,
				}

				var placementProductMap paymentsconfig.PlacementProductMap
				placementProductMap.SetMap(map[int]int32{
					0: 1,
				})
				config.Payments.PlacementPriceOverrides = placementProductMap

				price := paymentsconfig.ProjectUsagePrice{
					StorageTB: "4",
					EgressTB:  "7",
					Segment:   "0.0000088",
				}
				var productOverrides paymentsconfig.ProductPriceOverrides
				productOverrides.SetMap(map[int32]paymentsconfig.ProductUsagePrice{
					1: {ProjectUsagePrice: price},
					2: {ProjectUsagePrice: price},
				})
				config.Payments.Products = productOverrides

				config.Console.Placement.SelfServeDetails = []console.PlacementDetail{
					{ID: 0},
				}
				config.Console.Placement.AllowedPlacementIdsForNewProjects = []storj.PlacementConstraint{0}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		runCfg.Placement = sat.Config.Placement
		runCfg.Payments = sat.Config.Payments
		runCfg.Console = sat.Config.Console

		user, err := sat.AddUser(ctx, console.CreateUser{
			FullName: "Test User",
			Email:    "test@test.test",
		}, 1)
		require.NoError(t, err)

		entitlementUserEmail = user.Email

		p1, err := sat.AddProject(ctx, user.ID, "testproject")
		require.NoError(t, err)
		p2, err := sat.AddProject(ctx, user.ID, "testproject2")
		require.NoError(t, err)

		err = sat.DB.Console().Projects().UpdateUserAgent(ctx, p2.ID, []byte("part1"))
		require.NoError(t, err)

		t.Run("BothEmailAndCSVFlags", func(t *testing.T) {
			entitlementUserEmailCSV = "test.csv"
			entitlementJSON = ""
			entitlementSkipConfirm = true

			err := cmdSetNewBucketPlacements(nil, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot specify both --email and --csv flags")

			err = setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot specify both --email and --csv flags")
		})

		t.Run("InvalidJSONPlacements", func(t *testing.T) {
			t.Run("InvalidJSONPlacements", func(t *testing.T) {
				entitlementUserEmailCSV = ""
				entitlementJSON = "invalid-json"
				entitlementSkipConfirm = true

				err := cmdSetNewBucketPlacements(nil, nil)
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid JSON format for placements")

				err = setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid JSON format for placement-product mapping")
			})

			t.Run("InvalidMappingsAndPlacement", func(t *testing.T) {

				t.Run("SetNewBucketPlacements", func(t *testing.T) {
					entitlementJSON = `[20]`
					err := cmdSetNewBucketPlacements(nil, nil)
					require.Error(t, err)
					require.Contains(t, err.Error(), "invalid placement ID: 20")
				})

				t.Run("SetPlacementProductMap", func(t *testing.T) {
					entitlementJSON = `{"0": 3}`
					err = setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
					require.Error(t, err)
					require.Contains(t, err.Error(), "invalid product ID: 3")

					entitlementJSON = `{"20": 1}`
					err = setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
					require.Error(t, err)
					require.Contains(t, err.Error(), "invalid placement ID: 20")
				})
			})

			t.Run("AllValidationsPass", func(t *testing.T) {
				t.Run("SetPlacementProductMap", func(t *testing.T) {
					entitlementJSON = `{"0": 1}`
					err := setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
					require.NoError(t, err)

					entService := entitlements.NewService(zaptest.NewLogger(t).Named("entitlements"), sat.DB.Console().Entitlements())

					// test that entitlement was set
					for _, p := range []uuid.UUID{p1.PublicID, p2.PublicID} {
						features, err := entService.Projects().GetByPublicID(ctx, p)
						require.NoError(t, err)
						require.EqualValues(t, entitlements.PlacementProductMappings{
							0: 1,
						}, features.PlacementProductMappings)
					}

					// reset entitlement
					entitlementJSON = ""
					err = entService.Projects().DeleteByPublicID(ctx, p1.PublicID)
					require.NoError(t, err)
					err = entService.Projects().DeleteByPublicID(ctx, p2.PublicID)
					require.NoError(t, err)

					err = setPlacementProductMap(ctx, testplanet.NewLogger(t), sat.DB)
					require.NoError(t, err)

					// expect global mappings to be set
					features, err := entService.Projects().GetByPublicID(ctx, p1.PublicID)
					require.NoError(t, err)
					require.EqualValues(t, entitlements.PlacementProductMappings{
						0: 1,
					}, features.PlacementProductMappings)

					features, err = entService.Projects().GetByPublicID(ctx, p2.PublicID)
					require.NoError(t, err)
					require.EqualValues(t, entitlements.PlacementProductMappings{
						0: 1,
					}, features.PlacementProductMappings)
				})
			})
		})
	})
}
