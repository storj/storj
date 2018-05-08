// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"

	"github.com/minio/minio/cmd"

	_ "storj.io/storj/pkg/miniogw"
)

func main() { cmd.Main(os.Args) }
