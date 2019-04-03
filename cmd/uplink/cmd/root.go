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

// LibClient stores a reference to the Uplink client for interaction with the network.
var LibClient *libuplink.Uplink

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
	// defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "uplink")

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}

	// TODO (dylan): CLI won't need identity after libuplink changes, so this can be removed.
	// identityDirParam := cfgstruct.FindIdentityDirParam()
	// if identityDirParam != "" {
	// 	defaultIdentityDir = identityDirParam
	// }

	cfgstruct.Bind(cmd.Flags(), &cfg, isDev, cfgstruct.ConfDir(defaultConfDir))

	uplink, err := cfg.NewUplink(context.Background())
	if err != nil {
		fmt.Printf("error setting up uplink %+v\n", err)
		return cmd
	}

	LibClient = uplink

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

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (c *UplinkFlags) NewUplink(ctx context.Context) (*libuplink.Uplink, error) {
	return libuplink.NewUplink(ctx, nil)
}

// NewUplinkWithConfigs allows configs to be passed through to libuplink from the command line flags
func NewUplinkWithConfigs(ctx context.Context, config cfgstruct.FlagSet) (*libuplink.Uplink, error) {
	// TODO (dylan): Add a function to allow passing a FlagSet to a NewUplink.
	panic("TODO")
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (c *Client) GetProject(ctx context.Context) (*libuplink.Project, error) {
	apiKey, err := libuplink.ParseAPIKey(c.Flags.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := c.Flags.Config.Client.SatelliteAddr

	return c.Uplink.OpenProject(ctx, satelliteAddr, apiKey)
}

// CreateBucket will create a bucket and return an error if it wasn't created
func (c *Client) CreateBucket(ctx context.Context, name string) (storj.Bucket, error) {
	apiKey, err := c.getAPIKey()
	if err != nil {
		return storj.Bucket{}, err
	}

	satelliteAddr := c.getSatelliteAddr()
	project, err := c.Uplink.OpenProject(ctx, satelliteAddr, apiKey)
	if err != nil {
		return storj.Bucket{}, err
	}

	// TODO (dylan) Make this allow for configs
	return project.CreateBucket(ctx, name, nil)
}

// GetBucket returns a bucket or an error if no bucket with that name exists.
func (c *Client) GetBucket(ctx context.Context, name string) (storj.Bucket, error) {
	panic("TODO")
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
