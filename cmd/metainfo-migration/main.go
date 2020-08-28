// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metainfo"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Println("usage: metainfo-migration pointerdb-conn-url metabase-conn-url project-id bucket-name")
		os.Exit(1)
	}

	pointerDBUrl := os.Args[1]
	metabaseDBUrl := os.Args[2]
	projectIDString := os.Args[3]
	bucketName := os.Args[4]

	ctx := context.Background()

	// postgres://postgres:abc@localhost/metabase?sslmode=disable
	// postgresql://root@localhost:26257/metabase?sslmode=disable
	// postgres://postgres:abc@localhost/test_storj_sim?sslmode=disable&options=--search_path%3D%22satellite%2F0%2Fmeta%22
	// 8e1c62d9-5a0c-410f-a33e-817689520f34

	metabase, err := DialMetainfo(ctx, metabaseDBUrl)
	check(err)
	defer func() { check(metabase.Close(ctx)) }()

	check(metabase.Drop(ctx))
	check(metabase.Migrate(ctx))

	log := zap.L()
	pointerdb, err := metainfo.NewStore(log.Named("pointerdb"), pointerDBUrl)
	check(err)
	defer func() { check(pointerdb.Close()) }()

	projectID, err := uuid.FromString(projectIDString)
	check(err)

	start := time.Now()
	migrator := NewMigrator(pointerdb, metabase, projectID, []byte(bucketName))
	err = migrator.MigrateBucket(ctx)
	check(err)

	fmt.Println("Migration time:", time.Now().Sub(start).Seconds())
	fmt.Println("Objects created:", migrator.ObjectsCreated)
	fmt.Println("Segments created:", migrator.SegmentsCreated)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
