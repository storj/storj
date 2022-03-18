// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"

	pgx "github.com/jackc/pgx/v4"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/private/process"
	"storj.io/storj/satellite/rewards"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "partnerid-to-useragent-migration",
		Short: "partnerid-to-useragent-migration",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run partnerid-to-useragent-migration",
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
}

// BindFlags adds bench flags to the the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteDB, "satellitedb", "", "connection URL for satelliteDB")
	flag.IntVar(&config.Limit, "limit", 1000, "number of updates to perform at once")
	flag.IntVar(&config.MaxUpdates, "max-updates", 0, "max number of updates to perform on each table")
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
	return Migrate(ctx, log, config)
}

func main() {
	process.Exec(rootCmd)
}

// Partners holds slices of partner UUIDs and Names for easy insertion into SQL UNNEST().
type Partners struct {
	UUIDs []uuid.UUID
	Names [][]byte
}

// Migrate updates the user_agent column if partner_id != NULL AND user_agent IS NULL.
// If the partner_id matches a PartnerInfo.UUID in the partnerDB, user_agent will be
// set to PartnerInfo.Name. Otherwise, user_agent will be set to partner_id. If
// Config.MaxUpdates > 0, only that number of rows will be updated.
// Affected tables:
//
// users
// projects
// api_keys
// bucket_metainfos
// value_attributions.
func Migrate(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := pgx.Connect(ctx, config.SatelliteDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.SatelliteDB, err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	partnerDB := rewards.DefaultPartnersDB
	partnerInfo, err := partnerDB.All(ctx)
	if err != nil {
		return errs.New("could not get partners list: %w", err)
	}

	var p Partners
	for _, info := range partnerInfo {
		p.UUIDs = append(p.UUIDs, info.UUID)
		p.Names = append(p.Names, []byte(info.Name))
	}

	// The original migrations are already somewhat complex and in my opinion,
	// trying to edit them to be able to handle conditionally limiting updates increased the
	// complexity. While I think splitting out the limited update migrations isn't the
	// most ideal solution, since this code is temporary we don't need to worry about
	// maintenance concerns with having multiple queries.
	if config.MaxUpdates > 1000 {
		return errs.New("When running limited migration, set --max-updates to something less than 1000")
	}
	if config.MaxUpdates > 0 {
		return MigrateTablesLimited(ctx, log, conn, &p, config)
	}

	return MigrateTables(ctx, log, conn, &p, config)
}
