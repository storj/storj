// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psclient"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// StorageNodeFlags defines storage node configuration
type StorageNodeFlags struct {
	EditConf        bool `default:"false" help:"open config in default editor"`
	SaveAllDefaults bool `default:"false" help:"save all default values to config.yaml file" setup:"true"`

	storagenode.Config
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
		Address string `default:":28967" help:"address for dashboard service"`
	}

	diagCfg struct {
	}

	defaultConfDir  string
	defaultDiagDir  string
	defaultCredsDir string
	confDir         string
	credsDir        string
)

const (
	defaultServerAddr = ":28967"
)

func init() {
	defaultConfDir = fpath.ApplicationDir("storj", "storagenode")
	// TODO: this path should be defined somewhere else
	defaultCredsDir = fpath.ApplicationDir("storj", "identity")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	rootCmd.PersistentFlags().StringVar(&confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	err := rootCmd.PersistentFlags().SetAnnotation("config-dir", "setup", []string{"true"})
	if err != nil {
		zap.S().Error("Failed to set 'setup' annotation for 'config-dir'")
	}
	rootCmd.PersistentFlags().StringVar(&credsDir, "creds-dir", defaultCredsDir, "main directory for storagenode identity credentials")

	defaultDiagDir = filepath.Join(defaultConfDir, "storage")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(dashboardCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.BindSetup(configCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, cfgstruct.ConfDir(defaultDiagDir))
	cfgstruct.Bind(dashboardCmd.Flags(), &dashboardCfg, cfgstruct.ConfDir(defaultDiagDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	if identity, err := runCfg.Server.Identity.Load(); err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", identity.ID)
	}

	if err := runCfg.Verify(); err != nil {
		zap.S().Error("Invalid configuration:", err)
		return
	}

	ctx := process.Ctx(cmd)
	if err := process.InitMetricsWithCertPath(ctx, nil, runCfg.Server.Identity.CertPath); err != nil {
		zap.S().Error("Failed to initialize telemetry batcher:", err)
	}

	db, err := storagenodedb.New(runCfg.Storage.Path) // TODO: separate path
	if err != nil {
		return err
	}

	peer, err := storagenode.New(zap.L(), identity, db, runCfg)
	if err != nil {
		return err
	}

	runError := peer.Run(process.Context(cmd))
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

	// open the sql db
	dbpath := filepath.Join(diagDir, "storage", "piecestore.db")
	db, err := psdb.Open(context.Background(), nil, dbpath)
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return err
	}

	//get all bandwidth aggrements entries already ordered
	bwAgreements, err := db.GetBandwidthAllocations()
	if err != nil {
		fmt.Println("storage node 'bandwidth_agreements' table read error:", dbpath)
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
			// deserializing rbad you get payerbwallocation, total & storage node id
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(rbaDataVal.Agreement, rbad); err != nil {
				return err
			}

			// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
			pbad := &pb.PayerBandwidthAllocation_Data{}
			if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
				return err
			}

			summary, ok := summaries[pbad.SatelliteId]
			if !ok {
				summaries[pbad.SatelliteId] = &SatelliteSummary{}
				satelliteIDs = append(satelliteIDs, pbad.SatelliteId)
				summary = summaries[pbad.SatelliteId]
			}

			// fill the summary info
			summary.TotalBytes += rbad.GetTotal()
			summary.TotalTransactions++
			switch pbad.GetAction() {
			case pb.PayerBandwidthAllocation_PUT:
				summary.PutActionCount++
			case pb.PayerBandwidthAllocation_GET:
				summary.GetActionCount++
			case pb.PayerBandwidthAllocation_GET_AUDIT:
				summary.GetAuditActionCount++
			case pb.PayerBandwidthAllocation_GET_REPAIR:
				summary.GetRepairActionCount++
			case pb.PayerBandwidthAllocation_PUT_REPAIR:
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

	ident, err := runCfg.Server.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	// address of node to create client connection
	address := dashboardCfg.Address
	if dashboardCfg.Address == "" {
		if runCfg.Server.Address == "" {
			return fmt.Errorf("Storage Node address isn't specified")
		}

		address = runCfg.Server.Address
	}

	tc := transport.NewClient(ident)
	n := &pb.Node{
		Address: &pb.NodeAddress{
			Address:   address,
			Transport: 0,
		},
		Type: pb.NodeType_STORAGE,
	}

	// create new client
	lc, err := psclient.NewLiteClient(ctx, tc, n)
	if err != nil {
		return err
	}

	stream, err := lc.Dashboard(ctx)
	if err != nil {
		return err
	}

	for {
		data, err := stream.Recv()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		clr()
		heading := color.New(color.FgGreen, color.Bold)

		_, _ = heading.Printf("\nStorage Node Dashboard Stats\n")
		_, _ = heading.Printf("\n===============================\n")

		fmt.Fprintf(color.Output, "Node ID: %s\n", color.YellowString(data.GetNodeId()))

		if data.GetConnection() {
			fmt.Fprintf(color.Output, "%s ", color.GreenString("ONLINE"))
		} else {
			fmt.Fprintf(color.Output, "%s ", color.RedString("OFFLINE"))
		}

		uptime, err := ptypes.Duration(data.GetUptime())
		if err != nil {
			color.Red(" %+v \n", err)
		} else {
			color.Yellow(" %s \n", uptime)
		}

		fmt.Fprintf(color.Output, "Node Connections: %+v\n", whiteInt(data.GetNodeConnections()))

		color.Green("\nIO\t\t\tAvailable\t\t\tUsed\n--\t\t\t---------\t\t\t----")
		stats := data.GetStats()
		if stats != nil {
			fmt.Fprintf(color.Output, "Bandwidth\t\t%+v\t\t\t%+v\n", whiteInt(stats.GetAvailableBandwidth()), whiteInt(stats.GetUsedBandwidth()))
			fmt.Fprintf(color.Output, "Disk\t\t\t%+v\t\t\t%+v\n", whiteInt(stats.GetAvailableSpace()), whiteInt(stats.GetUsedSpace()))
		} else {
			color.Yellow("Loading...")
		}

	}

	return nil
}

func whiteInt(value int64) string {
	return color.WhiteString(fmt.Sprintf("%+v", value))
}

// clr clears the screen so it can be redrawn
func clr() {
	var clear = make(map[string]func())
	clear["linux"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			_ = fmt.Errorf("Linux clear screen command returned an error %+v", err)
		}
	}
	clear["darwin"] = func() {
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			_ = fmt.Errorf("MacOS clear screen command returned an error %+v", err)
		}
	}
	clear["windows"] = func() {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		err := cmd.Run()
		if err != nil {
			_ = fmt.Errorf("Windows clear screen command returned an error %+v", err)
		}
	}

	value, ok := clear[runtime.GOOS]
	if ok {
		value()
	} else {
		panic("Your platform is unsupported! I can't clear terminal screen :(")
	}
}

func main() {
	process.Exec(rootCmd)
}
