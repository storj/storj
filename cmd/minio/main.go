// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/minio/minio/cmd"
	"github.com/spf13/cobra"

	_ "storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
)

func main() { process.Must(process.Main(process.ServiceFunc(run))) }

func run(ctx context.Context, _ *cobra.Command, args []string) error {
	cmd.Main(append([]string{os.Args[0]}, args...))
	return nil
}
