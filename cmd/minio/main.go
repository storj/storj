// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"os"

	"github.com/minio/minio/cmd"

	_ "storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
)

func main() { process.Must(process.Main(process.ServiceFunc(run))) }

func run(ctx context.Context) error {
	cmd.Main(append([]string{os.Args[0]}, flag.Args()...))
	return nil
}
