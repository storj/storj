// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
)

// Factory contains default values for configuration flags.
type Factory struct {
	Defaults    cfgstruct.BindOpt
	ConfDir     string
	IdentityDir string
	UseColor    bool
}

// newRootCmd creates a new root command.
func newRootCmd() (*cobra.Command, *Factory) {
	cmd := &cobra.Command{
		Use:   "storagenode",
		Short: "Storagenode",
	}

	factory := &Factory{}

	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.ConfDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.IdentityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	cmd.PersistentFlags().BoolVar(&factory.UseColor, "color", false, "use color in user interface")

	factory.Defaults = cfgstruct.DefaultsFlag(cmd)

	cmd.AddCommand(
		newExecCmd(factory),
	)

	return cmd, factory
}
