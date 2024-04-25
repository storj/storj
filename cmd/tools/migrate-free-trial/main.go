// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	pgx "github.com/jackc/pgx/v5"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/cockroachutil"
	"storj.io/storj/shared/dbutil/pgutil"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "migrate-free-trial",
		Short: "migrate-free-trial",
	}

	runCmd = &cobra.Command{
		Use:   "run <trial-expiration> <notification-count>",
		Short: "run migrate-free-trial",
		Args:  cobra.ExactArgs(2),
		RunE:  run,
	}

	config Config
)

func init() {
	rootCmd.AddCommand(runCmd)

	config.BindFlags(runCmd.Flags())
}

// Config defines configuration for migration.
type Config struct {
	SatelliteDB string
	Limit       int
	MaxUpdates  int
	DryRun      bool
	Verbose     bool
}

// BindFlags adds bench flags to the the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteDB, "satellitedb", "", "connection URL for satelliteDB")
	flag.IntVar(&config.Limit, "limit", 1000, "number of updates to perform at once")
	flag.IntVar(&config.MaxUpdates, "max-updates", 0, "max number of updates to perform")
	flag.BoolVar(&config.DryRun, "dry-run", false, "return selected users for update without updating")
	flag.BoolVar(&config.Verbose, "verbose", false, "print ids to be updated")
}

// VerifyFlags verifies whether the values provided are valid.
func (config *Config) VerifyFlags() error {
	var errlist errs.Group
	if config.SatelliteDB == "" {
		errlist.Add(errors.New("flag '--satellitedb' is not set"))
	}
	return errlist.Err()
}

func run(cmd *cobra.Command, args []string) error {
	trialExpiration, err := time.Parse(time.RFC3339, args[0])
	if err != nil {
		return errs.New("error parsing trial-expiration argument: %w", err)
	}

	notificationCount, err := strconv.Atoi(args[1])
	if err != nil {
		return errs.New("error parsing notification-count: %w", err)
	}

	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	conn, err := pgx.Connect(ctx, config.SatelliteDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.SatelliteDB, err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	if config.MaxUpdates > 0 {
		return MigrateLimited(ctx, log, conn, trialExpiration, notificationCount, config)
	}
	return Migrate(ctx, log, conn, trialExpiration, notificationCount, config)
}

func main() {
	process.Exec(rootCmd)
}

// Migrate sets trial_expiration for free tier users where trial_expiration is null.
func Migrate(ctx context.Context, log *zap.Logger, conn *pgx.Conn, trialExpiration time.Time, notificationCount int, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	if config.DryRun {
		log.Info("executing dry run. No updates will be performed")
	}

	lastID := []byte{}
	var total int
	var rowsFound bool

	for {
		rowsFound = false
		ids := [][]byte{}

		err = func() error {
			rows, err := conn.Query(ctx, `
				SELECT id FROM users
				WHERE id > $1
					AND paid_tier = false
					AND trial_expiration IS NULL
				ORDER BY id
				LIMIT $2;
			`, lastID, config.Limit)
			if err != nil {
				return errs.New("error selecting IDs: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				rowsFound = true
				var id []byte
				err = rows.Scan(&id)
				if err != nil {
					return errs.New("error scanning results from select: %w", err)
				}
				ids = append(ids, id)
				lastID = id
			}

			return rows.Err()
		}()
		if err != nil {
			return err
		}
		if !rowsFound {
			break
		}

		uuids := []string{}
		if config.Verbose {
			for _, id := range ids {
				_uuid, err := uuid.FromBytes(id)
				if err != nil {
					return errs.New("error logging IDs: %w", err)
				}
				uuids = append(uuids, _uuid.String())
			}
		}

		fields := []zap.Field{zap.Int("count", len(ids))}
		if len(uuids) > 0 {
			fields = append(fields, zap.Strings("IDs", uuids))
		}

		log.Debug("selected users for update", fields...)

		if config.DryRun {
			continue
		}

		var updated int
		for {
			row := conn.QueryRow(ctx, `
				WITH updated as (
					UPDATE users
					SET trial_expiration = $2,
						trial_notifications = $3
					WHERE users.id IN (SELECT unnest($1::bytea[]))
					RETURNING 1
				)
				SELECT count(*)
				FROM updated;
			`, pgutil.ByteaArray(ids), trialExpiration, notificationCount,
			)
			err := row.Scan(&updated)
			if err != nil {
				if cockroachutil.NeedsRetry(err) {
					continue
				} else if errs.Is(err, pgx.ErrNoRows) {
					break
				}
				return errs.New("error updating users: %w", err)
			}
			break
		}
		total += updated
		log.Info("batch update complete", zap.Int("rows updated", updated), zap.Binary("last id", lastID))
	}
	log.Info("free tier trial expiration migration complete", zap.Int("total rows updated", total))
	return nil
}

// MigrateLimited sets trial_expiration for a limited number of free tier users where trial_expiration is null.
func MigrateLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, trialExpiration time.Time, notificationCount int, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	if config.DryRun {
		log.Info("executing dry run. No updates will be performed")
	}

	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id from users
			WHERE paid_tier = false
				AND trial_expiration IS NULL
			LIMIT $1;
		`, config.MaxUpdates)
		if err != nil {
			return nil, errs.New("selecting ids for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id []byte
			err = rows.Scan(&id)
			if err != nil {
				return nil, errs.New("error scanning results from select: %w", err)
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return err
	}

	uuids := []string{}
	if config.Verbose {
		for _, id := range selected {
			_uuid, err := uuid.FromBytes(id)
			if err != nil {
				return errs.New("error logging IDs: %w", err)
			}
			uuids = append(uuids, _uuid.String())
		}
	}

	fields := []zap.Field{zap.Int("count", len(selected))}
	if len(uuids) > 0 {
		fields = append(fields, zap.Strings("IDs", uuids))
	}

	log.Debug("selected users for update", fields...)

	if config.DryRun {
		return nil
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE users
			SET trial_expiration = $2,
				trial_notifications = $3
			WHERE users.id IN (SELECT unnest($1::bytea[]))
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
	`, pgutil.ByteaArray(selected), trialExpiration, notificationCount,
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}
