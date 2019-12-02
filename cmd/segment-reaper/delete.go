// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/csv"
	"os"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/process"
	"storj.io/storj/satellite/metainfo"
)

var (
	deleteCmd = &cobra.Command{
		Use:   "delete input_file.csv [flags]",
		Short: "Deletes zombie segments from DB",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdDelete,
	}

	deleteCfg struct {
		DatabaseURL string `help:"the database connection string to use" default:"postgres://"`
		DryRun      bool   `help:"with this option no deletion will be done, only printing results" default:"false"`
	}
)

func init() {
	rootCmd.AddCommand(deleteCmd)

	process.Bind(deleteCmd, &deleteCfg)
}

func cmdDelete(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()
	db, err := metainfo.NewStore(log.Named("pointerdb"), detectCfg.DatabaseURL)
	if err != nil {
		return errs.New("error connecting database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	inputFile, err := os.Open(args[0])
	if err != nil {
		return errs.New("error opening input file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, inputFile.Close())
	}()

	_ = csv.NewReader(inputFile)

	// TODO logice will be added in next PR

	return nil
}
