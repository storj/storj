// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"

	"google.golang.org/grpc"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/provider"
)

var (
	targetAddr = flag.String("target", "satellite.staging.storj.io:7777",
		"address of target")

	ic provider.IdentityConfig
)

func init() {
	cfgstruct.Bind(flag.CommandLine, &ic, cfgstruct.ConfDir("$HOME/.storj/gw"))
}

func main() {
	ctx := context.Background()
	flag.Parse()
	identity, err := ic.Load()
	if err != nil {
		panic(err)
	}
	dialOpt, err := identity.DialOption()
	if err != nil {
		panic(err)
	}
	conn, err := grpc.Dial(*targetAddr, dialOpt)
	if err != nil {
		panic(err)
	}
	fmt.Println(conn.GetState())
	err = conn.Invoke(ctx, "NonExistentMethod", nil, nil)
	if err != nil && err.Error() != `rpc error: code = ResourceExhausted desc = malformed method name: "NonExistentMethod"` {
		fmt.Println(err)
	}
	fmt.Println(conn.GetState())
	conn.Close()
}
