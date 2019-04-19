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
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// UplinkFlags configuration flags
type UplinkFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`
	Identity       identity.Config
	uplink.Config
}

var cfg UplinkFlags

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

	cfgstruct.Bind(cmd.Flags(), &cfg, defaults, cfgstruct.ConfDir(defaultConfDir))

  return cmd
}

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (c *UplinkFlags) NewUplink(ctx context.Context, config *libuplink.Config) (*libuplink.Uplink, error) {
	return libuplink.NewUplink(ctx, config)
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (c *UplinkFlags) GetProject(ctx context.Context) (*libuplink.Project, error) {
	apiKey, err := libuplink.ParseAPIKey(c.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := c.Client.SatelliteAddr

	identity, err := c.Identity.Load()
	if err != nil {
		return nil, err
	}

	identityVersion, err := identity.Version()
	if err != nil {
		return nil, err
	}

	cfg := &libuplink.Config{}

	cfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !c.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: c.TLS.PeerCAWhitelistPath,
	}

	cfg.Volatile.UseIdentity = identity
	cfg.Volatile.IdentityVersion = identityVersion
	cfg.Volatile.MaxInlineSize = c.Client.MaxInlineSize
	cfg.Volatile.MaxMemory = c.RS.MaxBufferMem

	uplink, err := c.NewUplink(ctx, cfg)
	if err != nil {
		return nil, err
	}

	opts := &libuplink.ProjectOptions{}

	encKey := new(storj.Key)
	copy(encKey[:], c.Enc.Key)
	opts.Volatile.EncryptionKey = encKey

	return uplink.OpenProject(ctx, satelliteAddr, apiKey, opts)
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
