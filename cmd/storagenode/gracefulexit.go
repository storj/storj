// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
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

func (client *gracefulExitClient) getSatelliteList(ctx context.Context) (*pb.GetSatellitesListResponse, error) {
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
	satelliteList, err := client.getSatelliteList(ctx)
	if err != nil {
		return err
	}
	output := "Domain Name\t" + "Node ID\t\t\t" + "Space Used\t" + "\n"

	for _, satellite := range satelliteList.GetSatellites() {

		output += (satellite.GetDomainName() + "\t")
		output += (satellite.NodeId.String() + "\t")
		output += (memory.Size(satellite.GetSpaceUsed()).Base10String() + "\n")
	}

	// display the list
	qs := []*survey.Question{
		{
			Name: "satellite selection",
			Prompt: &survey.Input{
				Message: "Enter the domain name of the satellite you want to exit from:\n" + output + "\n",
			},
			Validate: survey.Required,
		},
	}

	var selectedSatellite string
	err = survey.Ask(qs, &selectedSatellite)
	if err != nil {
		return err
	}

	// validate user input
	var found bool
	for _, satellite := range satelliteList.GetSatellites() {
		if satellite.GetDomainName() == selectedSatellite {
			found = true
			break
		}
	}

	if !found {
		fmt.Println("Invalid input. Please type in a valid satellite domian name.")
		return errs.New("Invalid satellite domain name")
	}
	// save it to the db

	return nil
}
