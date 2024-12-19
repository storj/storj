// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/cfgstruct"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// newExecCmd creates a new exec command.
func newExecCmd(ctx context.Context, ball *mud.Ball, factory *Factory, selector mud.ComponentSelector) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "execute selected components (VERY, VERY, EXPERIMENTAL)",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := LoadConfig(cmd, ball, selector)
			if err != nil {
				return err
			}
			err = cmdExec(ctx, ball, selector)
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		},
	}

	err := config.BindAll(context.Background(), cmd, ball, selector, factory.Defaults, cfgstruct.ConfDir(factory.ConfDir), cfgstruct.IdentityDir(factory.IdentityDir))
	if err != nil {
		panic(err)
	}

	return cmd
}

func cmdExec(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector) (err error) {
	err = modular.Initialize(ctx, ball, selector)
	if err != nil {
		return err
	}
	err1 := modular.Run(ctx, ball, selector)
	err2 := modular.Close(ctx, ball, selector)
	return errs.Combine(err1, err2)

}
