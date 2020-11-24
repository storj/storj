// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/csv"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/satellite/metainfo"
)

var (
	detectCmd = &cobra.Command{
		Use:   "detect",
		Short: "Detects zombie segments in DB",
		Args:  cobra.OnlyValidArgs,
		RunE:  cmdDetect,
	}

	detectCfg struct {
		DatabaseURL string `help:"the database connection string to use" default:"postgres://"`
		From        string `help:"begin of date range for detecting zombie segments (RFC3339)" default:""`
		To          string `help:"end of date range for detecting zombie segments (RFC3339)" default:""`
		File        string `help:"location of file with report" default:"zombie-segments.csv"`
	}
)

func init() {
	rootCmd.AddCommand(detectCmd)

	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(detectCmd, &detectCfg, defaults)
}

func cmdDetect(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	log := zap.L()

	if err := process.InitMetricsWithHostname(ctx, log, nil); err != nil {
		log.Warn("Failed to initialize telemetry batcher on segment reaper", zap.Error(err))
	}

	db, err := metainfo.OpenStore(ctx, log.Named("pointerdb"), detectCfg.DatabaseURL)
	if err != nil {
		return errs.New("error connecting database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	file, err := os.Create(detectCfg.File)
	if err != nil {
		return errs.New("error creating result file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, file.Close())
	}()

	writer := csv.NewWriter(file)
	defer func() {
		writer.Flush()
		err = errs.Combine(err, writer.Error())
	}()

	var from, to *time.Time
	if detectCfg.From != "" {
		fromTime, err := time.Parse(time.RFC3339, detectCfg.From)
		if err != nil {
			return err
		}
		from = &fromTime
	}

	if detectCfg.To != "" {
		toTime, err := time.Parse(time.RFC3339, detectCfg.To)
		if err != nil {
			return err
		}
		to = &toTime
	}

	observer, err := newObserver(db, writer, from, to)
	if err != nil {
		return err
	}

	err = observer.detectZombieSegments(ctx)
	if err != nil {
		return err
	}

	log.Info("number of inline segments", zap.Int("segments", observer.inlineSegments))
	log.Info("number of last inline segments", zap.Int("segments", observer.lastInlineSegments))
	log.Info("number of remote segments", zap.Int("segments", observer.remoteSegments))
	log.Info("number of zombie segments", zap.Int("segments", observer.zombieSegments))

	mon.IntVal("zombie_segments").Observe(int64(observer.zombieSegments)) //mon:locked

	return process.Report(ctx)
}
