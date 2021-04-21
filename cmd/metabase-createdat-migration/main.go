// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metainfo"
)

// Config defines configuration for migration.
type Config struct {
	LoopBatchSize   int
	UpdateBatchSize int
}

type update struct {
	StreamID  uuid.UUID
	CreatedAt time.Time
}

func main() {
	var metabasedb string
	var loopBatchSize, updateBatchSize int
	flag.StringVar(&metabasedb, "metabasedb", "", "connection URL for MetabaseDB")
	flag.IntVar(&loopBatchSize, "loop-batch-size", 10000, "number of objects to process at once")
	flag.IntVar(&updateBatchSize, "update-batch-size", 1000, "number of update requests in a single batch call")

	flag.Parse()

	if metabasedb == "" {
		log.Fatalln("Flag '--metabasedb' is not set")
	}

	ctx := context.Background()
	log, err := zap.Config{
		Encoding:         "console",
		Level:            zap.NewAtomicLevelAt(zapcore.DebugLevel),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stdout"},
		EncoderConfig: zapcore.EncoderConfig{
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
	}.Build()
	if err != nil {
		panic(err)
	}
	defer func() { _ = log.Sync() }()

	err = Migrate(ctx, log, metabasedb, Config{
		LoopBatchSize:   loopBatchSize,
		UpdateBatchSize: updateBatchSize,
	})
	if err != nil {
		panic(err)
	}
}

// Migrate migrates created_at from object to corresponding segment if value is missing there.
func Migrate(ctx context.Context, log *zap.Logger, metabaseDBStr string, config Config) error {
	rawMetabaseDB, err := pgx.Connect(ctx, metabaseDBStr)
	if err != nil {
		return errs.New("unable to connect %q: %w", metabaseDBStr, err)
	}
	defer func() { err = errs.Combine(err, rawMetabaseDB.Close(ctx)) }()

	metabaseDB, err := metainfo.OpenMetabase(ctx, log.Named("metabase"), metabaseDBStr)
	if err != nil {
		return errs.New("unable to connect %q: %w", metabaseDBStr, err)
	}
	defer func() { err = errs.Combine(err, metabaseDB.Close()) }()

	startingTime := time.Now()

	updates := make([]update, 0, config.UpdateBatchSize)

	numberOfObjects := 0
	return metabaseDB.IterateLoopObjects(ctx, metabase.IterateLoopObjects{
		BatchSize:      config.LoopBatchSize,
		AsOfSystemTime: startingTime,
	}, func(ctx context.Context, it metabase.LoopObjectsIterator) error {
		var entry metabase.LoopObjectEntry
		for it.Next(ctx, &entry) {
			updates = append(updates, update{
				StreamID:  entry.StreamID,
				CreatedAt: entry.CreatedAt,
			})

			if len(updates) == config.UpdateBatchSize {
				sendUpdates(ctx, log, rawMetabaseDB, updates)

				updates = updates[:0]
			}

			if numberOfObjects%1000000 == 0 {
				log.Info("updated", zap.Int("objects", numberOfObjects))
			}

			numberOfObjects++
		}

		sendUpdates(ctx, log, rawMetabaseDB, updates)

		log.Info("finished", zap.Int("objects", numberOfObjects))

		return nil
	})
}

func sendUpdates(ctx context.Context, log *zap.Logger, conn *pgx.Conn, updates []update) {
	if len(updates) == 0 {
		return
	}

	batch := &pgx.Batch{}
	for _, update := range updates {
		batch.Queue("UPDATE segments SET created_at = $2 WHERE stream_id = $1 AND created_at IS NULL;", update.StreamID, update.CreatedAt)
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
