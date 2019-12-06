// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package luacfg_test

import (
	"bytes"
	"fmt"

	"storj.io/storj/cmd/statreceiver/luacfg"
)

func Example() {
	scope := luacfg.NewScope()

	err := scope.RegisterVal("print", fmt.Println)
	if err != nil {
		panic(err)
	}

	err = scope.Run(bytes.NewBufferString(`print "hello"`))
	if err != nil {
		panic(err)
	}

	// Output: hello
}
