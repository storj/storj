// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/process"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/stripe"
	"storj.io/storj/satellite/satellitedb"
)

func cmdDeleteObjects(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-users",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, runCfg.Metainfo.Metabase("satellite-rangedloop"))
	if err != nil {
		return errs.New("error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	var csvData io.Reader
	if strings.Contains(args[0], "@") {
		csvData = strings.NewReader(args[0])
	} else {
		csvFile, err := os.Open(args[0])
		if err != nil {
			return errs.New("error opening CSV file: %+v", err)
		}
		defer func() {
			err = errs.Combine(err, csvFile.Close())
		}()

		csvData = csvFile
	}

	return deleteObjects(ctx, log, satDB, metabaseDB, batchSizeDeleteObjects, csvData)
}

func cmdDeleteAccounts(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-users",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	var csvData io.Reader
	if strings.Contains(args[0], "@") {
		csvData = strings.NewReader(args[0])
	} else {
		csvFile, err := os.Open(args[0])
		if err != nil {
			return errs.New("error opening CSV file: %+v", err)
		}
		defer func() {
			err = errs.Combine(err, csvFile.Close())
		}()

		csvData = csvFile
	}

	stripeService, err := setupPayments(log, satDB)
	if err != nil {
		return errs.New("error setting up Stripe service: %+v", err)
	}

	return deleteAccounts(
		ctx, log, satDB, stripeService.Accounts().Invoices(), csvData,
	)
}

func cmdSetAccountsStatusPendingDeletion(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-users",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	var csvData io.Reader
	if strings.Contains(args[0], "@") {
		csvData = strings.NewReader(args[0])
	} else {
		csvFile, err := os.Open(args[0])
		if err != nil {
			return errs.New("error opening CSV file: %+v", err)
		}
		defer func() {
			err = errs.Combine(err, csvFile.Close())
		}()

		csvData = csvFile
	}

	// Truncate the duration to days.
	defaultDaysTillEscalation := uint(
		runCfg.Console.AccountFreeze.TrialExpirationFreezeGracePeriod.Hours() / 24,
	)
	return setAccountsStatusPendingDeletion(ctx, log, satDB, defaultDaysTillEscalation, csvData)
}

// deleteObjects for each user's account with the email in csvData, delete all the objects and
// buckets.
//
// Accounts must exists and be in "pending deletion" status to be processed. A debug message is
// logged when the account doesn't exist or is "inactive" status. An info message is logged when an
// account isn't in "pending deletion" status.
//
// It returns an error when a system error is found or when the CSV file has an error.
func deleteObjects(
	ctx context.Context, log *zap.Logger, satDB satellite.DB, metabaseDB *metabase.DB, batchSize int,
	csvData io.Reader,
) error {
	rows := CSVEmails{
		Data: csvData,
		Log:  log,
		DB:   satDB.Console(),
	}

	return rows.ForEachWithProjects(ctx, func(user *console.User, projects []console.Project) error {
		if user.Status != console.PendingDeletion {
			log.Info("skipping not pending deletion user's account", zap.String("email", user.Email))
			return nil
		}

		for _, p := range projects {
			err := deleteAllBucketsAndObjects(ctx, satDB.Buckets(), metabaseDB, p.ID, batchSize)
			if err != nil {
				return errs.New("error deleting objects (%q): %w", user.Email, err)
			}
		}

		return nil
	})
}

// deleteAccounts for each user's account with the email in csvData, redacts the user's personal
// information and mark the account as deleted, marks their associated projects as disabled, and
// delete their API keys.
//
// The user's accounts that they fulfill the following requirements are skipped and logged with an
// error message:
//
//   - The account must exists. Accounts in "inactive" status are returned as not exists. An info
//     message is logged when it doesn't exist.
//
//   - The account must be in "pending deletion" status.
//
//   - If the account is in the paid tier:
//
//   - All the invoices must not be in "open" or "draft" status.
//
//   - There are no pending invoice items.
//
//   - All the projects must not have any usage in the current month.
//
//   - All the projects must not have any usage in the last month without a created invoice.
//
//   - Its projects must not have any buckets.
//
// It returns an error when a system error is found or when the CSV file has an error.
func deleteAccounts(
	ctx context.Context, log *zap.Logger, satDB satellite.DB, invoices payments.Invoices,
	csvData io.Reader,
) error {
	var (
		projectAccounting = satDB.ProjectAccounting()
		projectRecords    = satDB.StripeCoinPayments().ProjectRecords()
	)

	rows := CSVEmails{
		Data: csvData,
		Log:  log,
		DB:   satDB.Console(),
	}

	return rows.ForEachWithProjects(ctx, func(user *console.User, projects []console.Project) error {
		if user.Status != console.PendingDeletion {
			log.Info("skipping not pending deletion user's account", zap.String("email", user.Email))
			return nil
		}

		if user.PaidTier {
			{ // Check if the user has pending invoices.
				list, err := invoices.List(ctx, user.ID)
				if err != nil {
					return errs.New(
						"error listing invoices for user %q: %+v", user.Email, err,
					)
				}

				for _, inv := range list {
					if inv.Status == payments.InvoiceStatusOpen || inv.Status == payments.InvoiceStatusDraft {
						log.Error(
							"cannot mark as deleted the account because it has pending invoices ('open' or 'draft')",
							zap.String("user_email", user.Email),
						)
						return nil
					}
				}
			}

			// Check if the user has pending invoice items.
			hasPendingItems, err := invoices.CheckPendingItems(ctx, user.ID)
			if err != nil {
				return errs.New(
					"error checking pending invoice items for user %q: %+v", user.Email, err,
				)
			}
			if hasPendingItems {
				log.Error(
					"cannot mark as deleted the account because it has pending invoice items",
					zap.String("user_email", user.Email),
				)
				return nil
			}

			now := time.Now().UTC()
			year, month, _ := now.Date()
			firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
			for _, p := range projects {
				// Check if there is usage this month.
				usage, err := projectAccounting.GetProjectTotal(ctx, p.ID, firstOfMonth, now)
				if err != nil {
					return errs.New(
						"error getting usage for project %q (user: %q): %+v", p.ID, user.Email, err,
					)
				}

				if usage.Storage > 0 || usage.Egress > 0 || usage.SegmentCount > 0 {
					log.Error(
						"cannot mark as deleted the account because the project has usage",
						zap.String("user_email", user.Email),
						zap.String("project_id", p.ID.String()),
					)
					return nil
				}

				// Check usage for last month, if exists, ensure we have an invoice item created.
				usage, err = projectAccounting.GetProjectTotal(
					ctx, p.ID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1),
				)
				if err != nil {
					return errs.New("error getting usage for project %q (user: %q): %+v", p.ID, user.Email, err)
				}

				if usage.Storage > 0 || usage.Egress > 0 || usage.SegmentCount > 0 {
					err = projectRecords.Check(ctx, p.ID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
					if !errors.Is(err, stripe.ErrProjectRecordExists) {
						if err != nil {
							return errs.New(
								"error checking project record for project %q (user: %q): %+v", p.ID, user.Email, err,
							)
						}

						log.Error(
							"cannot mark as deleted the account because the project has usage last month and not invoiced yet",
							zap.String("user_email", user.Email),
							zap.String("project_id", p.ID.String()),
						)
						return nil
					}
				}
			}
		}

		for _, p := range projects {
			bcks, err := satDB.Buckets().ListBuckets(ctx,
				p.ID, buckets.ListOptions{
					Direction: buckets.DirectionForward,
					Limit:     1,
				}, macaroon.AllowedBuckets{All: true})
			if err != nil {
				return errs.New(
					"error listing buckets for project %q (user: %q): %+v", p.ID, user.Email, err,
				)
			}
			if len(bcks.Items) > 0 {
				log.Error(
					"cannot mark as deleted the account because the project has buckets",
					zap.String("email", user.Email),
					zap.String("project", p.ID.String()),
				)
				return nil
			}

			if err := satDB.Console().APIKeys().DeleteAllByProjectID(ctx, p.ID); err != nil {
				return errs.New(
					"error deleting API keys for project %q (user: %q): %+v", p.ID, user.Email, err,
				)
			}

			if err := satDB.Console().Projects().UpdateStatus(ctx, p.ID, console.ProjectDisabled); err != nil {
				return errs.New(
					"error updating project status %q (user: %q) to 'disabled': %+v", p.ID, user.Email, err,
				)
			}
		}

		emptyName := ""
		emptyNamePtr := &emptyName
		deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", user.ID.String())
		status := console.Deleted

		err := satDB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
			FullName:  &emptyName,
			ShortName: &emptyNamePtr,
			Email:     &deactivatedEmail,
			Status:    &status,
		})
		if err != nil {
			return errs.New(
				"error updating user %q to redact its personal data and set it to 'deleted' status: %+v",
				user.Email, err,
			)
		}

		return nil
	})
}

// setAccountsStatusPendingDeletion sets accounts to "pending deletion" status, but only if:
// 1. The account is currently in "active" status
// 2. The account is NOT in the paid tier
// 3. The account has an active "trial expiration freeze"
// 4. The active "trial expiration freeze" is over
// 5. The account is NOT a member of a third party project
//
// Any accounts that don't meet these criteria are logged and skipped.
//
// It returns an error when a system error is found or when the CSV file has an error.
func setAccountsStatusPendingDeletion(
	ctx context.Context, log *zap.Logger, satDB satellite.DB, defaultDaysTillEscalation uint, csvData io.Reader,
) error {
	rows := CSVEmails{
		Data: csvData,
		Log:  log,
		DB:   satDB.Console(),
	}

	return rows.ForEach(ctx, func(user *console.User) error {
		err := satDB.Console().Users().SetStatusPendingDeletion(ctx, user.ID, defaultDaysTillEscalation)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				return errs.New("error updating status to 'pending deletion' for user %q: %+v", user.Email, err)
			}

			log.Info(
				"skipping account it doesn't fulfill requirements to be set to 'pending deletion' status",
				zap.String("email", user.Email),
			)
		}

		return nil
	})
}

// CSVEmails is a CSV file with user's emails.
// If the first rown doesn't contain `@`, then its is considered a header and skipped.
//
// See the ForEach method for more details.
type CSVEmails struct {
	Data io.Reader
	Log  *zap.Logger
	DB   console.DB
}

// ForEach gets the user for each email in the CSV and call fn.
//
// It skips the emails which don't match any user's account or have "inactive" status, and logs a
// debug message.
//
// It returns an error if there is an error in the CSV, retrieving the user or projects, a user's
// account doesn't have the UserStatus, or the fn returns an error.
func (ce *CSVEmails) ForEach(ctx context.Context, fn func(user *console.User) error) error {
	firstRow := true
	csvReader := csv.NewReader(ce.Data)
	for {
		record, err := csvReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return errs.New("error reading CSV file: %+v", err)
		}

		email := record[0]
		if firstRow {
			firstRow = false
			// First row is the header. Skip it.
			if !strings.Contains(email, "@") {
				continue
			}
		}

		// TODO: User a method that returns the users in "inactive" status.
		user, err := ce.DB.Users().GetByEmail(ctx, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				ce.Log.Debug("skipping not found or 'inactive' user's account", zap.String("email", email))
				continue
			}
			return errs.New("error getting user %q: %+v", email, err)
		}

		if err := fn(user); err != nil {
			return err
		}
	}

	return nil
}

// ForEachWithProjects gets the user and its projects for each email in the CSV and call fn.
//
// It skips the emails which don't match any user's account and logs a debug message.
//
// It returns an error if there is an error in the CSV, retrieving the user or projects, a user's
// account doesn't have the UserStatus, or the fn returns an error.
func (ce *CSVEmails) ForEachWithProjects(
	ctx context.Context, fn func(user *console.User, projects []console.Project) error) error {

	return ce.ForEach(ctx, func(user *console.User) error {
		projects, err := ce.DB.Projects().GetOwn(ctx, user.ID)
		if err != nil {
			return errs.New("error getting project from user %q: %+v", user.Email, err)
		}

		if err := fn(user, projects); err != nil {
			return err
		}

		return nil

	})
}

func deleteAllBucketsAndObjects(
	ctx context.Context, bucketsDB buckets.DB, metabaseDB *metabase.DB, projectID uuid.UUID,
	batchSize int,
) error {
	var (
		blopts = buckets.ListOptions{
			Direction: buckets.DirectionForward,
		}
		bcks = buckets.List{
			More: true, // Allows to start the first iteration of the loop of buckets.
		}
		err error
	)

	for bcks.More {
		bcks, err = bucketsDB.ListBuckets(ctx,
			projectID, blopts, macaroon.AllowedBuckets{All: true},
		)
		if err != nil {
			return errs.New("error listing buckets for project %q: %+v", projectID, err)
		}

		for _, b := range bcks.Items {
			_, err := metabaseDB.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: metabase.BucketLocation{
					ProjectID:  projectID,
					BucketName: metabase.BucketName(b.Name),
				},
				BatchSize: batchSize,
			})
			if err != nil {
				return errs.New(
					"error deleting all objects from bucket %q (project: %q): %+v",
					b.Name, projectID, err,
				)
			}

			if err := bucketsDB.DeleteBucket(ctx, []byte(b.Name), projectID); err != nil {
				return errs.New("error deleting bucket %q (project: %q): %+v", b.Name, projectID, err)
			}
		}

		blopts = blopts.NextPage(bcks)
	}

	return nil
}
