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
	defaultSamplePercent = 1.0
)

var (
	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

	samplePercent = flag.Float64("samplePercent", defaultSamplePercent, "sample size to verify in percents")

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
	if *samplePercent < 0 || *samplePercent > 100 {
		log.Fatalln("Flag '--samplePercent' can take values between 0 and 100")
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
		SamplePercent: *samplePercent,
	}
	verifier := NewVerifier(log, *pointerdb, *metabasedb, config)
	err = verifier.VerifyPointers(ctx)
	if err != nil {
		panic(err)
	}
}
