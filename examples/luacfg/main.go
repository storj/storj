// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"

	"storj.io/storj/pkg/luacfg"
)

func main() {
	s := luacfg.NewScope()
	s.RegisterVal("print", fmt.Println)
	err := s.Run(os.Stdin)
	if err != nil {
		panic(err)
	}
}
