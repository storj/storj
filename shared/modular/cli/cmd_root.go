// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

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

// NewRootCmd creates a new root command.
func NewRootCmd(name string, module func(ball *mud.Ball)) (*cobra.Command, *Factory) {
	cmd := &cobra.Command{
		Use:   name,
		Short: name,
	}

	factory := &Factory{}

	defaultConfDir := fpath.ApplicationDir("storj", name)
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", name)
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.ConfDir, "config-dir", defaultConfDir, "main directory for "+name+" configuration")
	cfgstruct.SetupFlag(zap.L(), cmd, &factory.IdentityDir, "identity-dir", defaultIdentityDir, "main directory for "+name+" identity credentials")
	cmd.PersistentFlags().BoolVar(&factory.UseColor, "color", false, "use color in user interface")

	factory.Defaults = cfgstruct.DefaultsFlag(cmd)

	ball := mud.NewBall()
	module(ball)
	selector := modular.CreateSelector(ball)

	// TODO: use proper process context
	ctx, cancel := context.WithCancel(context.Background())
	stop := &modular.StopTrigger{}
	mud.Supply[*modular.StopTrigger](ball, stop)
	stop.Cancel = cancel

	cmd.AddCommand(
		newExecCmd(ctx, ball, factory, selector),
		NewComponentCmd(ctx, ball, selector),
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
