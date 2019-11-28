// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/csv"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/satellite/metainfo"
)

const maxNumOfSegments = byte(64)

var (
	detectCmd = &cobra.Command{
		Use:   "detect",
		Short: "Detects zombie segments in DB",
		Args:  cobra.OnlyValidArgs,
		RunE:  cmdDetect,
	}

	detectCfg struct {
		DatabaseURL string `help:"the database connection string to use" default:"postgres://"`
		From        string `help:"begin of date range for detecting zombie segments" default:""`
		To          string `help:"end of date range for detecting zombie segments" default:""`
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
	db, err := metainfo.NewStore(log.Named("pointerdb"), detectCfg.DatabaseURL)
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

	headers := []string{
		"ProjectID",
		"SegmentIndex",
		"Bucket",
		"EncodedEncryptedPath",
		"CreationDate",
	}
	err = writer.Write(headers)
	if err != nil {
		return err
	}

	observer := newObserver(db, writer)

	err = metainfo.IterateDatabase(ctx, db, observer)
	if err != nil {
		return err
	}

	log.Info("number of inline segments", zap.Int("segments", observer.inlineSegments))
	log.Info("number of last inline segments", zap.Int("segments", observer.lastInlineSegments))
	log.Info("number of remote segments", zap.Int("segments", observer.remoteSegments))
	return nil
}
