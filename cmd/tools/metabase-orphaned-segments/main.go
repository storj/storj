// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/dbutil/pgutil"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "metainfo-orphaned-segments",
		Short: "metainfo-orphaned-segments",
	}

	reportCmd = &cobra.Command{
		Use:   "report",
		Short: "report metainfo-orphaned-segments",
		RunE:  reportCommand,
	}

	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "delete metainfo-orphaned-segments",
		RunE:  deleteCommand,
	}

	config Config
)

func init() {
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(deleteCmd)

	config.BindFlags(reportCmd.Flags())
	config.BindFlags(deleteCmd.Flags())
}

// Config defines configuration for cleanup.
type Config struct {
	MetabaseDB      string
	LoopBatchSize   int
	DeleteBatchSize int
	Cockroach       bool
}

// BindFlags adds bench flags to the the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.MetabaseDB, "metabasedb", "", "connection URL for MetabaseDB")
	flag.IntVar(&config.LoopBatchSize, "loop-batch-size", 10000, "number of objects to process at once")
	flag.IntVar(&config.DeleteBatchSize, "delete-batch-size", 100, "number of entries to delete with single query")
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

func reportCommand(cmd *cobra.Command, args []string) error {
	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	return Report(ctx, log, config)
}

func deleteCommand(cmd *cobra.Command, args []string) error {
	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	return Delete(ctx, log, config)
}

func main() {
	process.Exec(rootCmd)
}

// Report finds and reports orphaned segments, nothing is deleted.
func Report(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	ids, err := findOrphanedSegments(ctx, log, config)
	if err != nil {
		return err
	}
	log.Info("orphaned segments stream ids (number of existing segments can be bigger)",
		zap.Int("count", len(ids)))
	for _, id := range ids {
		log.Info("StreamID", zap.String("id", id.String()))
	}
	return nil
}

// Delete finds and deletes orphaned segments.
func Delete(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	ids, err := findOrphanedSegments(ctx, log, config)
	if err != nil {
		return err
	}

	rawMetabaseDB, err := pgx.Connect(ctx, config.MetabaseDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.MetabaseDB, err)
	}
	defer func() { err = errs.Combine(err, rawMetabaseDB.Close(ctx)) }()

	log.Info("orphaned segments stream ids (number of existing segments can be bigger)", zap.Int("count", len(ids)))
	log.Info("starting deletion")

	for len(ids) > 0 {
		batch := config.DeleteBatchSize
		if batch > len(ids) {
			batch = len(ids)
		}
		err = sendDelete(ctx, rawMetabaseDB, ids[:batch])
		if err != nil {
			return err
		}
		ids = ids[batch:]

		log.Info("deleted", zap.Int("count", batch))
	}
	return nil
}

func findOrphanedSegments(ctx context.Context, log *zap.Logger, config Config) (_ []uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)

	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), config.MetabaseDB, metabase.Config{ApplicationName: "metabase-orphaned-segments"})
	if err != nil {
		return nil, errs.New("unable to connect %q: %w", config.MetabaseDB, err)
	}
	defer func() { err = errs.Combine(err, metabaseDB.Close()) }()

	// NOTE: this is the current time according to the first database adapter,
	// not necessarily the current time according to others. This mismatch is
	// probably acceptable for this tool.
	startingTime, err := metabaseDB.Now(ctx)
	if err != nil {
		return nil, err
	}

	streamIDs := make(map[uuid.UUID]struct{})
	numberOfElements := 0
	err = metabaseDB.IterateLoopSegments(ctx, metabase.IterateLoopSegments{
		BatchSize: config.LoopBatchSize,
	}, func(ctx context.Context, it metabase.LoopSegmentsIterator) error {
		var entry metabase.LoopSegmentEntry
		for it.Next(ctx, &entry) {
			// avoid segments created after starting processing
			if entry.CreatedAt.After(startingTime) {
				continue
			}

			streamIDs[entry.StreamID] = struct{}{}

			if numberOfElements%100000 == 0 {
				log.Info("segments iterated", zap.Int("segments", numberOfElements))
			}

			numberOfElements++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	numberOfElements = 0
	err = metabaseDB.IterateLoopObjects(ctx, metabase.IterateLoopObjects{
		BatchSize:      config.LoopBatchSize,
		AsOfSystemTime: startingTime,
	}, func(ctx context.Context, it metabase.LoopObjectsIterator) error {
		var entry metabase.LoopObjectEntry
		for it.Next(ctx, &entry) {
			// avoid objects created after starting processing
			if entry.CreatedAt.After(startingTime) {
				continue
			}

			delete(streamIDs, entry.StreamID)

			if numberOfElements%100000 == 0 {
				log.Info("objects iterated", zap.Int("objects", numberOfElements))
			}

			numberOfElements++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(streamIDs))
	for id := range streamIDs {
		ids = append(ids, id)
	}

	return ids, nil
}

func sendDelete(ctx context.Context, conn *pgx.Conn, ids []uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(ids) == 0 {
		return nil
	}

	_, err = conn.Exec(ctx, "DELETE FROM segments WHERE stream_id = ANY($1)", pgutil.UUIDArray(ids))
	if err != nil {
		return err
	}
	return nil
}
