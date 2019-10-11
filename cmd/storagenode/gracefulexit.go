// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/go-prompt"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

type gracefulExitClient struct {
	conn *rpc.Conn
}

func dialGracefulExitClient(ctx context.Context, address string) (*gracefulExitClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &gracefulExitClient{conn: conn}, nil
}

func (client *gracefulExitClient) getNonExitingSatellites(ctx context.Context) (*pb.GetNonExitingSatellitesResponse, error) {
	return client.conn.NodeGracefulExitClient().GetNonExitingSatellites(ctx, &pb.GetNonExitingSatellitesRequest{})
}

func (client *gracefulExitClient) initGracefulExit(ctx context.Context, req *pb.StartExitRequest) (*pb.StartExitResponse, error) {
	return client.conn.NodeGracefulExitClient().StartExit(ctx, req)
}

func (client *gracefulExitClient) close() error {
	return client.conn.Close()
}

func cmdGracefulExitInit(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	// display warning message
	if !prompt.Confirm("Please be aware that by starting a graceful exit on a satellite, you will no longer be allowed to participate in repairs or uploads from that satellite. This action can not be undone. Are you sure you want to continue? y/n\n") {
		return nil
	}

	client, err := dialGracefulExitClient(ctx, diagCfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.S().Debug("closing graceful exit client failed", err)
		}
	}()

	// get list of satellites
	satelliteList, err := client.getNonExitingSatellites(ctx)
	if err != nil {
		fmt.Println("Can't find any non-existing satellites.")
		return errs.Wrap(err)
	}

	// display satellite options
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "Domain Name\tNode ID\tSpace Used\t")

	for _, satellite := range satelliteList.GetSatellites() {
		fmt.Fprintln(w, satellite.GetDomainName()+"\t"+satellite.NodeId.String()+"\t"+memory.Size(satellite.GetSpaceUsed()).Base10String()+"\t\n")
	}
	fmt.Fprintln(w, "Please enter the domain name for each satellite you would like to start graceful exit on with a space in between each domain name and hit enter once you are done:")
	err = w.Flush()
	if err != nil {
		return errs.Wrap(err)
	}

	var selectedSatellite []string

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		// parse selected satellite from user input
		inputs := strings.Split(input, " ")
		selectedSatellite = append(selectedSatellite, inputs...)
		break
	}
	if err != scanner.Err() || err != nil {
		return errs.Wrap(err)
	}

	// validate user input
	satelliteIDs := make([]storj.NodeID, 0, len(satelliteList.GetSatellites()))
	for _, selected := range selectedSatellite {
		for _, satellite := range satelliteList.GetSatellites() {
			if satellite.GetDomainName() == selected {
				satelliteIDs = append(satelliteIDs, satellite.NodeId)
			}
		}
	}

	if len(satelliteIDs) < 1 {
		fmt.Println("Invalid input. Please use valid satellite domian names.")
		return errs.New("Invalid satellite domain names")
	}

	// save satellites for graceful exit into the db
	req := &pb.StartExitRequest{
		NodeIds: satelliteIDs,
	}
	resp, err := client.initGracefulExit(ctx, req)
	if err != nil {
		return errs.Wrap(err)
	}
	for _, status := range resp.Statuses {
		if !status.GetSuccess() {
			fmt.Printf("Failed to start graceful exit on satellite: %s\n", status.GetDomainName())
			continue
		}
		fmt.Printf("Started graceful exit on satellite: %s\n", status.GetDomainName())
	}
	return nil
}
