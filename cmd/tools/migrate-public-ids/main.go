// Copyright (C) 2021 Storj Labs, Inc.
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
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "migrate-public-ids",
		Short: "migrate-public-ids",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run migrate-public-ids",
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

// BindFlags adds bench flags to the flagset.
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
	logger, _, _ := process.NewLogger("migrate-public-ids")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}

// Migrate updates projects with a new public_id where public_id is null.
func Migrate(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := pgx.Connect(ctx, config.SatelliteDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.SatelliteDB, err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	return MigrateProjects(ctx, log, conn, config)
}
