// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package luacfg_test

import (
	"bytes"
	"fmt"

	"storj.io/storj/cmd/statreceiver/luacfg"
)

func Example() {
	s := luacfg.NewScope()
	s.RegisterVal("print", fmt.Println)

	err := s.Run(bytes.NewBufferString(`print "hello"`))
	if err != nil {
		panic(err)
	}

	// Output: hello
}
