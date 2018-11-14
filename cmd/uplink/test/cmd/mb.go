// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testuplink

import (
	"context"
	"fmt"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storage"
)

// MB HI THIS IS THE MB FUNCTION
func MB(ctx context.Context, uplink *testplanet.Node, args string) error {

	if len(args) == 0 {
		return fmt.Errorf("No bucket specified for creation")
	}

	dst, err := fpath.New(args)
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("No bucket specified, use format sj://bucket/")
	}

	if dst.Path() != "" {
		return fmt.Errorf("Nested buckets not supported, use format sj://bucket/")
	}

	bs, err := uplink.Client.GetBucketStore(ctx, uplink.Identity)
	if err != nil {
		return err
	}

	_, err = bs.Get(ctx, dst.Bucket())
	if err == nil {
		return fmt.Errorf("Bucket already exists")
	}
	if !storage.ErrKeyNotFound.Has(err) {
		return err
	}
	_, err = bs.Put(ctx, dst.Bucket(), storj.Cipher(uplink.Client.PathEncType))
	if err != nil {
		return err
	}

	fmt.Printf("Bucket %s created\n", dst.Bucket())

	return nil
}
