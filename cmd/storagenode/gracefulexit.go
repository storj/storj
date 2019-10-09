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

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

// TODO:
// rename pb messages since it;s global

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

func (client *gracefulExitClient) getNonExitingSatellites(ctx context.Context) (*pb.GetNonExitingSatellitesResponse, error) {
	return client.conn.NodeGracefulExitClient().GetNonExitingSatellites(ctx, &pb.GetNonExitingSatellitesRequest{})
}

func (client *gracefulExitClient) initGracefulExit(ctx context.Context, req *pb.StartExitRequest) (*pb.StartExitResponse, error) {
	return client.conn.NodeGracefulExitClient().StartExit(ctx, req)
}

func (client *gracefulExitClient) close() error {
	return client.conn.Close()
}

func cmdGracefulExit(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	// TODO: Display a warning and have user confirm before proceeding
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("test message.y/n")
	var userInput string
	for scanner.Scan() {
		userInput = scanner.Text()
		break
	}
	confirmationOptions := map[string]struct{}{"y": struct{}{}, "yes": struct{}{}, "Y": struct{}{}, "Yes": struct{}{}}
	if _, ok := confirmationOptions[userInput]; !ok {
		return nil
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
	satelliteList, err := client.getNonExitingSatellites(ctx)
	if err != nil {
		return err
	}

	// display satellite options
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "Domain Name\tNode ID\tSpace Used\t")

	for _, satellite := range satelliteList.GetSatellites() {
		fmt.Fprintln(w, satellite.GetDomainName()+"\t"+satellite.NodeId.String()+"\t"+memory.Size(satellite.GetSpaceUsed()).Base10String()+"\t\n")
	}
	fmt.Fprintln(w, "Please enter the domain name for each satellite you would like to start graceful exit on with a space in between each domain name and hit enter once you are done:")
	w.Flush()

	var selectedSatellite []string

	for scanner.Scan() {
		input := scanner.Text()
		inputs := strings.Split(input, " ")
		selectedSatellite = append(selectedSatellite, inputs...)
		break
	}
	if err != scanner.Err() || err != nil {
		return err
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

	// save satellite for graceful exit into the db
	req := &pb.StartExitRequest{
		NodeIds: satelliteIDs,
	}
	resp, err := client.initGracefulExit(ctx, req)
	if err != nil {
		return err
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
