// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/storage/buckets"
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

func makeUplinkPath() (defaultConfDir string) {
	base, err := homedir.Dir()
	if err != nil {
		zap.S().Errorf("error setting up uplink directory path: %s", err)
		return ""
	}
	return filepath.Join(base, ".storj", "uplink")
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	defaultConfDir := makeUplinkPath()
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

func convertError(err error, path fpath.FPath) error {
	if storj.ErrBucketNotFound.Has(err) {
		return fmt.Errorf("Bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("Object not found: %s", path.String())
	}

	return err
}
