// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

// StorageNodeFlags defines storage node configuration
type StorageNodeFlags struct {
	EditConf        bool `default:"false" help:"open config in default editor"`
	SaveAllDefaults bool `default:"false" help:"save all default values to config.yaml file" setup:"true"`

	storagenode.Config
}

// Inspector holds the kad client for node inspection
type Inspector struct {
	kad pb.KadInspectorClient
}

var (
	rootCmd = &cobra.Command{
		Use:   "storagenode",
		Short: "StorageNode",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the storagenode",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	configCmd = &cobra.Command{
		Use:         "config",
		Short:       "Edit config files",
		RunE:        cmdConfig,
		Annotations: map[string]string{"type": "setup"},
	}
	diagCmd = &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE:  cmdDiag,
	}
	dashboardCmd = &cobra.Command{
		Use:   "dashboard",
		Short: "Display a dashbaord",
		RunE:  dashCmd,
	}
	runCfg   StorageNodeFlags
	setupCfg StorageNodeFlags

	dashboardCfg struct {
		Address         string `default:":28967" help:"address for dashboard service"`
		ExternalAddress string `default:":28967" help:"address that your node is listening on if using a tunneling service"`
		BootstrapAddr   string `default:"bootstrap.storj.io:8888" help:"address of server the storage node was bootstrapped against"`
	}

	defaultConfDir = fpath.ApplicationDir("storj", "storagenode")
	// TODO: this path should be defined somewhere else
	defaultIdentityDir = fpath.ApplicationDir("storj", "identity", "storagenode")
	defaultDiagDir     string
	confDir            string
	identityDir        string
)

const (
	defaultServerAddr = ":28967"
)

func init() {
	confDirParam := cfgstruct.FindConfigDirParam()
	if confDirParam != "" {
		defaultConfDir = confDirParam
	}
	identityDirParam := cfgstruct.FindIdentityDirParam()
	if identityDirParam != "" {
		defaultIdentityDir = identityDirParam
	}

	rootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	err := rootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}
	rootCmd.PersistentFlags().StringVar(&identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	err = rootCmd.PersistentFlags().SetAnnotation("identity-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}

	defaultDiagDir = filepath.Join(defaultConfDir, "storage")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(dashboardCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.BindSetup(configCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir), cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.Bind(diagCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultDiagDir), cfgstruct.IdentityDir(defaultIdentityDir))
	cfgstruct.Bind(dashboardCmd.Flags(), &dashboardCfg, cfgstruct.ConfDir(defaultDiagDir))
}

func databaseConfig(config storagenode.Config) storagenodedb.Config {
	return storagenodedb.Config{
		Storage:  config.Storage.Path,
		Info:     filepath.Join(config.Storage.Path, "piecestore.db"),
		Kademlia: config.Kademlia.DBPath,
	}
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	log := zap.L()

	identity, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	}

	if err := runCfg.Verify(log); err != nil {
		log.Sugar().Error("Invalid configuration: ", err)
		return err
	}

	ctx := process.Ctx(cmd)
	if err := process.InitMetricsWithCertPath(ctx, nil, runCfg.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher: ", err)
	}

	db, err := storagenodedb.New(databaseConfig(runCfg.Config))

	if err != nil {
		return errs.New("Error starting master database on storagenode: %+v", err)
	}

	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	err = db.CreateTables()
	if err != nil {
		return errs.New("Error creating tables for master database on storagenode: %+v", err)
	}

	peer, err := storagenode.New(log, identity, db, runCfg.Config)
	if err != nil {
		return err
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()

	return errs.Combine(runError, closeError)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("storagenode configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"log.level": "info",
	}
	serverAddress := cmd.Flag("server.address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	configFile := filepath.Join(setupDir, "config.yaml")
	if setupCfg.SaveAllDefaults {
		err = process.SaveConfigWithAllDefaults(cmd.Flags(), configFile, overrides)
	} else {
		err = process.SaveConfig(cmd.Flags(), configFile, overrides)
	}
	if err != nil {
		return err
	}

	if setupCfg.EditConf {
		return fpath.EditFile(configFile)
	}

	return err
}

func cmdConfig(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}
	//run setup if we can't access the config file
	conf := filepath.Join(setupDir, "config.yaml")
	if _, err := os.Stat(conf); err != nil {
		return cmdSetup(cmd, args)
	}

	return fpath.EditFile(conf)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	diagDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	// check if the directory exists
	_, err = os.Stat(diagDir)
	if err != nil {
		fmt.Println("Storagenode directory doesn't exist", diagDir)
		return err
	}

	db, err := storagenodedb.New(databaseConfig(runCfg.Config))
	if err != nil {
		return errs.New("Error starting master database on storagenode: %v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	//get all bandwidth aggrements entries already ordered
	bwAgreements, err := db.PSDB().GetBandwidthAllocations()
	if err != nil {
		fmt.Printf("storage node 'bandwidth_agreements' table read error: %v\n", err)
		return err
	}

	// Agreement is a struct that contains a bandwidth agreement and the associated signature
	type SatelliteSummary struct {
		TotalBytes           int64
		PutActionCount       int64
		GetActionCount       int64
		GetAuditActionCount  int64
		GetRepairActionCount int64
		PutRepairActionCount int64
		TotalTransactions    int64
		// additional attributes add here ...
	}

	// attributes per satelliteid
	summaries := make(map[storj.NodeID]*SatelliteSummary)
	satelliteIDs := storj.NodeIDList{}

	for _, rbaVal := range bwAgreements {
		for _, rbaDataVal := range rbaVal {
			rba := rbaDataVal.Agreement
			pba := rba.PayerAllocation

			summary, ok := summaries[pba.SatelliteId]
			if !ok {
				summaries[pba.SatelliteId] = &SatelliteSummary{}
				satelliteIDs = append(satelliteIDs, pba.SatelliteId)
				summary = summaries[pba.SatelliteId]
			}

			// fill the summary info
			summary.TotalBytes += rba.Total
			summary.TotalTransactions++
			switch pba.Action {
			case pb.BandwidthAction_PUT:
				summary.PutActionCount++
			case pb.BandwidthAction_GET:
				summary.GetActionCount++
			case pb.BandwidthAction_GET_AUDIT:
				summary.GetAuditActionCount++
			case pb.BandwidthAction_GET_REPAIR:
				summary.GetRepairActionCount++
			case pb.BandwidthAction_PUT_REPAIR:
				summary.PutRepairActionCount++
			}
		}
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "SatelliteID\tTotal\t# Of Transactions\tPUT Action\tGET Action\tGET (Audit) Action\tGET (Repair) Action\tPUT (Repair) Action\t")

	// populate the row fields
	sort.Sort(satelliteIDs)
	for _, satelliteID := range satelliteIDs {
		summary := summaries[satelliteID]
		fmt.Fprint(w, satelliteID, "\t", summary.TotalBytes, "\t", summary.TotalTransactions, "\t",
			summary.PutActionCount, "\t", summary.GetActionCount, "\t", summary.GetAuditActionCount,
			"\t", summary.GetRepairActionCount, "\t", summary.PutRepairActionCount, "\t\n")
	}

	// display the data
	err = w.Flush()
	return err
}

func dashCmd(cmd *cobra.Command, args []string) (err error) {
	ctx := context.Background()

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	tc := transport.NewClient(ident)
	n := &pb.Node{
		Address: &pb.NodeAddress{
			Address:   dashboardCfg.Address,
			Transport: 0,
		},
		Type: pb.NodeType_STORAGE,
	}

	lc, err := psclient.NewLiteClient(ctx, tc, n)
	if err != nil {
		return err
	}

	stream, err := lc.Dashboard(ctx)
	if err != nil {
		return err
	}

	online, err := getConnectionStatus(ctx, tc, ident)
	if err != nil {
		zap.S().Error("error getting connection status %s", err.Error())
	}

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		clearScreen()
		heading := color.New(color.FgGreen, color.Bold)

		_, _ = heading.Printf("\nStorage Node Dashboard Stats\n")
		_, _ = heading.Printf("\n===============================\n\n")

		w := tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
		fmt.Fprintf(w, "ID\t%s\n", color.YellowString(data.GetNodeId()))

		if online {
			fmt.Fprintf(w, "Status\t%s\n", color.GreenString("ONLINE"))
		} else {
			fmt.Fprintf(w, "Status\t%s\n", color.RedString("OFFLINE"))
		}

		uptime, err := ptypes.Duration(data.GetUptime())
		if err != nil {
			fmt.Fprintf(w, "Uptime\t%s\n", color.RedString(uptime.String()))
		} else {
			fmt.Fprintf(w, "Uptime\t%s\n", color.YellowString(uptime.String()))
		}

		if err = w.Flush(); err != nil {
			return err
		}

		stats := data.GetStats()
		if stats != nil {
			availableBandwidth := color.WhiteString(memory.Size(stats.GetAvailableBandwidth()).String())
			usedBandwidth := color.WhiteString(memory.Size(stats.GetUsedBandwidth()).String())
			availableSpace := color.WhiteString(memory.Size(stats.GetAvailableSpace()).String())
			usedSpace := color.WhiteString(memory.Size(stats.GetUsedSpace()).String())

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
		fmt.Fprintf(w, "\nBootstrap\t%s\n", color.WhiteString(""))
		fmt.Fprintf(w, "Internal\t%s\n", color.WhiteString(""))
		fmt.Fprintf(w, "External\t%s\n", color.WhiteString(""))
		fmt.Fprintf(w, "\nNeighborhood Size %+v\n", whiteInt(data.GetNodeConnections()))
		if err = w.Flush(); err != nil {
			return err
		}
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

func getConnectionStatus(ctx context.Context, tc transport.Client, id *identity.FullIdentity) (bool, error) {
	bn := &pb.Node{
		Address: &pb.NodeAddress{
			Address:   dashboardCfg.BootstrapAddr,
			Transport: 0,
		},
		Type: pb.NodeType_BOOTSTRAP,
	}

	inspector, err := newInspectorClient(ctx, tc, bn)
	if err != nil {
		return false, err
	}

	resp, err := inspector.kad.PingNode(ctx, &pb.PingNodeRequest{
		Id:      id.ID,
		Address: dashboardCfg.ExternalAddress,
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

func newInspectorClient(ctx context.Context, tc transport.Client, bn *pb.Node) (*Inspector, error) {
	conn, err := tc.DialNode(ctx, bn)
	if err != nil {
		return &Inspector{}, err
	}

	return &Inspector{
		kad: pb.NewKadInspectorClient(conn),
	}, nil

}

func main() {
	process.Exec(rootCmd)
}
