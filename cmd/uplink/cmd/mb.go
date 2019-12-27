// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mb",
		Short: "Create a new bucket",
		RunE:  makeBucket,
	}, RootCmd)
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("no bucket specified for creation")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	if dst.Path() != "" {
		return fmt.Errorf("nested buckets not supported, use format sj://bucket/")
	}

	project, err := cfg.GetProject(ctx)
	if err != nil {
		return errs.New("error setting up project: %+v", err)
	}
	defer func() {
		if err := project.Close(); err != nil {
			fmt.Printf("error closing project: %+v\n", err)
		}
	}()

	bucketCfg := &uplink.BucketConfig{}
	bucketCfg.PathCipher = cfg.GetPathCipherSuite()
	bucketCfg.EncryptionParameters = cfg.GetEncryptionParameters()
	bucketCfg.Volatile = struct {
		RedundancyScheme storj.RedundancyScheme
		SegmentsSize     memory.Size
	}{
		RedundancyScheme: cfg.GetRedundancyScheme(),
		SegmentsSize:     cfg.GetSegmentSize(),
	}

	_, err = project.CreateBucket(ctx, dst.Bucket(), bucketCfg)
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", dst.Bucket())

	return nil
}
