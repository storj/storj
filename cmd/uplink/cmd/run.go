// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "run",
		Short: "Run the S3 gateway",
		RunE:  cmdRun,
	}, GWCmd)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	if _, err := cfg.Identity.Load(); err != nil {
		zap.S().Fatal(err)
	}

	for _, flagname := range args {
		return fmt.Errorf("Invalid argument %#v. Try 'uplink run'", flagname)
	}

	address := cfg.Server.Address
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	if host == "" {
		address = net.JoinHostPort("localhost", port)
	}

	fmt.Printf("Starting Storj S3-compatible gateway!\n\n")
	fmt.Printf("Endpoint: %s\n", address)
	fmt.Printf("Access key: %s\n", cfg.Minio.AccessKey)
	fmt.Printf("Secret key: %s\n", cfg.Minio.SecretKey)

	ctx := process.Ctx(cmd)
	metainfo, _, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, nil, cfg.Identity.CertPath); err != nil {
		zap.S().Errorf("Failed to initialize telemetry batcher: %v", err)
	}
	_, err = metainfo.ListBuckets(ctx, storj.BucketListOptions{Direction: storj.After})
	if err != nil {
		return fmt.Errorf("Failed to contact Satellite.\n"+
			"Perhaps your configuration is invalid?\n%s", err)
	}

	return cfg.Run(ctx)
}
