// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/private/process"
	"storj.io/storj/satellite/metabase"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "metainfo-expiresat-migration",
		Short: "metainfo-expiresat-migration",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run metainfo-expiresat-migration",
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
	MetabaseDB      string
	LoopBatchSize   int
	UpdateBatchSize int
	Cockroach       bool
}

// BindFlags adds bench flags to the the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.MetabaseDB, "metabasedb", "", "connection URL for MetabaseDB")
	flag.IntVar(&config.LoopBatchSize, "loop-batch-size", 10000, "number of objects to process at once")
	flag.IntVar(&config.UpdateBatchSize, "update-batch-size", 100, "number of update requests in a single batch call")
	flag.BoolVar(&config.Cockroach, "cockroach", true, "metabase is on CRDB")
}

// VerifyFlags verifies whether the values provided are valid.
func (config *Config) VerifyFlags() error {
	var errlist errs.Group
	if config.MetabaseDB == "" {
		errlist.Add(errors.New("flag '--metabasedb' is not set"))
	}
	return errlist.Err()
}

type update struct {
	StreamID  uuid.UUID
	ExpiresAt time.Time
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

// Migrate migrates created_at from object to corresponding segment if value is missing there.
func Migrate(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	poolConfig, err := pgxpool.ParseConfig(config.MetabaseDB)
	if err != nil {
		return err
	}
	poolConfig.MaxConns = 10

	rawMetabaseDB, err := pgxpool.ConnectConfig(ctx, poolConfig)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.MetabaseDB, err)
	}
	defer func() { rawMetabaseDB.Close() }()

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), config.MetabaseDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.MetabaseDB, err)
	}
	defer func() { err = errs.Combine(err, metabaseDB.Close()) }()

	startingTime, err := metabaseDB.Now(ctx)
	if err != nil {
		return err
	}

	limiter := sync2.NewLimiter(10)

	updates := make([]update, 0, config.UpdateBatchSize)
	numberOfObjects := 0
	return metabaseDB.IterateLoopObjects(ctx, metabase.IterateLoopObjects{
		BatchSize:      config.LoopBatchSize,
		AsOfSystemTime: startingTime,
	}, func(ctx context.Context, it metabase.LoopObjectsIterator) error {
		defer mon.TaskNamed("migrateExpiresAtLO")(&ctx, "objs", numberOfObjects)(&err)

		var entry metabase.LoopObjectEntry
		for it.Next(ctx, &entry) {
			if entry.ExpiresAt != nil {
				updates = append(updates, update{
					StreamID:  entry.StreamID,
					ExpiresAt: *entry.ExpiresAt,
				})

				if len(updates) == config.UpdateBatchSize {
					toSend := make([]update, len(updates))
					copy(toSend, updates)
					limiter.Go(ctx, func() {
						sendUpdates(ctx, log, rawMetabaseDB, toSend)
					})
					updates = updates[:0]
				}
			}

			if numberOfObjects%100000 == 0 {
				log.Info("updated", zap.Int("objects", numberOfObjects))
			}

			numberOfObjects++
		}

		limiter.Wait()

		sendUpdates(ctx, log, rawMetabaseDB, updates)

		log.Info("finished", zap.Int("objects", numberOfObjects))

		return nil
	})
}

func sendUpdates(ctx context.Context, log *zap.Logger, conn *pgxpool.Pool, updates []update) {
	defer mon.Task()(&ctx)(nil)

	if len(updates) == 0 {
		return
	}

	batch := &pgx.Batch{}
	for _, update := range updates {
		batch.Queue("UPDATE segments SET expires_at = $2 WHERE stream_id = $1 AND expires_at IS NULL;", update.StreamID, update.ExpiresAt)
	}

	br := conn.SendBatch(ctx, batch)
	for _, update := range updates {
		_, err := br.Exec()
		if err != nil {
			log.Error("error during updating segment", zap.String("StreamID", update.StreamID.String()), zap.Error(err))
		}
	}

	if err := br.Close(); err != nil {
		log.Error("error during closing batch result", zap.Error(err))
	}
}
