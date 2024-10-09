// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"

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
		Use:   "migrate-token-payers",
		Short: "migrate-token-payers",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run migrate-token-payers",
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
	DryRun      bool
	Verbose     bool
}

// BindFlags adds bench flags to the the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteDB, "satellitedb", "", "connection URL for satelliteDB")
	flag.IntVar(&config.Limit, "limit", 1000, "number of updates to perform")
	flag.BoolVar(&config.DryRun, "dry-run", false, "return selected users for update without updating")
	flag.BoolVar(&config.Verbose, "verbose", false, "print selected user IDs for update")
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

	return Migrate(ctx, log, conn, config)
}

func main() {
	process.Exec(rootCmd)
}

// Migrate updates users that have deposited storj tokens to paid tier.
func Migrate(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	if config.DryRun {
		log.Info("executing dry run. No updates will be performed")
	}

	rows, err := conn.Query(ctx, `
		SELECT id FROM users
		WHERE id IN (
			SELECT DISTINCT(COALESCE(bb.user_id, ct.user_id)) from billing_balances bb
			FULL JOIN (
			SELECT DISTINCT(user_id) FROM coinpayments_transactions
				WHERE status = 100
			) ct on bb.user_id = ct.user_id
		)
		AND status = 1
		AND paid_tier = false
		LIMIT $1;
	`, config.Limit)
	if err != nil {
		return errs.New("error selecting IDs: %w", err)
	}
	defer rows.Close()

	ids := [][]byte{}
	for rows.Next() {
		var id []byte
		err = rows.Scan(&id)
		if err != nil {
			return errs.New("error scanning results from select: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return errs.New("rows error: %w", err)
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
		return nil
	}

	var updated int
	for {
		row := conn.QueryRow(ctx, `
			WITH to_update AS (
				SELECT unnest($1::bytea[]) as id
			),
			updated as (
				UPDATE users
				SET paid_tier = true
				FROM to_update
				WHERE users.id = to_update.id
				RETURNING 1
			)
			SELECT count(*)
			FROM updated;
		`, pgutil.ByteaArray(ids))
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

	log.Info("token payers migration iteration complete", zap.Int("total rows updated", updated))
	return nil
}
