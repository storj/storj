// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/storage/buckets"
)

const defaultConfDir = "$HOME/.storj/uplink"

// Config is miniogw.Config configuration
type Config struct {
	miniogw.Config
}

var cfg Config

// CLICmd represents the base CLI command when called without any subcommands
var CLICmd = &cobra.Command{
	Use:   "uplink",
	Short: "The Storj client-side CLI",
}

// GWCmd represents the base gateway command when called without any subcommands
var GWCmd = &cobra.Command{
	Use:   "gateway",
	Short: "The Storj client-side S3 gateway",
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)
	cfgstruct.Bind(cmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	return cmd
}

// BucketStore loads the buckets.Store
func (c *Config) BucketStore(ctx context.Context) (buckets.Store, error) {
	identity, err := c.Load()
	if err != nil {
		return nil, err
	}

	return c.GetBucketStore(ctx, identity)
}
