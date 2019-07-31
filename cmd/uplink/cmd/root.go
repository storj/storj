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

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/version"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// UplinkFlags configuration flags
type UplinkFlags struct {
	NonInteractive bool `help:"disable interactive mode" default:"false" setup:"true"`
	uplink.Config

	Version version.Config
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

	defaultConfDir := fpath.ApplicationDir("storj", "uplink")

	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}

	process.Bind(cmd, &cfg, defaults, cfgstruct.ConfDir(defaultConfDir))

	return cmd
}

// NewUplink returns a pointer to a new Client with a Config and Uplink pointer on it and an error.
func (cliCfg *UplinkFlags) NewUplink(ctx context.Context) (*libuplink.Uplink, error) {

	// Transform the uplink cli config flags to the libuplink config object
	libuplinkCfg := &libuplink.Config{}
	libuplinkCfg.Volatile.MaxInlineSize = cliCfg.Client.MaxInlineSize
	libuplinkCfg.Volatile.MaxMemory = cliCfg.RS.MaxBufferMem
	libuplinkCfg.Volatile.PeerIDVersion = cliCfg.TLS.PeerIDVersions
	libuplinkCfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !cliCfg.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: cliCfg.TLS.PeerCAWhitelistPath,
	}

	libuplinkCfg.Volatile.DialTimeout = cliCfg.Client.DialTimeout
	libuplinkCfg.Volatile.RequestTimeout = cliCfg.Client.RequestTimeout

	return libuplink.NewUplink(ctx, libuplinkCfg)
}

// GetProject returns a *libuplink.Project for interacting with a specific project
func (cliCfg *UplinkFlags) GetProject(ctx context.Context) (*libuplink.Project, error) {
	err := version.CheckProcessVersion(ctx, zap.L(), cliCfg.Version, version.Build, "Uplink")
	if err != nil {
		return nil, err
	}

	apiKey, err := libuplink.ParseAPIKey(cliCfg.Client.APIKey)
	if err != nil {
		return nil, err
	}

	uplk, err := cliCfg.NewUplink(ctx)
	if err != nil {
		return nil, err
	}

	project, err := uplk.OpenProject(ctx, cliCfg.Client.SatelliteAddr, apiKey)
	if err != nil {
		if err := uplk.Close(); err != nil {
			fmt.Printf("error closing uplink: %+v\n", err)
		}
	}

	return project, err
}

// GetProjectAndBucket returns a *libuplink.Bucket for interacting with a specific project's bucket
func (cliCfg *UplinkFlags) GetProjectAndBucket(ctx context.Context, bucketName string, access *libuplink.EncryptionAccess) (project *libuplink.Project, bucket *libuplink.Bucket, err error) {
	project, err = cliCfg.GetProject(ctx)
	if err != nil {
		return project, bucket, err
	}

	defer func() {
		if err != nil {
			if err := project.Close(); err != nil {
				fmt.Printf("error closing project: %+v\n", err)
			}
		}
	}()

	bucket, err = project.OpenBucket(ctx, bucketName, access)
	if err != nil {
		return project, bucket, err
	}

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
