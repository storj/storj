// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/transport"
)

type dashboardClient struct {
	client pb.PieceStoreInspectorClient
}

func (dash *dashboardClient) dashboard(ctx context.Context) (*pb.DashboardResponse, error) {
	return dash.client.Dashboard(ctx, &pb.DashboardRequest{})
}

func newDashboardClient(ctx context.Context, address string) (*dashboardClient, error) {
	conn, err := transport.DialAddressInsecure(ctx, address)
	if err != nil {
		return &dashboardClient{}, err
	}

	return &dashboardClient{
		client: pb.NewPieceStoreInspectorClient(conn),
	}, nil
}

func cmdDashboard(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	online, err := getConnectionStatus(ctx, ident)
	if err != nil {
		zap.S().Error("error getting connection status %s", err.Error())
	}

	client, err := newDashboardClient(ctx, dashboardCfg.Address)
	if err != nil {
		return err
	}

	for {
		data, err := client.dashboard(ctx)
		if err != nil {
			return err
		}

		if err := printDashboard(data, online); err != nil {
			return err
		}

		// Refresh the dashboard every 3 seconds
		time.Sleep(3 * time.Second)
	}
}

func printDashboard(data *pb.DashboardResponse, online bool) error {
	clearScreen()
	color.NoColor = !useColor

	heading := color.New(color.FgGreen, color.Bold)
	_, _ = heading.Printf("\nStorage Node Dashboard\n")
	_, _ = heading.Printf("\n======================\n\n")

	w := tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "ID\t%s\n", color.YellowString(data.NodeId.String()))

	if online {
		fmt.Fprintf(w, "Status\t%s\n", color.GreenString("ONLINE"))
	} else {
		fmt.Fprintf(w, "Status\t%s\n", color.RedString("OFFLINE"))
	}

	uptime, err := ptypes.Duration(data.GetUptime())
	if err != nil {
		fmt.Fprintf(w, "Uptime\t%s\n", color.RedString(uptime.Truncate(time.Second).String()))
	} else {
		fmt.Fprintf(w, "Uptime\t%s\n", color.YellowString(uptime.Truncate(time.Second).String()))
	}

	if err = w.Flush(); err != nil {
		return err
	}

	stats := data.GetStats()
	if stats != nil {
		availableBandwidth := color.WhiteString(memory.Size(stats.GetAvailableBandwidth()).Base10String())
		usedBandwidth := color.WhiteString(memory.Size(stats.GetUsedBandwidth()).Base10String())
		availableSpace := color.WhiteString(memory.Size(stats.GetAvailableSpace()).Base10String())
		usedSpace := color.WhiteString(memory.Size(stats.GetUsedSpace()).Base10String())

		w = tabwriter.NewWriter(color.Output, 0, 0, 5, ' ', tabwriter.AlignRight)
		fmt.Fprintf(w, "\n\t%s\t%s\t\n", color.GreenString("Available"), color.GreenString("Used"))
		fmt.Fprintf(w, "Bandwidth\t%s\t%s\t\n", availableBandwidth, usedBandwidth)
		fmt.Fprintf(w, "Disk\t%s\t%s\t\n", availableSpace, usedSpace)
		if err = w.Flush(); err != nil {
			return err
		}

	} else {
		color.Yellow("Loading...\n")
	}

	w = tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	// TODO: Get addresses from server data
	fmt.Fprintf(w, "\nBootstrap\t%s\n", color.WhiteString(data.GetBootstrapAddress()))
	fmt.Fprintf(w, "Internal\t%s\n", color.WhiteString(dashboardCfg.Address))
	fmt.Fprintf(w, "External\t%s\n", color.WhiteString(data.GetExternalAddress()))
	fmt.Fprintf(w, "\nNeighborhood Size %+v\n", whiteInt(data.GetNodeConnections()))
	if err = w.Flush(); err != nil {
		return err
	}

	return nil
}

func whiteInt(value int64) string {
	return color.WhiteString(fmt.Sprintf("%+v", value))
}

// clearScreen clears the screen so it can be redrawn
func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	default:
		fmt.Print(strings.Repeat("\n", 100))
	}
}

type inspector struct {
	kadclient pb.KadInspectorClient
}

func newInspectorClient(ctx context.Context, bootstrapAddress string) (*inspector, error) {
	conn, err := transport.DialAddressInsecure(ctx, bootstrapAddress)
	if err != nil {
		return &inspector{}, err
	}

	return &inspector{
		kadclient: pb.NewKadInspectorClient(conn),
	}, nil
}

func getConnectionStatus(ctx context.Context, id *identity.FullIdentity) (bool, error) {
	inspector, err := newInspectorClient(ctx, dashboardCfg.BootstrapAddr)
	if err != nil {
		return false, err
	}

	resp, err := inspector.kadclient.PingNode(ctx, &pb.PingNodeRequest{
		Id:      id.ID,
		Address: dashboardCfg.Address,
	})

	if err != nil {
		zap.S().Error(err)
		return false, err
	}

	if resp.GetOk() {
		return true, err
	}

	return false, err
}
