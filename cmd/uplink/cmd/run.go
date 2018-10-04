// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
)

var (
	runCmd = addCmd(&cobra.Command{
		Use:   "run",
		Short: "Run the S3 gateway",
		RunE:  cmdRun,
	})
)

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	for _, flagname := range args {
		return fmt.Errorf("Invalid argument %#v. Try 'uplink run'", flagname)
	}

	address := cfg.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		address = net.JoinHostPort("localhost", port)
	}

	fmt.Printf("Starting Storj S3-compatible gateway!\n\n")
	fmt.Printf("Endpoint: %s\n", address)
	fmt.Printf("Access key: %s\n", cfg.AccessKey)
	fmt.Printf("Secret key: %s\n", cfg.SecretKey)

	ctx := process.Ctx(cmd)
	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	_, _, err = bs.List(ctx, "", "", 0)
	if err != nil {
		return fmt.Errorf("Failed to contact Satellite.\n"+
			"Perhaps your configuration is invalid?\n%s", err)
	}

	return cfg.Run(process.Ctx(cmd))
}
