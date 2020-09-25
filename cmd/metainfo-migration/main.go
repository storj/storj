// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"
	"storj.io/common/uuid"
	"storj.io/storj/cmd/metainfo-migration/metabase"
	"storj.io/storj/satellite/metainfo"
)

func main() {
	// fmt.Println(len(os.Args))
	if len(os.Args) != 4 {
		fmt.Println("usage: metainfo-migration pointerdb-conn-url metabase-conn-url csv-file")
		os.Exit(1)
	}

	// pointerDBUrl := "postgres://postgres:abc@localhost/test_storj_sim?sslmode=disable&options=--search_path%3D%22satellite%2F0%2Fmeta%22"
	pointerDBUrl := os.Args[1]
	// metabaseDBUrl := "postgres://postgres:abc@localhost/metabase?sslmode=disable"
	metabaseDBUrl := os.Args[2]
	// inputFile := "/home/wywrzal/Downloads/binary_local.txt"
	inputFile := os.Args[3]

	ctx := context.Background()

	// postgres://postgres:abc@localhost/metabase?sslmode=disable
	// postgresql://root@localhost:26257/metabase?sslmode=disable
	// postgres://postgres:abc@localhost/test_storj_sim?sslmode=disable&options=--search_path%3D%22satellite%2F0%2Fmeta%22
	// 8e1c62d9-5a0c-410f-a33e-817689520f34

	mb, err := metabase.Dial(ctx, metabaseDBUrl)
	check(err)
	defer func() { check(mb.Close(ctx)) }()

	check(mb.Drop(ctx))
	check(mb.Migrate(ctx))

	log := zap.L()
	pointerdb, err := metainfo.NewStore(log.Named("pointerdb"), pointerDBUrl)
	check(err)
	defer func() { check(pointerdb.Close()) }()

	csvfile, err := os.Open(inputFile)
	check(err)

	reader := csv.NewReader(csvfile)
	reader.Comma = ','

	// skip the first record - it's the header
	_, err = reader.Read()
	check(err)

	steps := []int{1, 11, 101, 1001, 1001, 50001, 100004, 200004, 900002}
	step := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		check(err)
		numberOfObjects, err := strconv.Atoi(record[2])
		check(err)
		if numberOfObjects > steps[step] {
			projectID, err := uuid.FromString(record[0])
			check(err)

			bucketName := record[1]

			start := time.Now()
			migrator := NewMigrator(pointerdb, mb, projectID, []byte(bucketName))
			err = migrator.MigrateBucket(ctx)
			check(err)

			fmt.Printf("%s,%s,%d,%d,%v,%s,%s\n", projectID.String(), bucketName, migrator.ObjectsCreated, migrator.SegmentsCreated, time.Now().Sub(start).Seconds(), record[2], record[3])

			step++
			if step == len(steps) {
				break
			}
		}
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
