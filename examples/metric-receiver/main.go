// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/telemetry"
)

var (
	addr = flag.String("addr", ":9000", "address to listen for metrics on")
)

func main() {
	process.Must(process.Main(process.ServiceFunc(run)))
}

func run(ctx context.Context) error {
	s, err := telemetry.Listen(*addr)
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Printf("listening on %s\n", s.Addr())
	return s.Serve(ctx, telemetry.HandlerFunc(handle))
}

func handle(application, instance string, key []byte, val float64) {
	fmt.Printf("%s %s %s %v\n", application, instance, string(key), val)
}
