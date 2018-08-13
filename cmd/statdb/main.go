// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/statdb"
)

func main() {
	process.Exec(&cobra.Command{
		Use:   "statdb",
		Short: "statdb",
		RunE:  run,
	})
}

func run(cmd *cobra.Command, args []string) error {
	s := &statdb.Service{}
	s.SetLogger(zap.L())
	s.SetMetricHandler(monkit.Default)
	return s.Process(process.Ctx(cmd), cmd, args)
}
