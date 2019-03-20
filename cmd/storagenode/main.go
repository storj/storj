// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
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
		Use:         "diag",
		Short:       "Diagnostic Tool support",
		RunE:        cmdDiag,
		Annotations: map[string]string{"type": "helper"},
	}
	dashboardCmd = &cobra.Command{
		Use:         "dashboard",
		Short:       "Display a dashbaord",
		RunE:        cmdDashboard,
		Annotations: map[string]string{"type": "helper"},
	}

	runCfg       StorageNodeFlags
	setupCfg     StorageNodeFlags
	diagCfg      storagenode.Config
	dashboardCfg struct {
		Address string `default:"127.0.0.1:7778" help:"address for dashboard service"`
	}
	defaultDiagDir string
	confDir        string
	identityDir    string
	useColor       bool
	isDev          bool
)

const (
	defaultServerAddr        = ":28967"
	defaultPrivateServerAddr = "127.0.0.1:7778"
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	defaultDiagDir = filepath.Join(defaultConfDir, "storage")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	cfgstruct.DevFlag(rootCmd, &isDev, false, "use development and test configuration settings")
	rootCmd.PersistentFlags().BoolVar(&useColor, "color", false, "use color in user interface")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(dashboardCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.BindSetup(setupCmd.Flags(), &setupCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.BindSetup(configCmd.Flags(), &setupCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, isDev, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	cfgstruct.Bind(dashboardCmd.Flags(), &dashboardCfg, isDev, cfgstruct.ConfDir(defaultDiagDir))
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

	db, err := storagenodedb.New(log.Named("db"), databaseConfig(runCfg.Config))

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

	serverPrivateAddress := cmd.Flag("server.private-address")
	if !serverPrivateAddress.Changed {
		overrides[serverPrivateAddress.Name] = defaultPrivateServerAddr
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

	db, err := storagenodedb.New(zap.L().Named("db"), databaseConfig(diagCfg))
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

func main() {
	process.Exec(rootCmd)
}
