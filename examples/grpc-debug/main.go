// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/storj/pkg/cfgstruct"
)

var (
	targetAddr = pflag.String("target", "satellite.staging.storj.io:7777", "address of target")

	identityConfig identity.Config
)

func init() {
	cfgstruct.Bind(pflag.CommandLine, &identityConfig, cfgstruct.UseDevDefaults(), cfgstruct.ConfDir("$HOME/.storj/gw"))
}

func main() {
	ctx := context.Background()
	pflag.Parse()
	identity, err := identityConfig.Load()
	if err != nil {
		panic(err)
	}
	clientOptions, err := tlsopts.NewOptions(identity, tlsopts.Config{}, nil)
	if err != nil {
		panic(err)
	}

	dialOption := clientOptions.DialUnverifiedIDOption()

	conn, err := grpc.Dial(*targetAddr, dialOption, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	fmt.Println(conn.GetState())
	err = conn.Invoke(ctx, "NonExistentMethod", nil, nil)
	if err != nil && err.Error() != `rpc error: code = ResourceExhausted desc = malformed method name: "NonExistentMethod"` {
		fmt.Println(err)
	}
	fmt.Println(conn.GetState())
	err = conn.Close()
	if err != nil {
		fmt.Println(err)
	}
}
