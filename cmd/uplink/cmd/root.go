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

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}

	cfgstruct.Bind(cmd.Flags(), &cfg, isDev, cfgstruct.ConfDir(defaultConfDir))

	return cmd
}

// Metainfo loads the storj.Metainfo
// Deprecated: Use Libuplink methods instead.
// Temporarily it also returns an instance of streams.Store until we improve
// the metainfo and streas implementations.
func (c *UplinkFlags) Metainfo(ctx context.Context) (storj.Metainfo, streams.Store, error) {
	identity, err := c.Identity.Load()
	if err != nil {
		return nil, nil, err
	}

	return c.GetMetainfo(ctx, identity)
}

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (c *UplinkFlags) NewUplink(ctx context.Context) (*libuplink.Uplink, error) {
	fmt.Println("got to NewUplink")
	return libuplink.NewUplink(ctx, nil)
}

// NewUplinkWithConfigs allows configs to be passed through to libuplink from the command line flags
func NewUplinkWithConfigs(ctx context.Context, config cfgstruct.FlagSet) (*libuplink.Uplink, error) {
	// TODO (dylan): Add a function to allow passing a FlagSet to a NewUplink.
	panic("TODO")
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (c *UplinkFlags) GetProject(ctx context.Context) (*libuplink.Project, error) {
	apiKey, err := libuplink.ParseAPIKey(c.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := c.Client.SatelliteAddr

	uplink, err := c.NewUplink(ctx)
	if err != nil {
		return nil, err
	}

	fmt.Println("Got to OpenProject")
	return uplink.OpenProject(ctx, satelliteAddr, apiKey)
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

func (c *Client) getAPIKey() (libuplink.APIKey, error) {
	return libuplink.ParseAPIKey(c.Flags.Client.APIKey)
}

func (c *Client) getSatelliteAddr() string {
	return c.Flags.Client.SatelliteAddr
}
