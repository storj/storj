// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/satellitedb"
)

const (
	actionSetNewBucketPlacements int = iota
	actionSetPlacementProductMap
)

var (
	entitlementUserEmail    string
	entitlementUserEmailCSV string
	entitlementJSON         string
	entitlementSkipConfirm  bool
	entitlementVerbose      bool

	// pricing migration vars
	mpFlagTargetNBP         string
	mpFlagSunsetPlacements  string
	mpFlagNewPPM            string
	mpFlagKnownPlacements   string
	mpFlagFallbackProductID int32
	mpFlagPhase             string
	mpFlagDryRun            bool
)

type processingArgs struct {
	log               *zap.Logger
	satDB             satellite.DB
	entService        *entitlements.Service
	newPlacements     []storj.PlacementConstraint
	allowedPlacements []storj.PlacementConstraint
	skipConfirm       bool
	verbose           bool

	placementProductMap        entitlements.PlacementProductMappings
	defaultPlacementProductMap payments.PlacementProductIdMap

	action int
}

func cmdSetNewBucketPlacements(cmd *cobra.Command, _ []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// Validate that only one target option is provided.
	if entitlementUserEmail != "" && entitlementUserEmailCSV != "" {
		return errs.New("cannot specify both --email and --csv flags, please use only one")
	}

	var newPlacements []storj.PlacementConstraint
	if entitlementJSON != "" {
		if err := json.Unmarshal([]byte(entitlementJSON), &newPlacements); err != nil {
			return errs.New("invalid JSON format for placements: %+v", err)
		}
	}

	if newPlacements == nil {
		log.Info("Setting new bucket placements to default values")
	} else {
		placements, err := runCfg.Placement.Parse(runCfg.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return err
		}

		for _, placementID := range newPlacements {
			placementValid := false
			for constraint := range placements {
				if constraint == placementID {
					placementValid = true
					break
				}
			}
			if !placementValid {
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
		skipConfirm:       entitlementSkipConfirm,
		verbose:           entitlementVerbose,
		action:            actionSetNewBucketPlacements,
	}

	if args.verbose {
		log.Info("Setting new bucket placements", zap.Any("placements", newPlacements), zap.Any("allowed_placements", args.allowedPlacements))
	}

	// Determine which users/projects to target.
	if entitlementUserEmail != "" {
		return processUserEmail(ctx, entitlementUserEmail, args, true)
	} else if entitlementUserEmailCSV != "" {
		return processCSVFile(ctx, entitlementUserEmailCSV, args)
	} else {
		// Process ALL active projects of ALL active users.
		return processAllUsers(ctx, args)
	}
}

func cmdSetPlacementProductMap(cmd *cobra.Command, _ []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName: "satellite-entitlements",
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	return setPlacementProductMap(ctx, log, satDB)
}

// this method is separated to allow easier testing with a mock satellitedb
func setPlacementProductMap(ctx context.Context, log *zap.Logger, satDB satellite.DB) error {
	// Validate that only one target option is provided.
	if entitlementUserEmail != "" && entitlementUserEmailCSV != "" {
		return errs.New("cannot specify both --email and --csv flags, please use only one")
	}

	var mappings entitlements.PlacementProductMappings
	if entitlementJSON != "" {
		if err := json.Unmarshal([]byte(entitlementJSON), &mappings); err != nil {
			return errs.New("invalid JSON format for placement-product mapping: %+v", err)
		}
	}

	var placementMapping payments.PlacementProductIdMap
	if mappings == nil {

		placementMapping = runCfg.Payments.PlacementPriceOverrides.ToMap()

		logArgs := make([]zap.Field, 0)
		if entitlementVerbose {
			logArgs = append(logArgs, zap.Any("mapping", placementMapping))
		}
		log.Info("Setting new bucket placements using default placement mappings", logArgs...)
	} else {
		productPrices, err := runCfg.Payments.Products.ToModels()
		if err != nil {
			return errs.New("error converting product prices: %+v", err)
		}

		placements, err := runCfg.Placement.Parse(runCfg.Overlay.Node.CreateDefaultPlacement, nil)
		if err != nil {
			return err
		}

		for placementID, productID := range mappings {
			if _, ok := productPrices[productID]; !ok {
				return errs.New("invalid product ID: %d", productID)
			}

			placementValid := false
			for constraint := range placements {
				if constraint == placementID {
					placementValid = true
					break
				}
			}
			if !placementValid {
				return errs.New("invalid placement ID: %d", placementID)
			}
		}

		logArgs := make([]zap.Field, 0)
		if entitlementVerbose {
			logArgs = append(logArgs, zap.Any("mapping", mappings))
		}
		log.Info("Setting placement-product mapping to provided values", logArgs...)
	}

	entitlementsService := entitlements.NewService(log.Named("entitlements"), satDB.Console().Entitlements())
	args := processingArgs{
		log:                        log,
		satDB:                      satDB,
		entService:                 entitlementsService,
		placementProductMap:        mappings,
		defaultPlacementProductMap: placementMapping,
		skipConfirm:                entitlementSkipConfirm,
		verbose:                    entitlementVerbose,
		action:                     actionSetPlacementProductMap,
	}

	// Determine which users/projects to target.
	if entitlementUserEmail != "" {
		return processUserEmail(ctx, entitlementUserEmail, args, true)
	} else if entitlementUserEmailCSV != "" {
		return processCSVFile(ctx, entitlementUserEmailCSV, args)
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

	user, err := args.satDB.Console().Users().GetByEmailAndTenant(ctx, email, nil)
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
		if err = processProject(ctx, project, args); err != nil {
			return err
		}
	}

	actionTxt := "new bucket placements"
	if args.action == actionSetPlacementProductMap {
		actionTxt = "placement-product mapping"
	}
	args.log.Info(fmt.Sprintf("Successfully updated %s for user", actionTxt), zap.String("email", email), zap.Int("project_count", len(projects)))

	return nil
}

func processProject(ctx context.Context, project console.Project, args processingArgs) (err error) {
	if args.action == actionSetPlacementProductMap {
		placementProductMap := args.placementProductMap
		if placementProductMap == nil {
			placementProductMap = entitlements.PlacementProductMappings{}
			for placement, productID := range args.defaultPlacementProductMap {
				placementProductMap[storj.PlacementConstraint(placement)] = productID
			}
		}

		if err = args.entService.Projects().SetPlacementProductMappingsByPublicID(ctx, project.PublicID, placementProductMap); err != nil {
			return errs.New("error setting placement-product mapping for project %s: %+v", project.PublicID, err)
		}

		if args.verbose {
			args.log.Info("Set placement-product mapping for project", zap.String("project_id", project.PublicID.String()), zap.Any("mapping", placementProductMap))
		}

		return nil
	}

	if args.newPlacements != nil {
		if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.newPlacements); err != nil {
			return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
		}
		return nil
	}

	newPlacements := args.allowedPlacements
	if project.DefaultPlacement == storj.DefaultPlacement {
		if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.allowedPlacements); err != nil {
			return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
		}
	} else {
		newPlacements = []storj.PlacementConstraint{project.DefaultPlacement}
		if err = args.entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, []storj.PlacementConstraint{project.DefaultPlacement}); err != nil {
			return errs.New("error setting new bucket placements for project %s: %+v", project.PublicID, err)
		}
	}

	if args.verbose {
		args.log.Info("Set new bucket placements for project", zap.String("project_id", project.PublicID.String()), zap.Any("placements", newPlacements))
	}

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

	if !args.skipConfirm {
		actionText := "Set bucket placements"
		if args.action == actionSetPlacementProductMap {
			actionText = "Set placement-product mapping"
		}
		if !askForConfirmation(fmt.Sprintf("%s for %d users from CSV file?", actionText, len(emails))) {
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

	actionText := "new bucket placements"
	if args.action == actionSetPlacementProductMap {
		actionText = "placement-product mapping"
	}

	args.log.Info(fmt.Sprintf("Successfully updated %s for all users from CSV file", actionText), zap.Int("count", len(emails)))

	return nil
}

func processAllUsers(ctx context.Context, args processingArgs) error {
	if !args.skipConfirm {
		actionText := "Set bucket placements"
		if args.action == actionSetPlacementProductMap {
			actionText = "Set placement-product mapping"
		}
		if !askForConfirmation(fmt.Sprintf("%s for ALL active projects of ALL active users?", actionText)) {
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
				if err = processProject(ctx, project, args); err != nil {
					errList.Add(err)
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

	actionTxt := "new bucket placements"
	if args.action == actionSetPlacementProductMap {
		actionTxt = "placement-product mapping"
	}
	args.log.Info(fmt.Sprintf("Successfully updated %s for all active users and their projects", actionTxt))

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

// migratePricingArgs holds all parsed arguments for the migrate-pricing command.
type migratePricingArgs struct {
	targetNewBucketPlacements []storj.PlacementConstraint
	// map of placements being sunset to their replacements
	sunsetMap                   map[storj.PlacementConstraint]storj.PlacementConstraint
	newPlacementProductMappings entitlements.PlacementProductMappings
	// known set of placements
	knownSet        map[storj.PlacementConstraint]struct{}
	fallbackProduct int32
	phase           string
	dryRun          bool
}

// migratePricingCounts tracks per-run statistics.
type migratePricingCounts struct {
	updated  int
	skipped  int
	noChange int
	custom   int
}

// validateMigratePricingFlags checks the migrate-pricing command-line flags for
// completeness based on the requested phase.
func validateMigratePricingFlags(phase, targetNBP, newPlacementProductMappings, knownPlacements string, fallbackProductSet bool) error {
	if phase != "ui" && phase != "billing" {
		return errs.New("--phase must be 'ui' or 'billing'")
	}
	if knownPlacements == "" {
		return errs.New("--known-placement-ids is required")
	}
	if phase == "ui" && targetNBP == "" {
		return errs.New("--target-new-bucket-placements is required for the ui phase")
	}
	if phase == "billing" {
		if newPlacementProductMappings == "" {
			return errs.New("--new-placement-product-map is required for phase billing")
		}
		if !fallbackProductSet {
			return errs.New("--fallback-product-id is required for phase billing")
		}
	}
	return nil
}

// validateMigratePricingFlagValues checks that every parsed placement ID and product ID
// supplied via flags are actually configured.
func validateMigratePricingFlagValues(
	phase string,
	targetNBP []storj.PlacementConstraint,
	sunsetMap map[storj.PlacementConstraint]storj.PlacementConstraint,
	newPPM entitlements.PlacementProductMappings,
	knownList []storj.PlacementConstraint,
	fallbackProductID int32,
) error {
	placements, err := runCfg.Placement.Parse(runCfg.Overlay.Node.CreateDefaultPlacement, nil)
	if err != nil {
		return err
	}
	products, err := runCfg.Payments.Products.ToModels()
	if err != nil {
		return errs.New("error loading product config: %+v", err)
	}

	for _, p := range targetNBP {
		if _, ok := placements[p]; !ok {
			return errs.New("--target-new-bucket-placements: unknown placement ID %d", p)
		}
	}
	for _, p := range knownList {
		if _, ok := placements[p]; !ok {
			return errs.New("--known-placement-ids: unknown placement ID %d", p)
		}
	}
	for old, newer := range sunsetMap {
		if _, ok := placements[old]; !ok {
			return errs.New("--sunset-default-placements: unknown placement ID %d", old)
		}
		if _, ok := placements[newer]; !ok {
			return errs.New("--sunset-default-placements: unknown mapped placement ID %d", newer)
		}
	}
	for p, productID := range newPPM {
		if _, ok := placements[p]; !ok {
			return errs.New("--new-placement-product-map: unknown placement ID %d", p)
		}
		if _, ok := products[productID]; !ok {
			return errs.New("--new-placement-product-map: unknown product ID %d", productID)
		}
	}
	if phase == "billing" {
		if _, ok := products[fallbackProductID]; !ok {
			return errs.New("--fallback-product-id: unknown product ID %d", fallbackProductID)
		}
	}
	return nil
}

func cmdMigratePricing(cmd *cobra.Command, _ []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	if err := validateMigratePricingFlags(
		mpFlagPhase, mpFlagTargetNBP, mpFlagNewPPM, mpFlagKnownPlacements,
		cmd.Flags().Changed("fallback-product-id"),
	); err != nil {
		return err
	}

	targetNBP, err := parsePlacementList(mpFlagTargetNBP)
	if err != nil {
		return errs.New("--target-new-bucket-placements: %w", err)
	}

	sunsetMap, err := parsePlacementPairMap(mpFlagSunsetPlacements)
	if err != nil {
		return errs.New("--sunset-default-placements: %w", err)
	}

	newPlacementProductMappings, err := parsePlacementProductMap(mpFlagNewPPM)
	if err != nil {
		return errs.New("--new-placement-product-map: %w", err)
	}

	knownList, err := parsePlacementList(mpFlagKnownPlacements)
	if err != nil {
		return errs.New("--known-placement-ids: %w", err)
	}
	knownSet := placementSet(knownList)

	if err := validateMigratePricingFlagValues(
		mpFlagPhase, targetNBP, sunsetMap, newPlacementProductMappings, knownList,
		mpFlagFallbackProductID,
	); err != nil {
		return err
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

	entService := entitlements.NewService(log.Named("entitlements"), satDB.Console().Entitlements())

	return runMigratePricing(ctx, log, satDB, entService, migratePricingArgs{
		targetNewBucketPlacements:   targetNBP,
		sunsetMap:                   sunsetMap,
		newPlacementProductMappings: newPlacementProductMappings,
		knownSet:                    knownSet,
		fallbackProduct:             mpFlagFallbackProductID,
		phase:                       mpFlagPhase,
		dryRun:                      mpFlagDryRun,
	})
}

func runMigratePricing(ctx context.Context, log *zap.Logger, satDB satellite.DB, entService *entitlements.Service, args migratePricingArgs) error {
	const batchSize = 1000

	if args.phase == "billing" && len(args.newPlacementProductMappings) == 0 {
		return errs.New("--new-placement-product-map parsed to empty map")
	}

	var counts migratePricingCounts
	targetNBPSet := placementSet(args.targetNewBucketPlacements)

	offset := int64(0)
	batchNum := 0
	before := time.Now()

	for {
		page, err := satDB.Console().Projects().List(ctx, offset, batchSize, before)
		if err != nil {
			return errs.New("error listing projects: %+v", err)
		}

		batchNum++
		log.Info("processing batch", zap.Int("batch", batchNum), zap.Int("count", len(page.Projects)), zap.Int64("offset", offset))

		for _, project := range page.Projects {
			if project.Status == nil || *project.Status != console.ProjectActive {
				continue
			}
			if args.phase == "ui" {
				err = migratePricingPhase1(ctx, log, satDB, entService, project, args, targetNBPSet, &counts)
			} else {
				err = migratePricingPhase2(ctx, log, entService, project, args, &counts)
			}
			if err != nil {
				return err
			}
		}

		if !page.Next {
			break
		}
		offset = page.NextOffset
	}

	log.Info("migrate-pricing complete",
		zap.Int("updated", counts.updated),
		zap.Int("skipped", counts.skipped),
		zap.Int("no_change", counts.noChange),
		zap.Int("custom", counts.custom),
		zap.Bool("dry_run", args.dryRun),
	)
	return nil
}

func migratePricingPhase1(
	ctx context.Context,
	log *zap.Logger,
	satDB satellite.DB,
	entService *entitlements.Service,
	project console.Project,
	args migratePricingArgs,
	targetNBPSet map[storj.PlacementConstraint]struct{},
	counts *migratePricingCounts,
) error {
	feats, err := entService.Projects().GetByPublicID(ctx, project.PublicID)
	rowNotFound := entitlements.ErrNotFound.Has(err)
	if err != nil && !rowNotFound {
		return errs.New("error fetching entitlements for project %s: %+v", project.PublicID, err)
	}

	for _, p := range feats.NewBucketPlacements {
		if _, ok := args.knownSet[p]; !ok {
			log.Info("skipping custom project (Phase 1)",
				zap.String("project_id", project.PublicID.String()),
				zap.Any("placement", p),
			)
			counts.skipped++
			return nil
		}
	}

	// whether all the project's NewBucketPlacements are in the target set.
	isSubset := func() bool {
		for _, p := range feats.NewBucketPlacements {
			if _, ok := targetNBPSet[p]; !ok {
				return false
			}
		}
		return true
	}()

	// whether any of the project's NewBucketPlacements is being sunset.
	hasSunset := func() bool {
		for _, p := range feats.NewBucketPlacements {
			if _, ok := args.sunsetMap[p]; ok {
				return true
			}
		}
		return false
	}()

	updateNewBucketPlacements := !isSubset || hasSunset || rowNotFound

	if updateNewBucketPlacements {
		if args.dryRun {
			log.Info("dry-run: would update NewBucketPlacements",
				zap.String("project_id", project.PublicID.String()),
				zap.Any("old", feats.NewBucketPlacements),
				zap.Any("new", args.targetNewBucketPlacements),
			)
		} else {
			if err := entService.Projects().SetNewBucketPlacementsByPublicID(ctx, project.PublicID, args.targetNewBucketPlacements); err != nil {
				return errs.New("error updating NewBucketPlacements for project %s: %+v", project.PublicID, err)
			}
		}
	}

	// update DefaultPlacement if it being sunset.
	defaultPlacementUpdated := false
	if newDP, ok := args.sunsetMap[project.DefaultPlacement]; ok {
		defaultPlacementUpdated = true
		if args.dryRun {
			log.Info("dry-run: would update DefaultPlacement",
				zap.String("project_id", project.PublicID.String()),
				zap.Any("old", project.DefaultPlacement),
				zap.Any("new", newDP),
			)
		} else {
			if err := satDB.Console().Projects().UpdateDefaultPlacement(ctx, project.ID, newDP); err != nil {
				return errs.New("error updating DefaultPlacement for project %s: %+v", project.ID, err)
			}
		}
	}

	if updateNewBucketPlacements || defaultPlacementUpdated {
		counts.updated++
	} else {
		counts.noChange++
	}
	return nil
}

func migratePricingPhase2(
	ctx context.Context,
	log *zap.Logger,
	entService *entitlements.Service,
	project console.Project,
	args migratePricingArgs,
	counts *migratePricingCounts,
) error {
	feats, err := entService.Projects().GetByPublicID(ctx, project.PublicID)
	if err != nil && !entitlements.ErrNotFound.Has(err) {
		return errs.New("error fetching entitlements for project %s: %+v", project.PublicID, err)
	}

	// whether the project has a NewBucketPlacement that is not
	// a known placement.
	isCustom := func() bool {
		for _, p := range feats.NewBucketPlacements {
			if _, ok := args.knownSet[p]; !ok {
				return true
			}
		}
		return false
	}()

	if !isCustom {
		if args.dryRun {
			log.Info("dry-run: would replace PlacementProductMappings (standard)",
				zap.String("project_id", project.PublicID.String()),
				zap.Any("old", feats.PlacementProductMappings),
				zap.Any("new", args.newPlacementProductMappings),
			)
		} else {
			// replace PlacementProductMappings for project with custom placement.
			if err := entService.Projects().SetPlacementProductMappingsByPublicID(ctx, project.PublicID, args.newPlacementProductMappings); err != nil {
				return errs.New("error updating PlacementProductMappings for project %s: %+v", project.PublicID, err)
			}
		}
		counts.updated++
		return nil
	}

	// merge project's PlacementProductMappings with newPlacementProductMappings so
	// preserve the custom mappings, i.e.; put the known newPlacementProductMappings
	// into PlacementProductMappings.
	merged := make(entitlements.PlacementProductMappings)
	maps.Copy(merged, feats.PlacementProductMappings)
	for p, productID := range args.newPlacementProductMappings {
		if _, known := args.knownSet[p]; known {
			merged[p] = productID
		}
	}
	// add fallback product mapping for NewBucketPlacements that
	// were not already mapped.
	for _, p := range feats.NewBucketPlacements {
		if _, known := args.knownSet[p]; !known {
			if _, alreadyMapped := merged[p]; !alreadyMapped {
				merged[p] = args.fallbackProduct
			}
		}
	}

	if args.dryRun {
		log.Info("dry-run: would update PlacementProductMappings (custom)",
			zap.String("project_id", project.PublicID.String()),
			zap.Any("old", feats.PlacementProductMappings),
			zap.Any("new", merged),
		)
	} else {
		if err := entService.Projects().SetPlacementProductMappingsByPublicID(ctx, project.PublicID, merged); err != nil {
			return errs.New("error updating PlacementProductMappings for custom project %s: %+v", project.PublicID, err)
		}
	}
	counts.custom++
	return nil
}

// parsePlacementList parses a comma-separated list of placement IDs (e.g. 0,12).
func parsePlacementList(s string) ([]storj.PlacementConstraint, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	result := make([]storj.PlacementConstraint, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return nil, errs.New("invalid placement ID %q: %w", p, err)
		}
		result = append(result, storj.PlacementConstraint(n))
	}
	return result, nil
}

// parsePlacementPairMap parses "old:new" pairs (e.g. 30:0,31:12).
func parsePlacementPairMap(s string) (map[storj.PlacementConstraint]storj.PlacementConstraint, error) {
	result := make(map[storj.PlacementConstraint]storj.PlacementConstraint)
	if s == "" {
		return result, nil
	}
	for pair := range strings.SplitSeq(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, errs.New("invalid pair %q: expected old:new format", pair)
		}
		oldID, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
		if err != nil {
			return nil, errs.New("invalid placement ID %q: %w", parts[0], err)
		}
		newID, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 16)
		if err != nil {
			return nil, errs.New("invalid placement ID %q: %w", parts[1], err)
		}
		result[storj.PlacementConstraint(oldID)] = storj.PlacementConstraint(newID)
	}
	return result, nil
}

// parsePlacementProductMap parses "placement:productID" pairs (e.g. 0:20,12:21).
func parsePlacementProductMap(s string) (entitlements.PlacementProductMappings, error) {
	result := make(entitlements.PlacementProductMappings)
	if s == "" {
		return result, nil
	}
	for pair := range strings.SplitSeq(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, errs.New("invalid pair %q: expected placement:productID format", pair)
		}
		placementID, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 16)
		if err != nil {
			return nil, errs.New("invalid placement ID %q: %w", parts[0], err)
		}
		productID, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 32)
		if err != nil {
			return nil, errs.New("invalid product ID %q: %w", parts[1], err)
		}
		result[storj.PlacementConstraint(placementID)] = int32(productID)
	}
	return result, nil
}

func placementSet(ps []storj.PlacementConstraint) map[storj.PlacementConstraint]struct{} {
	s := make(map[storj.PlacementConstraint]struct{}, len(ps))
	for _, p := range ps {
		s[p] = struct{}{}
	}
	return s
}
