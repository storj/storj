// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/version"
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
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
		Use:   "satellite",
		Short: "satellite",
	}

	factory := &Factory{}

	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.ConfDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.IdentityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	cmd.PersistentFlags().BoolVar(&factory.UseColor, "color", false, "use color in user interface")

	factory.Defaults = cfgstruct.DefaultsFlag(cmd)

	ball := CreateModule()
	selector := modular.CreateSelector(ball)

	// TODO: use proper process context
	ctx, cancel := context.WithCancel(context.Background())
	stop := &modular.StopTrigger{}
	mud.Supply[*modular.StopTrigger](ball, stop)
	stop.Cancel = cancel

	cmd.AddCommand(
		newExecCmd(ctx, ball, factory, selector),
		newComponentCmd(ctx, ball, selector),
		&cobra.Command{
			Use:   "version",
			Short: "output the version's build information, if any",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Println(version.Build)
				return nil
			},
			Annotations: map[string]string{"type": "setup"}},
	)

	return cmd, factory
}
