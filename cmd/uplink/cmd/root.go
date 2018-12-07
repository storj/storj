// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

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

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	cfgstruct.Bind(cmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
	cmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	return cmd
}

// Metainfo loads the storj.Metainfo
//
// Temporarily it also returns an instance of streams.Store until we improve
// the metainfo and streas implementations.
func (c *Config) Metainfo(ctx context.Context) (storj.Metainfo, streams.Store, error) {
	identity, err := c.Identity.Load()
	if err != nil {
		return nil, nil, err
	}

	return c.GetMetainfo(ctx, identity)
}

func convertError(err error, path fpath.FPath) error {
	if storj.ErrBucketNotFound.Has(err) {
		return fmt.Errorf("Bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("Object not found: %s", path.String())
	}

	return err
}
