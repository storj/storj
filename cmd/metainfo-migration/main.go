// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	defaultReadBatchSize         = 3000000
	defaultWriteBatchSize        = 100
	defaultWriteParallelLimit    = 6
	defaultPreGeneratedStreamIDs = 100
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

	readBatchSize         = flag.Int("readBatchSize", defaultReadBatchSize, "batch size for selecting pointers from DB")
	writeBatchSize        = flag.Int("writeBatchSize", defaultWriteBatchSize, "batch size for inserting objects and segments")
	writeParallelLimit    = flag.Int("writeParallelLimit", defaultWriteParallelLimit, "limit of parallel batch writes")
	preGeneratedStreamIDs = flag.Int("preGeneratedStreamIDs", defaultPreGeneratedStreamIDs, "number of pre generated stream ids for segment")
	nodes                 = flag.String("nodes", "", "file with nodes ids")
	invalidObjects        = flag.String("invalidObjects", "", "file for storing invalid objects")

	pointerdb  = flag.String("pointerdb", "", "connection URL for PointerDB")
	metabasedb = flag.String("metabasedb", "", "connection URL for MetabaseDB")
)

func main() {
	flag.Parse()

	if *pointerdb == "" {
		log.Fatalln("Flag '--pointerdb' is not set")
	}
	if *metabasedb == "" {
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

	config := Config{
		PreGeneratedStreamIDs: *preGeneratedStreamIDs,
		ReadBatchSize:         *readBatchSize,
		WriteBatchSize:        *writeBatchSize,
		WriteParallelLimit:    *writeParallelLimit,
		Nodes:                 *nodes,
		InvalidObjectsFile:    *invalidObjects,
	}
	migrator := NewMigrator(log, *pointerdb, *metabasedb, config)
	err = migrator.MigrateProjects(ctx)
	if err != nil {
		panic(err)
	}
}
