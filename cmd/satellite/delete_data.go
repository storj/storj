// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/process"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/satellitedb"
)

func cmdDeleteObjects(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	satDB, err := satellitedb.Open(ctx, log.Named("db"), runCfg.Database, satellitedb.Options{
		ApplicationName:      "satellite-delete-data",
		APIKeysLRUOptions:    runCfg.APIKeysLRUOptions(),
		RevocationLRUOptions: runCfg.RevocationLRUOptions(),
	})
	if err != nil {
		return errs.New("Error connecting to satellite database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, satDB.Close())
	}()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), runCfg.Metainfo.DatabaseURL, runCfg.Metainfo.Metabase("satellite-rangedloop"))
	if err != nil {
		return errs.New("Error creating metabase connection: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, metabaseDB.Close())
	}()

	csvFile, err := os.Open(args[0])
	if err != nil {
		return errs.New("error opening CSV file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, csvFile.Close())
	}()

	return deleteObjects(ctx, log, satDB, metabaseDB, csvFile)
}

func cmdDeleteAccounts(_ *cobra.Command, _ []string) error {
	// TODO: implement it
	panic("not implemented")
}

// deleteObjects for each user's account with the email in csvData, delete all the objects and
// buckets.
//
// Accounts must be in "pending deletion" status to be processed. An info message is logged when
// an account doesn't have that status.
//
// It returns an error when a system error is found or when the CSV file has an error.
func deleteObjects(
	ctx context.Context, log *zap.Logger, satDB satellite.DB, metabaseDB *metabase.DB, csvFile io.Reader,
) error {
	firstRow := true
	csvReader := csv.NewReader(csvFile)
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
		user, err := satDB.Console().Users().GetByEmail(ctx, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Debug("skipping not found user's account", zap.String("email", email))
				continue
			}
			return errs.New("error getting user %q: %+v", email, err)
		}

		if user.Status != console.PendingDeletion {
			log.Debug("skipping not pending deletion user's account", zap.String("email", email))
			continue
		}

		projects, err := satDB.Console().Projects().GetOwn(ctx, user.ID)
		if err != nil {
			return errs.New("error getting project from user %q: %+v", email, err)
		}

		for _, p := range projects {
			var (
				blopts = buckets.ListOptions{
					Direction: buckets.DirectionForward,
				}
				bcks = buckets.List{
					More: true, // Allows to start the first iteration of the loop of buckets.
				}
			)

			for bcks.More {
				bcks, err = satDB.Buckets().ListBuckets(ctx,
					p.ID, blopts, macaroon.AllowedBuckets{All: true},
				)
				if err != nil {
					return errs.New(
						"error listing buckets for project %q (user: %q): %+v", p.Name, email, err,
					)
				}

				for _, b := range bcks.Items {
					_, err := metabaseDB.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
						Bucket: metabase.BucketLocation{
							ProjectID:  p.ID,
							BucketName: metabase.BucketName(b.Name),
						},
					})
					if err != nil {
						return errs.New(
							"error deleting all objects from bucket %q (project: %q, user: %q): %+v",
							b.Name, p.Name, email, err,
						)
					}

					if err := satDB.Buckets().DeleteBucket(ctx, []byte(b.Name), p.ID); err != nil {
						return errs.New(
							"error deleting bucket %q (project: %q, user: %q): %+v",
							b.Name, p.Name, email, err,
						)
					}
				}

				blopts = blopts.NextPage(bcks)
			}
		}
	}

	return nil
}
