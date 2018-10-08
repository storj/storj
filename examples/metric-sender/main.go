// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
)

func main() {
	err := flag.Set("metrics.interval", "1s")
	if err != nil {
		fmt.Println(err)
	}

	process.Exec(&cobra.Command{
		Use:   "metric-sender",
		Short: "send metrics",
		RunE:  run,
	})
}

func run(cmd *cobra.Command, args []string) error {
	// just go to sleep and let the background telemetry start sending
	select {}
}
