// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademliadb

import (
	"context"

	"storj.io/storj/storage"
)


type KBuckets struct {
	db storage.KeyValueStore
}

// bID bucketID, now time.Time
func (k *KBuckets) Put(ctx context.Context, bucketID []byte, timeStamp []byte) error {}
