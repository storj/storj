// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/telemetry"
)

var (
	addr = flag.String("addr", ":9000", "address to listen for metrics on")
)

func main() {
	process.Exec(&cobra.Command{
		Use:   "metric-receiver",
		Short: "receive metrics",
		RunE:  run,
	})
}

func run(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	s, err := telemetry.Listen(*addr)
	if err != nil {
		return err
	}
	defer printError(s.Close)

	fmt.Printf("listening on %s\n", s.Addr())
	return s.Serve(ctx, telemetry.HandlerFunc(handle))
}

func handle(application, instance string, key []byte, val float64) {
	fmt.Printf("%s %s %s %v\n", application, instance, string(key), val)
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
