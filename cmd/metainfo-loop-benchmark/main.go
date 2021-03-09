// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/private/process"
)

var (
	rootCmd = &cobra.Command{
		Use:   "metainfo-loop-benchmark",
		Short: "metainfo-loop-benchmark",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run metainfo-loop-benchmark",
		RunE:  run,
	}

	bench Bench
)

func init() {
	rootCmd.AddCommand(runCmd)

	bench.BindFlags(runCmd.Flags())
}

func run(cmd *cobra.Command, args []string) error {
	if err := bench.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()
	return bench.Run(ctx, log)
}

func main() {
	process.Exec(rootCmd)
}
