// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package luacfg_test

import (
	"bytes"
	"fmt"

	"storj.io/storj/cmd/statreceiver/luacfg"
)

func Example() {
	scope := luacfg.NewScope()
	scope.RegisterVal("print", fmt.Println)

	err := scope.Run(bytes.NewBufferString(`print "hello"`))
	if err != nil {
		panic(err)
	}

	// Output: hello
}
