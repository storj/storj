// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/spf13/cobra"
	"storj.io/storj/pkg/miniogw"
)

const defaultConfDir = "$HOME/.storj/cli"

type Config struct {
	miniogw.Config
}

func getStorjObjects(ctx context.Context, cfg Config) (minio.ObjectLayer, error) {
	identity, err := cfg.Load()
	if err != nil {
		return nil, err
	}

	gateway, err := cfg.NewGateway(ctx, identity)
	if err != nil {
		return nil, err
	}

	credentials, err := auth.CreateCredentials(cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return nil, err
	}

	storjObjects, err := gateway.NewGatewayLayer(credentials)
	if err != nil {
		return nil, err
	}

	return storjObjects, nil
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "storj",
	Short: "A brief description of your application",
}
