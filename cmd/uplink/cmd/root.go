// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storage/streams"
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

var cpuProfile = flag.String("profile.cpu", "", "file path of the cpu profile to be created")
var memoryProfile = flag.String("profile.mem", "", "file path of the memory profile to be created")

//RootCmd represents the base CLI command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:                "uplink",
	Short:              "The Storj client-side CLI",
	Args:               cobra.OnlyValidArgs,
	PersistentPreRunE:  startCPUProfile,
	PersistentPostRunE: stopAndWriteProfile,
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "uplink")

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	cfgstruct.Bind(cmd.Flags(), &cfg, defaults, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
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

func convertError(err error, path fpath.FPath) error {
	if storj.ErrBucketNotFound.Has(err) {
		return fmt.Errorf("Bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("Object not found: %s", path.String())
	}

	return err
}

func startCPUProfile(cmd *cobra.Command, args []string) error {
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			return err
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			return err
		}
	}
	return nil
}

func stopAndWriteProfile(cmd *cobra.Command, args []string) error {
	if *cpuProfile != "" {
		pprof.StopCPUProfile()
	}
	if *memoryProfile != "" {
		return writeMemoryProfile()
	}
	return nil
}

func writeMemoryProfile() error {
	f, err := os.Create(*memoryProfile)
	if err != nil {
		return err
	}
	runtime.GC()
	if err := pprof.WriteHeapProfile(f); err != nil {
		return err
	}
	return f.Close()
}
