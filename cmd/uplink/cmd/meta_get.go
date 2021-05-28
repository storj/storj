// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "get [KEY] PATH",
		Short: "Get a Storj object's metadata",
		RunE:  metaGetMain,
		Args:  cobra.RangeArgs(1, 2),
	}, metaCmd)
}

// metaGetMain is the function executed when metaGetCmd is called.
func metaGetMain(cmd *cobra.Command, args []string) (err error) {
	var key *string
	var path string

	switch len(args) {
	case 0:
		return fmt.Errorf("no object specified")
	case 1:
		path = args[0]
	case 2:
		key = &args[0]
		path = args[1]
	default:
		return fmt.Errorf("too many arguments")
	}

	ctx, _ := withTelemetry(cmd)

	src, err := fpath.New(path)
	if err != nil {
		return err
	}
	if src.IsLocal() {
		return fmt.Errorf("the source destination must be a Storj URL")
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	object, err := project.StatObject(ctx, src.Bucket(), src.Path())
	if err != nil {
		return err
	}

	if key != nil {
		var keyNorm string
		err := json.Unmarshal([]byte("\""+*key+"\""), &keyNorm)
		if err != nil {
			return err
		}

		value, ok := object.Custom[keyNorm]
		if !ok {
			return fmt.Errorf("key does not exist")
		}

		str, err := json.Marshal(value)
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", str[1:len(str)-1])

		return nil
	}

	if object.Custom != nil {
		str, err := json.MarshalIndent(object.Custom, "", "  ")
		if err != nil {
			return err
		}

		fmt.Printf("%s\n", string(str))

		return nil
	}

	fmt.Printf("{}\n")

	return nil
}
