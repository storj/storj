// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/minio/cli"
	"github.com/minio/minio/cmd"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/objects"
	"storj.io/storj/pkg/process"
)

func main() {
	process.Must(process.Main(
		process.ConfigEnvironment, process.ServiceFunc(run)))
}

func run(ctx context.Context, _ *cobra.Command, args []string) error {
	err := cmd.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})
	if err != nil {
		return err
	}
	cmd.Main(append([]string{os.Args[0]}, args...))
	return nil
}

func storjGatewayMain(ctx *cli.Context) {
	cmd.StartGateway(ctx, miniogw.NewStorjGateway(objects.NewObjectStore()))
}
