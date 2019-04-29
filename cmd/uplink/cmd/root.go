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

	encryptionKey, err := c.Enc.LoadKey()
	if err != nil {
		return nil, err
	}

	opts := &libuplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &encryptionKey

	project, err := uplink.OpenProject(ctx, satelliteAddr, apiKey, opts)

	if err != nil {
		if err := uplink.Close(); err != nil {
			fmt.Printf("error closing uplink: %+v\n", err)
		}
	}

	return project, err
}

// GetProjectAndBucket returns a *libuplink.Bucket for interacting with a specific project's bucket
func (c *UplinkFlags) GetProjectAndBucket(ctx context.Context, bucketName string, access libuplink.EncryptionAccess) (project *libuplink.Project, bucket *libuplink.Bucket, err error) {
	project, err = c.GetProject(ctx)
	if err != nil {
		return nil, nil, err
	}

	defer func() {
		if err != nil {
			if err := project.Close(); err != nil {
				fmt.Printf("error closing project: %+v\n", err)
			}
		}
	}()

	bucket, err = project.OpenBucket(ctx, bucketName, &access)
	if err != nil {
		return nil, nil, err
	}

	return project, bucket, nil
}

func closeProjectAndBucket(project *libuplink.Project, bucket *libuplink.Bucket) {
	if err := bucket.Close(); err != nil {
		fmt.Printf("error closing bucket: %+v\n", err)
	}

	if err := project.Close(); err != nil {
		fmt.Printf("error closing project: %+v\n", err)
	}
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
