// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/storj/cmd/storagenode/internalcmd"
	"storj.io/storj/storagenode"
)

// StorageNodeFlags defines storage node configuration.
type StorageNodeFlags struct {
	EditConf bool `default:"false" help:"open config in default editor"`

	storagenode.Config

	Deprecated
}

// Factory contains default values for configuration flags.
type Factory struct {
	Defaults    cfgstruct.BindOpt
	ConfDir     string
	IdentityDir string
	UseColor    bool
}

// newRootCmd creates a new root command.
func newRootCmd(setDefaults bool) (*cobra.Command, *Factory) {
	cmd := &cobra.Command{
		Use:   "storagenode",
		Short: "Storagenode",
	}

	factory := &Factory{}

	if setDefaults {
		defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
		defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
		cfgstruct.SetupFlag(zap.L(), cmd, &factory.ConfDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
		cfgstruct.SetupFlag(zap.L(), cmd, &factory.IdentityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
		cmd.PersistentFlags().BoolVar(&factory.UseColor, "color", false, "use color in user interface")

		factory.Defaults = cfgstruct.DefaultsFlag(cmd)
	}

	cmd.AddCommand(
		newConfigCmd(factory),
		newSetupCmd(factory),
		newDashboardCmd(factory),
		newDiagCmd(factory),
		newRunCmd(factory),
		newNodeInfoCmd(factory),
		newIssueAPIKeyCmd(factory),
		newGracefulExitInitCmd(factory),
		newGracefulExitStatusCmd(factory),
		newForgetSatelliteCmd(factory),
		newForgetSatelliteStatusCmd(factory),
		// internal hidden commands
		internalcmd.NewUsedSpaceFilewalkerCmd().Command,
		internalcmd.NewGCFilewalkerCmd().Command,
		internalcmd.NewTrashFilewalkerCmd().Command,
	)

	return cmd, factory
}
