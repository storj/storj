// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mv SOURCE DESTINATION",
		Short: "Moves a Storj object to another location in Storj",
		RunE:  move,
		Args:  cobra.ExactArgs(2),
	}, RootCmd)

}

func move(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := withTelemetry(cmd)

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	dst, err := fpath.New(args[1])
	if err != nil {
		return err
	}

	if src.IsLocal() || dst.IsLocal() {
		return errors.New("the source and the destination must be a Storj URL")
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	err = project.MoveObject(ctx, src.Bucket(), src.Path(), dst.Bucket(), dst.Path(), nil)
	if err != nil {
		return err
	}

	fmt.Printf("%s moved to %s\n", src.String(), dst.String())

	return nil
}
