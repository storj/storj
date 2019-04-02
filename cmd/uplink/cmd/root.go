// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// UplinkFlags configuration flags
type UplinkFlags struct {
	Identity identity.Config
	uplink.Config
}

var cfg UplinkFlags

// Client holds the libuplink Uplink and Config information
type Client struct {
	Uplink *libuplink.Uplink
	Config *libuplink.Config
	Flags  *UplinkFlags
}

//RootCmd represents the base CLI command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "uplink",
	Short: "The Storj client-side CLI",
	Args:  cobra.OnlyValidArgs,
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "uplink")

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	// TODO (dylan): CLI won't need identity after libuplink changes, so this can be removed.
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	cfgstruct.Bind(cmd.Flags(), &cfg, isDev, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))

	return cmd
}

// Metainfo loads the storj.Metainfo
//
// Temporarily it also returns an instance of streams.Store until we improve
// the metainfo and streas implementations.
func (c *UplinkFlags) Metainfo(ctx context.Context) (storj.Metainfo, streams.Store, error) {
	identity, err := c.Identity.Load()
	if err != nil {
		return nil, nil, err
	}

	return c.GetMetainfo(ctx, identity)
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (c *Client) GetProject(ctx context.Context, flags cfgstruct.FlagSet) (*libuplink.Project, error) {
	apiKey, err := libuplink.ParseAPIKey(c.Flags.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := c.Flags.Config.Client.SatelliteAddr

	return c.Uplink.OpenProject(ctx, satelliteAddr, apiKey)
}

// NewClient returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func GetUplink(ctx context.Context, flags cfgstruct.FlagSet) (*libuplink.Uplink, error) {
	var config libuplink.Config
	cfgstruct.Bind(flags, config, false)
	return libuplink.NewUplink(ctx, config)
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
