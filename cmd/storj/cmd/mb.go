// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"path/filepath"

	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

var (
	mbCfg Config
	mbCmd = &cobra.Command{
		Use:   "mb",
		Short: "Create a new bucket",
		RunE:  makeBucket,
	}
)

func init() {
	RootCmd.AddCommand(mbCmd)
	cfgstruct.Bind(mbCmd.Flags(), &mbCfg, cfgstruct.ConfDir(defaultConfDir))
	mbCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func makeBucket(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	identity, err := mbCfg.Load()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return errs.New("No bucket specified for creation")
	}

	bs, err := mbCfg.GetBucketStore(ctx, identity)
	if err != nil {
		return err
	}

	u, err := utils.ParseURL(args[0])
	if err != nil {
		return err
	}

	_, err = bs.Get(ctx, u.Host)
	if err == nil {
		return minio.BucketAlreadyExists{Bucket: u.Host}
	}
	if !storage.ErrKeyNotFound.Has(err) {
		return err
	}
	m, err = bs.Put(ctx, u.Host)
	if err != nil {
		return err
	}

	fmt.Println(m, u.Host)

	return nil
}
