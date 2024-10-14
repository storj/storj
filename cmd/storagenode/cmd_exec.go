// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/process"
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/storagenode"
)

// newExecCmd creates a new exec command.
func newExecCmd(f *Factory) *cobra.Command {
	ball := &mud.Ball{}
	mud.Provide[*zap.Logger](ball, zap.L)
	modular.IdentityModule(ball)
	storagenode.Module(ball)
	selector := modular.CreateSelector()
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "execute selected components (VERY, VERY, EXPERIMENTAL)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return cmdExec(ctx, ball, selector)
		},
	}

	err := config.BindAll(context.Background(), cmd, ball, selector, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))
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
