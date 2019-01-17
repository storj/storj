// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package luacfg_test

import (
	"bytes"
	"fmt"

	"storj.io/storj/internal/testcmd"

	"storj.io/storj/cmd/statreceiver/luacfg"
)

func init() {
	// NB: `flag.Parse()` is called in an import and causes an error when passing
	//	   flags to cmd tests unless they're defined.
	testcmd.Noop()
}

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
