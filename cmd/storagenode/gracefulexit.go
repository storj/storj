// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/rpc"
)

// const contactWindow = time.Minute * 10

type gracefulExitClient struct {
	conn *rpc.Conn
}

// func dialGracefulExitClient(ctx context.Context, address string) (*gracefulExitClient, error) {
// 	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
// 	if err != nil {
// 		return &gracefulExitClient{}, err
// 	}
// 	return &gracefulExitClient{conn: conn}, nil
// }

// func (dash *dashboardClient) gracefulExit(ctx context.Context) (*pb.GracefulExitResponse, error) {
// 	return dash.conn.
// }

// func (dash *dashboardClient) close() error {
// 	return dash.conn.Close()
// }

func cmdGracefulExit(cmd *cobra.Command, args []string) error {
	// present a message describing the consequences of starting a graceful exit
	// user must confirm before continuing
	// user needs to type the satellite domian name to start
	// get starting_disk_usage from pieces.Service
	// adds an entry to satellite table

	ctx, _ := process.Ctx(cmd)
	fmt.Println("hello", ctx)
	// get list of satellites

	return nil
}
