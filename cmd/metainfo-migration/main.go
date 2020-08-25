// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/satellite/satellitedb"
)

func main() {
	// bucketName := os.Args[1]

	ctx := context.Background()

	metabase, err := DialMetainfo(ctx, "postgres://postgres:abc@localhost/metabase?sslmode=disable")
	check(err)
	defer func() { check(metabase.Close(ctx)) }()

	check(metabase.Drop(ctx))
	check(metabase.Migrate(ctx))

	log := zap.L()
	satellitedb, err := satellitedb.New(log.Named("db"), "postgres://postgres:abc@localhost/test_storj_sim?sslmode=disable&options=--search_path%3D%22satellite%2F0%22",
		satellitedb.Options{})
	check(err)
	defer func() { check(satellitedb.Close()) }()

	pointerdb, err := metainfo.NewStore(log.Named("pointerdb"), "postgres://postgres:abc@localhost/test_storj_sim?sslmode=disable&options=--search_path%3D%22satellite%2F0%2Fmeta%22")
	check(err)
	defer func() { check(pointerdb.Close()) }()

	projectID, err := uuid.FromString("8e1c62d9-5a0c-410f-a33e-817689520f34")
	check(err)

	bucketID, err := satellitedb.Buckets().GetBucketID(ctx, []byte("metabase"), projectID)
	check(err)

	migrator := NewMigrator(pointerdb, metabase, projectID, bucketID, []byte("metabase"))
	err = migrator.MigrateBucket(ctx)
	check(err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
