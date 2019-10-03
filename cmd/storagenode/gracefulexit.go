// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/rpc"
)

// const contactWindow = time.Minute * 10

type gracefulExitClient struct {
	conn *rpc.Conn
}

func dialGracefulExitClient(ctx context.Context, address string) (*gracefulExitClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return &gracefulExitClient{}, err
	}
	return &gracefulExitClient{conn: conn}, nil
}

func (client *gracefulExitClient) gracefulExit(ctx context.Context) (*pb.GetSatellitesListResponse, error) {
	return client.conn.GracefulExitClient().GetSatellitesList(ctx, &pb.GetSatellitesListRequest{})
}

func (client *gracefulExitClient) close() error {
	return client.conn.Close()
}

func cmdGracefulExit(cmd *cobra.Command, args []string) error {
	// present a message describing the consequences of starting a graceful exit
	// user must confirm before continuing
	// user needs to type the satellite domian name to start
	// get starting_disk_usage from pieces.Service
	// adds an entry to satellite table

	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	client, err := dialGracefulExitClient(ctx, diagCfg.Server.PrivateAddress)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.S().Debug("closing graceful exit client failed", err)
		}
	}()

	// get list of satellites
	satelliteList, err := client.gracefulExit(ctx)
	if err != nil {
		return err
	}

	fmt.Println("got here")
	fmt.Printf("%+v", satelliteList.GetSatellites()[0])

	return nil
}
