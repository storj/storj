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
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/fpath"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/private/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/uplink"
)

// UplinkFlags configuration flags
type UplinkFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`
	uplink.Config

	Version checker.Config

	PBKDFConcurrency int `help:"Unfortunately, up until v0.26.2, keys generated from passphrases depended on the number of cores the local CPU had. If you entered a passphrase with v0.26.2 earlier, you'll want to set this number to the number of CPU cores your computer had at the time. This flag may go away in the future. For new installations the default value is highly recommended." default:"0"`
}

var (
	cfg     UplinkFlags
	confDir string

	defaults = cfgstruct.DefaultsFlag(RootCmd)

	// Error is the class of errors returned by this package
	Error = errs.Class("uplink")
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "uplink")
	cfgstruct.SetupFlag(zap.L(), RootCmd, &confDir, "config-dir", defaultConfDir, "main directory for uplink configuration")
}

var cpuProfile = flag.String("profile.cpu", "", "file path of the cpu profile to be created")
var memoryProfile = flag.String("profile.mem", "", "file path of the memory profile to be created")

// RootCmd represents the base CLI command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:                "uplink",
	Short:              "The Storj client-side CLI",
	Args:               cobra.OnlyValidArgs,
	PersistentPreRunE:  startCPUProfile,
	PersistentPostRunE: stopAndWriteProfile,
}

func addCmd(cmd *cobra.Command, root *cobra.Command) *cobra.Command {
	root.AddCommand(cmd)

	process.Bind(cmd, &cfg, defaults, cfgstruct.ConfDir(getConfDir()))

	return cmd
}

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (cliCfg *UplinkFlags) NewUplink(ctx context.Context) (*libuplink.Uplink, error) {

	// Transform the uplink cli config flags to the libuplink config object
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.Log = zap.L()
	libuplinkCfg.Volatile.MaxInlineSize = cliCfg.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = cliCfg.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = cliCfg.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS.SkipPeerCAWhitelist = !cliCfg.TLS.UsePeerCAWhitelist
	libuplinkCfg.Volatile.TLS.PeerCAWhitelistPath = cliCfg.TLS.PeerCAWhitelistPath
	libuplinkCfg.Volatile.DialTimeout = cliCfg.Client.DialTimeout
	libuplinkCfg.Volatile.PBKDFConcurrency = cliCfg.PBKDFConcurrency

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (cliCfg *UplinkFlags) GetProject(ctx context.Context) (_ *libuplink.Project, err error) {
	err = checker.CheckProcessVersion(ctx, zap.L(), cliCfg.Version, version.Build, "Uplink")
	if err != nil {
		return nil, err
	}

	scope, err := cliCfg.GetScope()
	if err != nil {
		return nil, err
	}

	uplk, err := cliCfg.NewUplink(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			if err := uplk.Close(); err != nil {
				fmt.Printf("error closing uplink: %+v\n", err)
			}
		}
	}()

	return uplk.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
}

// GetProjectAndBucket returns a *libuplink.Bucket for interacting with a specific project's bucket
func (cliCfg *UplinkFlags) GetProjectAndBucket(ctx context.Context, bucketName string) (project *libuplink.Project, bucket *libuplink.Bucket, err error) {
	scope, err := cliCfg.GetScope()
	if err != nil {
		return nil, nil, err
	}

	uplk, err := cliCfg.NewUplink(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if err != nil {
			if err := uplk.Close(); err != nil {
				fmt.Printf("error closing uplink: %+v\n", err)
			}
		}
	}()

	project, err = uplk.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
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

	bucket, err = project.OpenBucket(ctx, bucketName, scope.EncryptionAccess)
	return project, bucket, err
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
		return fmt.Errorf("bucket not found: %s", path.Bucket())
	}

	if storj.ErrObjectNotFound.Has(err) {
		return fmt.Errorf("object not found: %s", path.String())
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
