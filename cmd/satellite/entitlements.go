// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/satellitedb"
)

var (
	setNewBucketPlacementsEmail       string
	setNewBucketPlacementsCSV         string
	setNewBucketPlacementsJSON        string
	setNewBucketPlacementsSkipConfirm bool
)

type processingArgs struct {
	log               *zap.Logger
	satDB             satellite.DB
	entService        *entitlements.Service
	newPlacements     []storj.PlacementConstraint
	allowedPlacements []storj.PlacementConstraint
}

func cmdSetNewBucketPlacements(cmd *cobra.Command, _ []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// Validate that only one target option is provided.
	if setNewBucketPlacementsEmail != "" && setNewBucketPlacementsCSV != "" {
		return errs.New("cannot specify both --email and --csv flags, please use only one")
	}

	var newPlacements []storj.PlacementConstraint
	if setNewBucketPlacementsJSON != "" {
		if err := json.Unmarshal([]byte(setNewBucketPlacementsJSON), &newPlacements); err != nil {
			return errs.New("invalid JSON format for placements: %+v", err)
		}
	}

	if newPlacements == nil {
		log.Info("Setting new bucket placements to default values")
	} else {
		for _, placementID := range newPlacements {
			if _, ok := runCfg.Console.Placement.SelfServeDetails.Get(placementID); !ok {
				return errs.New("invalid placement ID: %d", placementID)
			}
		}

		log.Info("Setting new bucket placements to provided value")
	}

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName: "satellite-entitlements",
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	entitlementsService := entitlements.NewService(log.Named("entitlements"), satDB.Console().Entitlements())
	args := processingArgs{
		log:               log,
		satDB:             satDB,
		entService:        entitlementsService,
		newPlacements:     newPlacements,
		allowedPlacements: runCfg.Console.Placement.AllowedPlacementIdsForNewProjects,
	}

	// Determine which users/projects to target.
	if setNewBucketPlacementsEmail != "" {
		return processUserEmail(ctx, setNewBucketPlacementsEmail, args, true)
	} else if setNewBucketPlacementsCSV != "" {
		return processCSVFile(ctx, setNewBucketPlacementsCSV, args)
	} else {
		// Process ALL active projects of ALL active users.
		return processAllUsers(ctx, args)
	}
}

func processUserEmail(ctx context.Context, email string, args processingArgs, validate bool) error {
	if validate && !utils.ValidateEmail(email) {
		return errs.New("invalid email format: %s", email)
	}

	args.log.Info("Processing single user", zap.String("email", email))

	user, err := args.satDB.Console().Users().GetByEmail(ctx, email)
	if err != nil {
		return err
	}

	if user.Status != console.Active {
		return errs.New("user with email %s is not active", email)
	}

	projects, err := args.satDB.Console().Projects().GetOwnActive(ctx, user.ID)
	if err != nil {
		return errs.New("error fetching active projects for user %s: %+v", email, err)
	}

	for _, project := range projects {
		if args.newPlacements != nil {
			if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.newPlacements); err != nil {
				return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
			}
			continue
		}

		if project.DefaultPlacement == storj.DefaultPlacement {
			if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.allowedPlacements); err != nil {
				return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
			}
			continue
		}

		if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, []storj.PlacementConstraint{project.DefaultPlacement}); err != nil {
			return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
		}
	}

	args.log.Info("Successfully updated new bucket placements for user", zap.String("email", email), zap.Int("project_count", len(projects)))

	return nil
}

func processCSVFile(ctx context.Context, csvPath string, args processingArgs) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return errs.New("error opening CSV file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return errs.New("error reading CSV file: %+v", err)
	}

	if len(records) == 0 {
		return errs.New("CSV file is empty")
	}

	var emails []string
	var invalidEmails []string
	for i, record := range records {
		if len(record) == 0 {
			continue
		}
		email := strings.TrimSpace(record[0])

		// Skip the header row if it doesn't look like an email.
		if i == 0 && !utils.ValidateEmail(email) {
			args.log.Info("Skipping header row", zap.String("header", email))
			continue
		}

		if !utils.ValidateEmail(email) {
			invalidEmails = append(invalidEmails, email)
			continue
		}

		emails = append(emails, email)
	}

	if len(invalidEmails) > 0 {
		return errs.New("CSV file contains invalid email addresses: %v", invalidEmails)
	}
	if len(emails) == 0 {
		return errs.New("no valid emails found in CSV file")
	}

	if !setNewBucketPlacementsSkipConfirm {
		if !askForConfirmation(fmt.Sprintf("Set bucket placements for %d users from CSV file?", len(emails))) {
			args.log.Info("Operation cancelled by user")
			return nil
		}
	}

	args.log.Info("Processing CSV users", zap.Int("count", len(emails)))

	var errList errs.Group
	for _, email := range emails {
		if err = processUserEmail(ctx, email, args, false); err != nil {
			errList.Add(err)
		}
	}
	if err = errList.Err(); err != nil {
		return errs.New("errors occurred while processing CSV users: %+v", err)
	}

	args.log.Info("Successfully updated new bucket placements for all users from CSV file", zap.Int("count", len(emails)))

	return nil
}

func processAllUsers(ctx context.Context, args processingArgs) error {
	if !setNewBucketPlacementsSkipConfirm {
		if !askForConfirmation("Set bucket placements for ALL active projects of ALL active users?") {
			args.log.Info("Operation cancelled by user")
			return nil
		}
	}

	args.log.Info("Processing all users and their projects")

	var errList errs.Group
	cursor := console.UserCursor{Limit: 500, Page: 1}

	for {
		usersPage, err := args.satDB.Console().Users().GetByStatus(ctx, console.Active, cursor)
		if err != nil {
			return errs.New("error fetching active users: %+v", err)
		}

		if len(usersPage.Users) == 0 {
			break
		}

		args.log.Info("Processing users batch", zap.Int("count", len(usersPage.Users)), zap.Uint("page", cursor.Page))

		for _, user := range usersPage.Users {
			projects, err := args.satDB.Console().Projects().GetOwnActive(ctx, user.ID)
			if err != nil {
				errList.Add(errs.New("error fetching active projects for user %s: %+v", user.Email, err))
				continue
			}

			for _, project := range projects {
				if args.newPlacements != nil {
					if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.newPlacements); err != nil {
						errList.Add(errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err))
						continue
					}
					continue
				}

				if project.DefaultPlacement == storj.DefaultPlacement {
					if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.allowedPlacements); err != nil {
						errList.Add(errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err))
						continue
					}
					continue
				}

				if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, []storj.PlacementConstraint{project.DefaultPlacement}); err != nil {
					errList.Add(errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err))
				}
			}
		}

		if cursor.Page >= usersPage.PageCount {
			break
		}
		cursor.Page++
	}
	if err := errList.Err(); err != nil {
		return errs.New("errors occurred while processing users: %+v", err)
	}

	args.log.Info("Successfully updated new bucket placements for all active users and their projects")

	return nil
}

func askForConfirmation(prompt string) bool {
	fmt.Printf("%s (y/n): ", prompt)
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		response := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if response == "y" || response == "yes" {
			return true
		} else if response == "n" || response == "no" {
			return false
		}
		fmt.Print("Please enter 'y' for yes or 'n' for no: ")
	}

	return false
}
