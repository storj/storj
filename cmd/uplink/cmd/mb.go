// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mb",
		Short: "Create a new bucket",
		RunE:  makeBucket,
	})
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for creation")
	}

	u, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}
	if u.Host == "" {
		return fmt.Errorf("No bucket specified. Please use format sj://bucket/")
	}

	bs, err := cfg.BucketStore(ctx)
	if err != nil {
		return err
	}

	_, err = bs.Get(ctx, u.Host)
	if err == nil {
		return fmt.Errorf("Bucket already exists")
	}
	if !storage.ErrKeyNotFound.Has(err) {
		return err
	}
	_, err = bs.Put(ctx, u.Host)
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", u.Host)

	return nil
}
