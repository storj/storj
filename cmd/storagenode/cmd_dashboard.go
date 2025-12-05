// Copyright (C) 2020 Storj Labs, Inc.
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
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/version"
	"storj.io/storj/storagenode/internalpb"
)

const contactWindow = time.Hour * 2

type dashboardClient struct {
	conn *rpc.Conn
}

type dashboardCfg struct {
	Address  string `default:"127.0.0.1:7778" testDefault:"$HOST:0" help:"address for dashboard service"`
	Identity identity.Config
	UseColor bool `internal:"true"`
}

func newDashboardCmd(f *Factory) *cobra.Command {
	var cfg dashboardCfg

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Run the dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.UseColor = f.UseColor
			return cmdDashboard(cmd, &cfg)
		},
	}

	process.Bind(cmd, &cfg, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func dialDashboardClient(ctx context.Context, address string) (*dashboardClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return &dashboardClient{}, err
	}
	return &dashboardClient{conn: conn}, nil
}

func (dash *dashboardClient) dashboard(ctx context.Context) (*internalpb.DashboardResponse, error) {
	return internalpb.NewDRPCPieceStoreInspectorClient(dash.conn).Dashboard(ctx, &internalpb.DashboardRequest{})
}

func (dash *dashboardClient) close() error {
	return dash.conn.Close()
}

func cmdDashboard(cmd *cobra.Command, cfg *dashboardCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	ident, err := cfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	client, err := dialDashboardClient(ctx, cfg.Address)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.L().Debug("Closing dashboard client failed.", zap.Error(err))
		}
	}()

	for {
		data, err := client.dashboard(ctx)
		if err != nil {
			return err
		}

		if err := printDashboard(cfg, data); err != nil {
			return err
		}

		// Refresh the dashboard every 3 seconds
		time.Sleep(3 * time.Second)
	}
}

func printDashboard(cfg *dashboardCfg, data *internalpb.DashboardResponse) error {
	clearScreen()
	var warnFlag bool
	color.NoColor = !cfg.UseColor

	heading := color.New(color.FgGreen, color.Bold)
	_, _ = heading.Printf("\nStorage Node Dashboard ( Node Version: %s )\n", version.Build.Version.String())
	_, _ = heading.Printf("\n======================\n\n")

	w := tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	_, _ = fmt.Fprintf(w, "ID\t%s\n", color.YellowString(data.NodeId.String()))

	if data.LastPinged.IsZero() || time.Since(data.LastPinged) >= contactWindow {
		_, _ = fmt.Fprintf(w, "Status\t%s\n", color.RedString("OFFLINE"))
	} else {
		_, _ = fmt.Fprintf(w, "Status\t%s\n", color.GreenString("ONLINE"))
	}

	uptime, err := time.ParseDuration(data.GetUptime())
	if err == nil {
		_, _ = fmt.Fprintf(w, "Uptime\t%s\n", color.YellowString(uptime.Truncate(time.Second).String()))
	}

	if err = w.Flush(); err != nil {
		return err
	}

	stats := data.GetStats()
	if stats != nil {
		usedBandwidth := color.WhiteString(memory.Size(stats.GetUsedBandwidth()).Base10String())
		availableSpace := color.WhiteString(memory.Size(stats.GetAvailableSpace()).Base10String())
		usedSpace := color.WhiteString(memory.Size(stats.GetUsedSpace()).Base10String())
		usedEgress := color.WhiteString(memory.Size(stats.GetUsedEgress()).Base10String())
		usedIngress := color.WhiteString(memory.Size(stats.GetUsedIngress()).Base10String())

		w = tabwriter.NewWriter(color.Output, 0, 0, 5, ' ', tabwriter.AlignRight)
		_, _ = fmt.Fprintf(w, "\n\t%s\t%s\t%s\t%s\t\n", color.GreenString("Available"), color.GreenString("Used"), color.GreenString("Egress"), color.GreenString("Ingress"))
		_, _ = fmt.Fprintf(w, "Bandwidth\t%s\t%s\t%s\t%s\t (since %s 1)\n", color.WhiteString("N/A"), usedBandwidth, usedEgress, usedIngress, time.Now().Format("Jan"))
		_, _ = fmt.Fprintf(w, "Disk\t%s\t%s\t\n", availableSpace, usedSpace)
		if err = w.Flush(); err != nil {
			return err
		}

	} else {
		color.Yellow("Loading...\n")
	}

	w = tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	// TODO: Get addresses from server data
	_, _ = fmt.Fprintf(w, "Internal\t%s\n", color.WhiteString(cfg.Address))
	_, _ = fmt.Fprintf(w, "External\t%s\n", color.WhiteString(data.GetExternalAddress()))
	// Disabling the Link to the Dashboard as its not working yet
	// _, _ = fmt.Fprintf(w, "Dashboard\t%s\n", color.WhiteString(data.GetDashboardAddress()))
	if err = w.Flush(); err != nil {
		return err
	}

	if warnFlag {
		_, _ = fmt.Fprintf(w, "\nWARNING!!!!! %s\n", color.WhiteString("Increase your bandwidth"))
	}

	return nil
}

// clearScreen clears the screen so it can be redrawn.
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
