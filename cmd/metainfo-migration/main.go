// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"

	"go.uber.org/zap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: metainfo-migration pointerdb-conn-url metabase-conn-url")
		os.Exit(1)
	}

	pointerDBStr := os.Args[1]
	metabaseDBStr := os.Args[2]

	fmt.Println("Migrating metainfo:", pointerDBStr, "->", metabaseDBStr)

	ctx := context.Background()
	log := zap.L()
	migrator := NewMigrator(log, pointerDBStr, metabaseDBStr)
	err := migrator.Migrate(ctx)
	if err != nil {
		panic(err)
	}
}
